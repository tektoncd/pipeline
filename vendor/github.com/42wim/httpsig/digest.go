package httpsig

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"fmt"
	"hash"
	"net/http"
	"strings"
)

type DigestAlgorithm string

const (
	DigestSha256 DigestAlgorithm = "SHA-256"
	DigestSha512                 = "SHA-512"
)

var digestToDef = map[DigestAlgorithm]crypto.Hash{
	DigestSha256: crypto.SHA256,
	DigestSha512: crypto.SHA512,
}

// IsSupportedDigestAlgorithm returns true if hte string is supported by this
// library, is not a hash known to be weak, and is supported by the hardware.
func IsSupportedDigestAlgorithm(algo string) bool {
	uc := DigestAlgorithm(strings.ToUpper(algo))
	c, ok := digestToDef[uc]
	return ok && c.Available()
}

func getHash(alg DigestAlgorithm) (h hash.Hash, toUse DigestAlgorithm, err error) {
	upper := DigestAlgorithm(strings.ToUpper(string(alg)))
	c, ok := digestToDef[upper]
	if !ok {
		err = fmt.Errorf("unknown or unsupported Digest algorithm: %s", alg)
	} else if !c.Available() {
		err = fmt.Errorf("unavailable Digest algorithm: %s", alg)
	} else {
		h = c.New()
		toUse = upper
	}
	return
}

const (
	digestHeader = "Digest"
	digestDelim  = "="
)

func addDigest(r *http.Request, algo DigestAlgorithm, b []byte) (err error) {
	_, ok := r.Header[digestHeader]
	if ok {
		err = fmt.Errorf("cannot add Digest: Digest is already set")
		return
	}
	var h hash.Hash
	var a DigestAlgorithm
	h, a, err = getHash(algo)
	if err != nil {
		return
	}
	h.Write(b)
	sum := h.Sum(nil)
	r.Header.Add(digestHeader,
		fmt.Sprintf("%s%s%s",
			a,
			digestDelim,
			base64.StdEncoding.EncodeToString(sum[:])))
	return
}

func addDigestResponse(r http.ResponseWriter, algo DigestAlgorithm, b []byte) (err error) {
	_, ok := r.Header()[digestHeader]
	if ok {
		err = fmt.Errorf("cannot add Digest: Digest is already set")
		return
	}
	var h hash.Hash
	var a DigestAlgorithm
	h, a, err = getHash(algo)
	if err != nil {
		return
	}
	h.Write(b)
	sum := h.Sum(nil)
	r.Header().Add(digestHeader,
		fmt.Sprintf("%s%s%s",
			a,
			digestDelim,
			base64.StdEncoding.EncodeToString(sum[:])))
	return
}

func verifyDigest(r *http.Request, body *bytes.Buffer) (err error) {
	d := r.Header.Get(digestHeader)
	if len(d) == 0 {
		err = fmt.Errorf("cannot verify Digest: request has no Digest header")
		return
	}
	elem := strings.SplitN(d, digestDelim, 2)
	if len(elem) != 2 {
		err = fmt.Errorf("cannot verify Digest: malformed Digest: %s", d)
		return
	}
	var h hash.Hash
	h, _, err = getHash(DigestAlgorithm(elem[0]))
	if err != nil {
		return
	}
	h.Write(body.Bytes())
	sum := h.Sum(nil)
	encSum := base64.StdEncoding.EncodeToString(sum[:])
	if encSum != elem[1] {
		err = fmt.Errorf("cannot verify Digest: header Digest does not match the digest of the request body")
		return
	}
	return
}
