/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/protocol"
)

func (p *Protocol) do(ctx context.Context, req *http.Request) (binding.Message, error) {
	params := cecontext.RetriesFrom(ctx)

	switch params.Strategy {
	case cecontext.BackoffStrategyConstant, cecontext.BackoffStrategyLinear, cecontext.BackoffStrategyExponential:
		return p.doWithRetry(ctx, params, req)
	case cecontext.BackoffStrategyNone:
		fallthrough
	default:
		return p.doOnce(req)
	}
}

func (p *Protocol) doOnce(req *http.Request) (binding.Message, protocol.Result) {
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, protocol.NewReceipt(false, "%w", err)
	}

	var result protocol.Result
	if resp.StatusCode/100 == 2 {
		result = protocol.ResultACK
	} else {
		result = protocol.ResultNACK
	}

	return NewMessage(resp.Header, resp.Body), NewResult(resp.StatusCode, "%w", result)
}

func (p *Protocol) doWithRetry(ctx context.Context, params *cecontext.RetryParams, req *http.Request) (binding.Message, error) {
	then := time.Now()
	retry := 0
	results := make([]protocol.Result, 0)

	for {
		msg, result := p.doOnce(req)

		// Fast track common case.
		if protocol.IsACK(result) {
			return msg, NewRetriesResult(result, retry, then, results)
		}

		// Try again?
		//
		// Make sure the error was something we should retry.

		{
			var uErr *url.Error
			if errors.As(result, &uErr) {
				goto DoBackoff
			}
		}

		{
			var httpResult *Result
			if errors.As(result, &httpResult) {
				sc := httpResult.StatusCode
				if p.isRetriableFunc(sc) {
					// retry!
					goto DoBackoff
				} else {
					// Permanent error
					cecontext.LoggerFrom(ctx).Debugw("status code not retryable, will not try again",
						zap.Error(httpResult),
						zap.Int("statusCode", sc))
					return msg, NewRetriesResult(result, retry, then, results)
				}
			}
		}

	DoBackoff:
		// Wait for the correct amount of backoff time.

		// total tries = retry + 1
		if err := params.Backoff(ctx, retry+1); err != nil {
			// do not try again.
			cecontext.LoggerFrom(ctx).Debugw("backoff error, will not try again", zap.Error(err))
			return msg, NewRetriesResult(result, retry, then, results)
		}

		retry++
		results = append(results, result)
	}
}
