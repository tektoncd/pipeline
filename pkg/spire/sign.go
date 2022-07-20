/*
Copyright 2022 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spire

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"strings"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// Signs the TaskRun results with the TaskRun spire SVID and appends the results to PipelineResourceResult
func (w *spireEntrypointerAPIClient) Sign(ctx context.Context, results []v1beta1.PipelineResourceResult) ([]v1beta1.PipelineResourceResult, error) {
	err := w.setupClient(ctx)
	if err != nil {
		return nil, err
	}

	xsvid, err := w.getWorkloadSVID(ctx)
	if err != nil {
		return nil, err
	}

	output := []v1beta1.PipelineResourceResult{}
	p := pem.EncodeToMemory(&pem.Block{
		Bytes: xsvid.Certificates[0].Raw,
		Type:  "CERTIFICATE",
	})
	output = append(output, v1beta1.PipelineResourceResult{
		Key:        KeySVID,
		Value:      string(p),
		ResultType: v1beta1.TaskRunResultType,
	})

	for _, r := range results {
		if r.ResultType == v1beta1.TaskRunResultType {
			resultValue, err := getResultValue(r)
			if err != nil {
				return nil, err
			}
			s, err := signWithKey(xsvid, resultValue)
			if err != nil {
				return nil, err
			}
			output = append(output, v1beta1.PipelineResourceResult{
				Key:        r.Key + KeySignatureSuffix,
				Value:      base64.StdEncoding.EncodeToString(s),
				ResultType: v1beta1.TaskRunResultType,
			})
		}
	}
	// get complete manifest of keys such that it can be verified
	manifest := getManifest(results)
	output = append(output, v1beta1.PipelineResourceResult{
		Key:        KeyResultManifest,
		Value:      manifest,
		ResultType: v1beta1.TaskRunResultType,
	})
	manifestSig, err := signWithKey(xsvid, manifest)
	if err != nil {
		return nil, err
	}
	output = append(output, v1beta1.PipelineResourceResult{
		Key:        KeyResultManifest + KeySignatureSuffix,
		Value:      base64.StdEncoding.EncodeToString(manifestSig),
		ResultType: v1beta1.TaskRunResultType,
	})

	return output, nil
}

func signWithKey(xsvid *x509svid.SVID, value string) ([]byte, error) {
	dgst := sha256.Sum256([]byte(value))
	s, err := xsvid.PrivateKey.Sign(rand.Reader, dgst[:], crypto.SHA256)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func getManifest(results []v1beta1.PipelineResourceResult) string {
	keys := []string{}
	for _, r := range results {
		if strings.HasSuffix(r.Key, KeySignatureSuffix) || r.Key == KeySVID || r.ResultType != v1beta1.TaskRunResultType {
			continue
		}
		keys = append(keys, r.Key)
	}
	return strings.Join(keys, ",")
}

// AppendStatusInternalAnnotation creates the status annotations which are used by the controller to verify the status hash
func (sc *spireControllerAPIClient) AppendStatusInternalAnnotation(ctx context.Context, tr *v1beta1.TaskRun) error {
	err := sc.setupClient(ctx)
	if err != nil {
		return err
	}

	// Add status hash
	currentHash, err := hashTaskrunStatusInternal(tr)
	if err != nil {
		return err
	}

	// Sign with controller private key
	xsvid, err := sc.fetchControllerSVID(ctx)
	if err != nil {
		return err
	}

	sig, err := signWithKey(xsvid, currentHash)
	if err != nil {
		return err
	}

	// Store Controller SVID
	p := pem.EncodeToMemory(&pem.Block{
		Bytes: xsvid.Certificates[0].Raw,
		Type:  "CERTIFICATE",
	})
	if tr.Status.Annotations == nil {
		tr.Status.Annotations = map[string]string{}
	}
	tr.Status.Annotations[controllerSvidAnnotation] = string(p)
	tr.Status.Annotations[TaskRunStatusHashAnnotation] = currentHash
	tr.Status.Annotations[taskRunStatusHashSigAnnotation] = base64.StdEncoding.EncodeToString(sig)
	return nil
}
