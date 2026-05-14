// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	version "github.com/hashicorp/go-version"
)

var jsonHeader = http.Header{"content-type": []string{"application/json"}}

// Version return the library version
func Version() string {
	return "0.16.0"
}

// Client represents a thread-safe Gitea API client.
type Client struct {
	url            string
	accessToken    string
	username       string
	password       string
	otp            string
	sudo           string
	userAgent      string
	debug          bool
	httpsigner     *HTTPSign
	client         *http.Client
	ctx            context.Context
	mutex          sync.RWMutex
	serverVersion  *version.Version
	getVersionOnce sync.Once
	ignoreVersion  bool // only set by SetGiteaVersion so don't need a mutex lock
}

// Response represents the gitea response
type Response struct {
	*http.Response

	FirstPage int
	PrevPage  int
	NextPage  int
	LastPage  int
}

// ClientOption are functions used to init a new client
type ClientOption func(*Client) error

// NewClient initializes and returns a API client.
// Usage of all gitea.Client methods is concurrency-safe.
func NewClient(url string, options ...ClientOption) (*Client, error) {
	client := &Client{
		url:    strings.TrimSuffix(url, "/"),
		client: &http.Client{},
		ctx:    context.Background(),
	}
	for _, opt := range options {
		if err := opt(client); err != nil {
			return nil, err
		}
	}
	if err := client.checkServerVersionGreaterThanOrEqual(version1_11_0); err != nil {
		if errors.Is(err, &ErrUnknownVersion{}) {
			return client, err
		}
		return nil, err
	}

	return client, nil
}

// NewClientWithHTTP creates an API client with a custom http client
// Deprecated use SetHTTPClient option
func NewClientWithHTTP(url string, httpClient *http.Client) *Client {
	client, _ := NewClient(url, SetHTTPClient(httpClient))
	return client
}

// SetHTTPClient is an option for NewClient to set custom http client
func SetHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) error {
		client.SetHTTPClient(httpClient)
		return nil
	}
}

// SetHTTPClient replaces default http.Client with user given one.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.mutex.Lock()
	c.client = client
	c.mutex.Unlock()
}

// SetToken is an option for NewClient to set token
func SetToken(token string) ClientOption {
	return func(client *Client) error {
		client.mutex.Lock()
		client.accessToken = token
		client.mutex.Unlock()
		return nil
	}
}

// SetBasicAuth is an option for NewClient to set username and password
func SetBasicAuth(username, password string) ClientOption {
	return func(client *Client) error {
		client.SetBasicAuth(username, password)
		return nil
	}
}

// UseSSHCert is an option for NewClient to enable SSH certificate authentication via HTTPSign
// If you want to auth against the ssh-agent you'll need to set a principal, if you want to
// use a file on disk you'll need to specify sshKey.
// If you have an encrypted sshKey you'll need to also set the passphrase.
func UseSSHCert(principal, sshKey, passphrase string) ClientOption {
	return func(client *Client) error {
		if err := client.checkServerVersionGreaterThanOrEqual(version1_17_0); err != nil {
			return err
		}

		client.mutex.Lock()
		defer client.mutex.Unlock()

		var err error
		client.httpsigner, err = NewHTTPSignWithCert(principal, sshKey, passphrase)
		if err != nil {
			return err
		}

		return nil
	}
}

// UseSSHPubkey is an option for NewClient to enable SSH pubkey authentication via HTTPSign
// If you want to auth against the ssh-agent you'll need to set a fingerprint, if you want to
// use a file on disk you'll need to specify sshKey.
// If you have an encrypted sshKey you'll need to also set the passphrase.
func UseSSHPubkey(fingerprint, sshKey, passphrase string) ClientOption {
	return func(client *Client) error {
		if err := client.checkServerVersionGreaterThanOrEqual(version1_17_0); err != nil {
			return err
		}

		client.mutex.Lock()
		defer client.mutex.Unlock()

		var err error
		client.httpsigner, err = NewHTTPSignWithPubkey(fingerprint, sshKey, passphrase)
		if err != nil {
			return err
		}

		return nil
	}
}

