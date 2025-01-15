package httpsig

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

const (
	// Signature Parameters
	keyIdParameter            = "keyId"
	algorithmParameter        = "algorithm"
	headersParameter          = "headers"
	signatureParameter        = "signature"
	prefixSeparater           = " "
	parameterKVSeparater      = "="
	parameterValueDelimiter   = "\""
	parameterSeparater        = ","
	headerParameterValueDelim = " "
	// RequestTarget specifies to include the http request method and
	// entire URI in the signature. Pass it as a header to NewSigner.
	RequestTarget = "(request-target)"
	createdKey    = "created"
	expiresKey    = "expires"
	dateHeader    = "date"

	// Signature String Construction
	headerFieldDelimiter   = ": "
	headersDelimiter       = "\n"
	headerValueDelimiter   = ", "
	requestTargetSeparator = " "
)

var defaultHeaders = []string{dateHeader}

var _ Signer = &macSigner{}

type macSigner struct {
	m            macer
	makeDigest   bool
	dAlgo        DigestAlgorithm
	headers      []string
	targetHeader SignatureScheme
	prefix       string
	created      int64
	expires      int64
}

func (m *macSigner) SignRequest(pKey crypto.PrivateKey, pubKeyId string, r *http.Request, body []byte) error {
	if body != nil {
		err := addDigest(r, m.dAlgo, body)
		if err != nil {
			return err
		}
	}
	s, err := m.signatureString(r)
	if err != nil {
		return err
	}
	enc, err := m.signSignature(pKey, s)
	if err != nil {
		return err
	}
	setSignatureHeader(r.Header, string(m.targetHeader), m.prefix, pubKeyId, m.m.String(), enc, m.headers, m.created, m.expires)
	return nil
}

func (m *macSigner) SignResponse(pKey crypto.PrivateKey, pubKeyId string, r http.ResponseWriter, body []byte) error {
	if body != nil {
		err := addDigestResponse(r, m.dAlgo, body)
		if err != nil {
			return err
		}
	}
	s, err := m.signatureStringResponse(r)
	if err != nil {
		return err
	}
	enc, err := m.signSignature(pKey, s)
	if err != nil {
		return err
	}
	setSignatureHeader(r.Header(), string(m.targetHeader), m.prefix, pubKeyId, m.m.String(), enc, m.headers, m.created, m.expires)
	return nil
}

func (m *macSigner) signSignature(pKey crypto.PrivateKey, s string) (string, error) {
	pKeyBytes, ok := pKey.([]byte)
	if !ok {
		return "", fmt.Errorf("private key for MAC signing must be of type []byte")
	}
	sig, err := m.m.Sign([]byte(s), pKeyBytes)
	if err != nil {
		return "", err
	}
	enc := base64.StdEncoding.EncodeToString(sig)
	return enc, nil
}

func (m *macSigner) signatureString(r *http.Request) (string, error) {
	return signatureString(r.Header, m.headers, addRequestTarget(r), m.created, m.expires)
}

func (m *macSigner) signatureStringResponse(r http.ResponseWriter) (string, error) {
	return signatureString(r.Header(), m.headers, requestTargetNotPermitted, m.created, m.expires)
}

var _ Signer = &asymmSigner{}

type asymmSigner struct {
	s            signer
	makeDigest   bool
	dAlgo        DigestAlgorithm
	headers      []string
	targetHeader SignatureScheme
	prefix       string
	created      int64
	expires      int64
}

func (a *asymmSigner) SignRequest(pKey crypto.PrivateKey, pubKeyId string, r *http.Request, body []byte) error {
	if body != nil {
		err := addDigest(r, a.dAlgo, body)
		if err != nil {
			return err
		}
	}
	s, err := a.signatureString(r)
	if err != nil {
		return err
	}
	enc, err := a.signSignature(pKey, s)
	if err != nil {
		return err
	}
	setSignatureHeader(r.Header, string(a.targetHeader), a.prefix, pubKeyId, a.s.String(), enc, a.headers, a.created, a.expires)
	return nil
}

func (a *asymmSigner) SignResponse(pKey crypto.PrivateKey, pubKeyId string, r http.ResponseWriter, body []byte) error {
	if body != nil {
		err := addDigestResponse(r, a.dAlgo, body)
		if err != nil {
			return err
		}
	}
	s, err := a.signatureStringResponse(r)
	if err != nil {
		return err
	}
	enc, err := a.signSignature(pKey, s)
	if err != nil {
		return err
	}
	setSignatureHeader(r.Header(), string(a.targetHeader), a.prefix, pubKeyId, a.s.String(), enc, a.headers, a.created, a.expires)
	return nil
}

func (a *asymmSigner) signSignature(pKey crypto.PrivateKey, s string) (string, error) {
	sig, err := a.s.Sign(rand.Reader, pKey, []byte(s))
	if err != nil {
		return "", err
	}
	enc := base64.StdEncoding.EncodeToString(sig)
	return enc, nil
}

