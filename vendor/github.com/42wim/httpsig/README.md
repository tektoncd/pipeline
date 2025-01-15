# httpsig

Forked from <https://github.com/go-fed/httpsig>

> HTTP Signatures made simple

[![Build Status][Build-Status-Image]][Build-Status-Url] [![Go Reference][Go-Reference-Image]][Go-Reference-Url]
[![Go Report Card][Go-Report-Card-Image]][Go-Report-Card-Url] [![License][License-Image]][License-Url]
[![Chat][Chat-Image]][Chat-Url] [![OpenCollective][OpenCollective-Image]][OpenCollective-Url]

`go get github.com/42wim/httpsig`

Implementation of [HTTP Signatures](https://tools.ietf.org/html/draft-cavage-http-signatures).

Supports many different combinations of MAC, HMAC signing of hash, or RSA
signing of hash schemes. Its goals are:

* Have a very simple interface for signing and validating
* Support a variety of signing algorithms and combinations
* Support setting either headers (`Authorization` or `Signature`)
* Remaining flexible with headers included in the signing string
* Support both HTTP requests and responses
* Explicitly not support known-cryptographically weak algorithms
* Support automatic signing and validating Digest headers

## How to use

`import "github.com/42wim/httpsig"`

### Signing

Signing a request or response requires creating a new `Signer` and using it:

```
func sign(privateKey crypto.PrivateKey, pubKeyId string, r *http.Request) error {
	prefs := []httpsig.Algorithm{httpsig.RSA_SHA512, httpsig.RSA_SHA256}
	digestAlgorithm := DigestSha256
	// The "Date" and "Digest" headers must already be set on r, as well as r.URL.
	headersToSign := []string{httpsig.RequestTarget, "date", "digest"}
	signer, chosenAlgo, err := httpsig.NewSigner(prefs, digestAlgorithm, headersToSign, httpsig.Signature)
	if err != nil {
		return err
	}
	// To sign the digest, we need to give the signer a copy of the body...
	// ...but it is optional, no digest will be signed if given "nil"
	body := ...
	// If r were a http.ResponseWriter, call SignResponse instead.
	return signer.SignRequest(privateKey, pubKeyId, r, body)
}
```

`Signer`s are not safe for concurrent use by goroutines, so be sure to guard
access:

```
type server struct {
	signer httpsig.Signer
	mu *sync.Mutex
}

func (s *server) handlerFunc(w http.ResponseWriter, r *http.Request) {
	privateKey := ...
	pubKeyId := ...
	// Set headers and such on w
	s.mu.Lock()
	defer s.mu.Unlock()
	// To sign the digest, we need to give the signer a copy of the response body...
	// ...but it is optional, no digest will be signed if given "nil"
	body := ...
	err := s.signer.SignResponse(privateKey, pubKeyId, w, body)
	if err != nil {
		...
	}
	...
}
```

The `pubKeyId` will be used at verification time.

### Verifying

Verifying requires an application to use the `pubKeyId` to both retrieve the key
needed for verification as well as determine the algorithm to use. Use a
`Verifier`:

```
func verify(r *http.Request) error {
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		return err
	}
	pubKeyId := verifier.KeyId()
	var algo httpsig.Algorithm = ...
	var pubKey crypto.PublicKey = ...
	// The verifier will verify the Digest in addition to the HTTP signature
	return verifier.Verify(pubKey, algo)
}
```

`Verifier`s are not safe for concurrent use by goroutines, but since they are
constructed on a per-request or per-response basis it should not be a common
restriction.

[Build-Status-Image]: https://travis-ci.org/42wim/httpsig.svg?branch=master
[Build-Status-Url]: https://travis-ci.org/42wim/httpsig
[Go-Reference-Image]: https://pkg.go.dev/badge/github.com/42wim/httpsig
[Go-Reference-Url]: https://pkg.go.dev/github.com/42wim/httpsig
[Go-Report-Card-Image]: https://goreportcard.com/badge/github.com/42wim/httpsig
[Go-Report-Card-Url]: https://goreportcard.com/report/github.com/42wim/httpsig
[License-Image]: https://img.shields.io/github/license/42wim/httpsig?color=blue
[License-Url]: https://opensource.org/licenses/BSD-3-Clause
[Chat-Image]: https://img.shields.io/matrix/42wim:feneas.org?server_fqdn=matrix.org
[Chat-Url]: https://matrix.to/#/!BLOSvIyKTDLIVjRKSc:feneas.org?via=feneas.org&via=matrix.org
[OpenCollective-Image]: https://img.shields.io/opencollective/backers/42wim-activitypub-labs
[OpenCollective-Url]: https://opencollective.com/42wim-activitypub-labs