// SetBasicAuth sets username and password
func (c *Client) SetBasicAuth(username, password string) {
	c.mutex.Lock()
	c.username, c.password = username, password
	c.mutex.Unlock()
}

// SetOTP is an option for NewClient to set OTP for 2FA
func SetOTP(otp string) ClientOption {
	return func(client *Client) error {
		client.SetOTP(otp)
		return nil
	}
}

// SetOTP sets OTP for 2FA
func (c *Client) SetOTP(otp string) {
	c.mutex.Lock()
	c.otp = otp
	c.mutex.Unlock()
}

// SetContext is an option for NewClient to set the default context
func SetContext(ctx context.Context) ClientOption {
	return func(client *Client) error {
		client.SetContext(ctx)
		return nil
	}
}

// SetContext set default context witch is used for http requests
func (c *Client) SetContext(ctx context.Context) {
	c.mutex.Lock()
	c.ctx = ctx
	c.mutex.Unlock()
}

// SetSudo is an option for NewClient to set sudo header
func SetSudo(sudo string) ClientOption {
	return func(client *Client) error {
		client.SetSudo(sudo)
		return nil
	}
}

// SetSudo sets username to impersonate.
func (c *Client) SetSudo(sudo string) {
	c.mutex.Lock()
	c.sudo = sudo
	c.mutex.Unlock()
}

// SetUserAgent is an option for NewClient to set user-agent header
func SetUserAgent(userAgent string) ClientOption {
	return func(client *Client) error {
		client.SetUserAgent(userAgent)
		return nil
	}
}

// SetUserAgent sets the user-agent to send with every request.
func (c *Client) SetUserAgent(userAgent string) {
	c.mutex.Lock()
	c.userAgent = userAgent
	c.mutex.Unlock()
}

// SetDebugMode is an option for NewClient to enable debug mode
func SetDebugMode() ClientOption {
	return func(client *Client) error {
		client.mutex.Lock()
		client.debug = true
		client.mutex.Unlock()
		return nil
	}
}

func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	response.parseLinkHeader()

	return response
}

func (r *Response) parseLinkHeader() {
	link := r.Header.Get("Link")
	if link == "" {
		return
	}

	links := strings.Split(link, ",")
	for _, l := range links {
		u, param, ok := strings.Cut(l, ";")
		if !ok {
			continue
		}
		u = strings.Trim(u, " <>")

		key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
		if !ok || key != "rel" {
			continue
		}

		value = strings.Trim(value, "\"")

		parsed, err := url.Parse(u)
		if err != nil {
			continue
		}

		page := parsed.Query().Get("page")
		if page == "" {
			continue
		}

		switch value {
		case "first":
			r.FirstPage, _ = strconv.Atoi(page)
		case "prev":
			r.PrevPage, _ = strconv.Atoi(page)
		case "next":
			r.NextPage, _ = strconv.Atoi(page)
		case "last":
			r.LastPage, _ = strconv.Atoi(page)
		}
	}
}

func (c *Client) getWebResponse(method, path string, body io.Reader) ([]byte, *Response, error) {
	c.mutex.RLock()
	debug := c.debug
	if debug {
		fmt.Printf("%s: %s\nBody: %v\n", method, c.url+path, body)
	}
	req, err := http.NewRequestWithContext(c.ctx, method, c.url+path, body)

	client := c.client // client ref can change from this point on so safe it
	c.mutex.RUnlock()

	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if debug {
		fmt.Printf("Response: %v\n\n", resp)
	}

	return data, newResponse(resp), err
}