func (a *asymmSigner) signatureString(r *http.Request) (string, error) {
	return signatureString(r.Header, a.headers, addRequestTarget(r), a.created, a.expires)
}

func (a *asymmSigner) signatureStringResponse(r http.ResponseWriter) (string, error) {
	return signatureString(r.Header(), a.headers, requestTargetNotPermitted, a.created, a.expires)
}

var _ SSHSigner = &asymmSSHSigner{}

type asymmSSHSigner struct {
	*asymmSigner
}

func (a *asymmSSHSigner) SignRequest(pubKeyId string, r *http.Request, body []byte) error {
	return a.asymmSigner.SignRequest(nil, pubKeyId, r, body)
}

func (a *asymmSSHSigner) SignResponse(pubKeyId string, r http.ResponseWriter, body []byte) error {
	return a.asymmSigner.SignResponse(nil, pubKeyId, r, body)
}

func setSignatureHeader(h http.Header, targetHeader, prefix, pubKeyId, algo, enc string, headers []string, created int64, expires int64) {
	if len(headers) == 0 {
		headers = defaultHeaders
	}
	var b bytes.Buffer
	// KeyId
	b.WriteString(prefix)
	if len(prefix) > 0 {
		b.WriteString(prefixSeparater)
	}
	b.WriteString(keyIdParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(pubKeyId)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Algorithm
	b.WriteString(algorithmParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString("hs2019") //real algorithm is hidden, see newest version of spec draft
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)

	hasCreated := false
	hasExpires := false
	for _, h := range headers {
		val := strings.ToLower(h)
		if val == "("+createdKey+")" {
			hasCreated = true
		} else if val == "("+expiresKey+")" {
			hasExpires = true
		}
	}

	// Created
	if hasCreated == true {
		b.WriteString(createdKey)
		b.WriteString(parameterKVSeparater)
		b.WriteString(strconv.FormatInt(created, 10))
		b.WriteString(parameterSeparater)
	}

	// Expires
	if hasExpires == true {
		b.WriteString(expiresKey)
		b.WriteString(parameterKVSeparater)
		b.WriteString(strconv.FormatInt(expires, 10))
		b.WriteString(parameterSeparater)
	}

	// Headers
	b.WriteString(headersParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	for i, h := range headers {
		b.WriteString(strings.ToLower(h))
		if i != len(headers)-1 {
			b.WriteString(headerParameterValueDelim)
		}
	}
	b.WriteString(parameterValueDelimiter)
	b.WriteString(parameterSeparater)
	// Signature
	b.WriteString(signatureParameter)
	b.WriteString(parameterKVSeparater)
	b.WriteString(parameterValueDelimiter)
	b.WriteString(enc)
	b.WriteString(parameterValueDelimiter)
	h.Add(targetHeader, b.String())
}

func requestTargetNotPermitted(b *bytes.Buffer) error {
	return fmt.Errorf("cannot sign with %q on anything other than an http request", RequestTarget)
}

func addRequestTarget(r *http.Request) func(b *bytes.Buffer) error {
	return func(b *bytes.Buffer) error {
		b.WriteString(RequestTarget)
		b.WriteString(headerFieldDelimiter)
		b.WriteString(strings.ToLower(r.Method))
		b.WriteString(requestTargetSeparator)
		b.WriteString(r.URL.Path)

		if r.URL.RawQuery != "" {
			b.WriteString("?")
			b.WriteString(r.URL.RawQuery)
		}

		return nil
	}
}

func signatureString(values http.Header, include []string, requestTargetFn func(b *bytes.Buffer) error, created int64, expires int64) (string, error) {
	if len(include) == 0 {
		include = defaultHeaders
	}
	var b bytes.Buffer
	for n, i := range include {
		i := strings.ToLower(i)
		if i == RequestTarget {
			err := requestTargetFn(&b)
			if err != nil {
				return "", err
			}
		} else if i == "("+expiresKey+")" {
			if expires == 0 {
				return "", fmt.Errorf("missing expires value")
			}
			b.WriteString(i)
			b.WriteString(headerFieldDelimiter)
			b.WriteString(strconv.FormatInt(expires, 10))
		} else if i == "("+createdKey+")" {
			if created == 0 {
				return "", fmt.Errorf("missing created value")
			}
			b.WriteString(i)
			b.WriteString(headerFieldDelimiter)
			b.WriteString(strconv.FormatInt(created, 10))
		} else {
			hv, ok := values[textproto.CanonicalMIMEHeaderKey(i)]
			if !ok {
				return "", fmt.Errorf("missing header %q", i)
			}
			b.WriteString(i)
			b.WriteString(headerFieldDelimiter)
			for i, v := range hv {
				b.WriteString(strings.TrimSpace(v))
				if i < len(hv)-1 {
					b.WriteString(headerValueDelimiter)
				}
			}
		}
		if n < len(include)-1 {
			b.WriteString(headersDelimiter)
		}
	}
	return b.String(), nil
}
