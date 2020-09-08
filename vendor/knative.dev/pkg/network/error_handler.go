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

package network

import (
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
)

// ErrorHandler sets up a handler suitable for use with the ErrorHandler field on
// httputil's reverse proxy.
// TODO(mattmoor): Move the implementation into handlers/error.go once downstream consumers
// have adopted the alias.
func ErrorHandler(logger *zap.SugaredLogger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, req *http.Request, err error) {
		ss := readSockStat(logger)
		logger.Errorw("error reverse proxying request; sockstat: "+ss, zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

func readSockStat(logger *zap.SugaredLogger) string {
	b, err := ioutil.ReadFile("/proc/net/sockstat")
	if err != nil {
		logger.Errorw("Unable to read sockstat", zap.Error(err))
		return ""
	}
	return string(b)
}