func (c *Client) doRequest(method, path string, header http.Header, body io.Reader) (*Response, error) {
	c.mutex.RLock()
	debug := c.debug
	if debug {
		var bodyStr string
		if body != nil {
			bs, _ := io.ReadAll(body)
			body = bytes.NewReader(bs)
			bodyStr = string(bs)
		}
		fmt.Printf("%s: %s\nHeader: %v\nBody: %s\n", method, c.url+"/api/v1"+path, header, bodyStr)
	}
	req, err := http.NewRequestWithContext(c.ctx, method, c.url+"/api/v1"+path, body)
	if err != nil {
		c.mutex.RUnlock()
		return nil, err
	}
	if len(c.accessToken) != 0 {
		req.Header.Set("Authorization", "token "+c.accessToken)
	}
	if len(c.otp) != 0 {
		req.Header.Set("X-GITEA-OTP", c.otp)
	}
	if len(c.username) != 0 {
		req.SetBasicAuth(c.username, c.password)
	}
	if len(c.sudo) != 0 {
		req.Header.Set("Sudo", c.sudo)
	}
	if len(c.userAgent) != 0 {
		req.Header.Set("User-Agent", c.userAgent)
	}

	client := c.client // client ref can change from this point on so safe it
	c.mutex.RUnlock()

	for k, v := range header {
		req.Header[k] = v
	}

	if c.httpsigner != nil {
		err = c.SignRequest(req)
		if err != nil {
			return nil, err
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if debug {
		fmt.Printf("Response: %v\n\n", resp)
	}

	return newResponse(resp), nil
}

// Converts a response for a HTTP status code indicating an error condition
// (non-2XX) to a well-known error value and response body. For non-problematic
// (2XX) status codes nil will be returned. Note that on a non-2XX response, the
// response body stream will have been read and, hence, is closed on return.
func statusCodeToErr(resp *Response) (body []byte, err error) {
	// no error
	if resp.StatusCode/100 == 2 {
		return nil, nil
	}

	//
	// error: body will be read for details
	//
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("body read on HTTP error %d: %v", resp.StatusCode, err)
	}

	// Try to unmarshal and get an error message
	errMap := make(map[string]interface{})
	if err = json.Unmarshal(data, &errMap); err != nil {
		// when the JSON can't be parsed, data was probably empty or a
		// plain string, so we try to return a helpful error anyway
		path := resp.Request.URL.Path
		method := resp.Request.Method
		return data, fmt.Errorf("unknown API error: %d\nRequest: '%s' with '%s' method and '%s' body", resp.StatusCode, path, method, string(data))
	}

	if msg, ok := errMap["message"]; ok {
		return data, fmt.Errorf("%v", msg)
	}

	// If no error message, at least give status and data
	return data, fmt.Errorf("%s: %s", resp.Status, string(data))
}

func (c *Client) getResponseReader(method, path string, header http.Header, body io.Reader) (io.ReadCloser, *Response, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return nil, resp, err
	}

	// check for errors
	data, err := statusCodeToErr(resp)
	if err != nil {
		return io.NopCloser(bytes.NewReader(data)), resp, err
	}

	return resp.Body, resp, nil
}

func (c *Client) doRequestWithStatusHandle(method, path string, header http.Header, body io.Reader) (*Response, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return resp, err
	}

	// check for errors
	if _, err = statusCodeToErr(resp); err != nil {
		// resp.Body has already been closed in statusCodeToErr
		return resp, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	return resp, err
}

func (c *Client) getResponse(method, path string, header http.Header, body io.Reader) ([]byte, *Response, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return nil, resp, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// check for errors
	data, err := statusCodeToErr(resp)
	if err != nil {
		return data, resp, err
	}

	// success (2XX), read body
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}

	return data, resp, nil
}

func (c *Client) getParsedResponse(method, path string, header http.Header, body io.Reader, obj interface{}) (*Response, error) {
	data, resp, err := c.getResponse(method, path, header, body)
	if err != nil {
		return resp, err
	}
	return resp, json.Unmarshal(data, obj)
}

func (c *Client) getStatusCode(method, path string, header http.Header, body io.Reader) (int, *Response, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return -1, resp, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	return resp.StatusCode, resp, nil
}

// pathEscapeSegments escapes segments of a path while not escaping forward slash
func pathEscapeSegments(path string) string {
	slice := strings.Split(path, "/")
	for index := range slice {
		slice[index] = url.PathEscape(slice[index])
	}
	escapedPath := strings.Join(slice, "/")
	return escapedPath
}

// escapeValidatePathSegments is a help function to validate and encode url path segments
func escapeValidatePathSegments(seg ...*string) error {
	for i := range seg {
		if seg[i] == nil || len(*seg[i]) == 0 {
			return fmt.Errorf("path segment [%d] is empty", i)
		}
		*seg[i] = url.PathEscape(*seg[i])
	}
	return nil
}
