/*
Copyright 2019 The Knative Authors

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

// http.go includes functions to send HTTP requests.

package slackutil

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// post sends an HTTP post request
func post(url string, uv url.Values) ([]byte, error) {
	resp, err := http.PostForm(url, uv)
	if err != nil {
		return nil, err
	}
	return handleResponse(resp)
}

// get sends an HTTP get request
func get(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	return handleResponse(resp)
}

// handleResponse handles the HTTP response and returns the body content
func handleResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http response code is not StatusOK: '%v'", resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}
