package pullrequest

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"

	"github.com/google/go-cmp/cmp"
)

type handler struct {
	Header http.Header
	Data   string
}

// reflect returns the incoming body as the response. This is useful for
// simulating responses to POST methods.
func reflect(w http.ResponseWriter, r *http.Request) {
	t := io.TeeReader(r.Body, w)
	defer r.Body.Close()
	if _, err := ioutil.ReadAll(t); err != nil {
		log.Printf("ioutil.ReadAll: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// ServeHTTP serves the proxy HTTP handler. Any POST requests will reflect
// provided data back to the client, any other methods will attempt to read
// an HTTP response from from <data dir>/<url path>/<method>, e.g.
// testdata/github.com/foo/bar/GET.
// The server will also check headers provided by the client. If they do not
// match, an error is returned (this is useful for verifying auth headers are
// being sent).
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
	for k, v := range h.Header {
		if diff := cmp.Diff(v, r.Header[k]); diff != "" {
			log.Printf("required header [%s], -want +got: %s", k, diff)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	if r.Method == http.MethodPost {
		reflect(w, r)
		return
	}

	http.ServeFile(w, r, filepath.Join(h.Data, r.URL.Path, r.Method))
}
