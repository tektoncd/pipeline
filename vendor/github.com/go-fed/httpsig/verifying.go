package httpsig

import (
	"crypto"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var _ Verifier = &verifier{}

type verifier struct {
	header      http.Header
	kId         string
	signature   string
	created     int64
	expires     int64
	headers     []string
	sigStringFn func(http.Header, []string, int64, int64) (string, error)
}

func newVerifier(h http.Header, sigStringFn func(http.Header, []string, int64, int64) (string, error)) (*verifier, error) {
	scheme, s, err := getSignatureScheme(h)
	if err != nil {
		return nil, err
	}
	kId, sig, headers, created, expires, err := getSignatureComponents(scheme, s)
	if created != 0 {
		//check if created is not in the future, we assume a maximum clock offset of 10 seconds
		now := time.Now().Unix()
		if created-now > 10 {
			return nil, errors.New("created is in the future")
		}
	}
	if expires != 0 {
		//check if expires is in the past, we assume a maximum clock offset of 10 seconds
		now := time.Now().Unix()
		if now-expires > 10 {
			return nil, errors.New("signature expired")
		}
	}
	if err != nil {
		return nil, err
	}
	return &verifier{
		header:      h,
		kId:         kId,
		signature:   sig,
		created:     created,
		expires:     expires,
		headers:     headers,
		sigStringFn: sigStringFn,
	}, nil
}

func (v *verifier) KeyId() string {
	return v.kId
}

func (v *verifier) Verify(pKey crypto.PublicKey, algo Algorithm) error {
	s, err := signerFromString(string(algo))
	if err == nil {
		return v.asymmVerify(s, pKey)
	}
	m, err := macerFromString(string(algo))
	if err == nil {
		return v.macVerify(m, pKey)
	}
	return fmt.Errorf("no crypto implementation available for %q", algo)
}

func (v *verifier) macVerify(m macer, pKey crypto.PublicKey) error {
	key, ok := pKey.([]byte)
	if !ok {
		return fmt.Errorf("public key for MAC verifying must be of type []byte")
	}
	signature, err := v.sigStringFn(v.header, v.headers, v.created, v.expires)
	if err != nil {
		return err
	}
	actualMAC, err := base64.StdEncoding.DecodeString(v.signature)
	if err != nil {
		return err
	}
	ok, err = m.Equal([]byte(signature), actualMAC, key)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("invalid http signature")
	}
	return nil
}

func (v *verifier) asymmVerify(s signer, pKey crypto.PublicKey) error {
	toHash, err := v.sigStringFn(v.header, v.headers, v.created, v.expires)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(v.signature)
	if err != nil {
		return err
	}
	err = s.Verify(pKey, []byte(toHash), signature)
	if err != nil {
		return err
	}
	return nil
}

func getSignatureScheme(h http.Header) (scheme SignatureScheme, val string, err error) {
	s := h.Get(string(Signature))
	sigHasAll := strings.Contains(s, keyIdParameter) ||
		strings.Contains(s, headersParameter) ||
		strings.Contains(s, signatureParameter)
	a := h.Get(string(Authorization))
	authHasAll := strings.Contains(a, keyIdParameter) ||
		strings.Contains(a, headersParameter) ||
		strings.Contains(a, signatureParameter)
	if sigHasAll && authHasAll {
		err = fmt.Errorf("both %q and %q have signature parameters", Signature, Authorization)
		return
	} else if !sigHasAll && !authHasAll {
		err = fmt.Errorf("neither %q nor %q have signature parameters", Signature, Authorization)
		return
	} else if sigHasAll {
		val = s
		scheme = Signature
		return
	} else { // authHasAll
		val = a
		scheme = Authorization
		return
	}
}

func getSignatureComponents(scheme SignatureScheme, s string) (kId, sig string, headers []string, created int64, expires int64, err error) {
	if as := scheme.authScheme(); len(as) > 0 {
		s = strings.TrimPrefix(s, as+prefixSeparater)
	}
	params := strings.Split(s, parameterSeparater)
	for _, p := range params {
		kv := strings.SplitN(p, parameterKVSeparater, 2)
		if len(kv) != 2 {
			err = fmt.Errorf("malformed http signature parameter: %v", kv)
			return
		}
		k := kv[0]
		v := strings.Trim(kv[1], parameterValueDelimiter)
		switch k {
		case keyIdParameter:
			kId = v
		case createdKey:
			created, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return
			}
		case expiresKey:
			expires, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return
			}
		case algorithmParameter:
			// Deprecated, ignore
		case headersParameter:
			headers = strings.Split(v, headerParameterValueDelim)
		case signatureParameter:
			sig = v
		default:
			// Ignore unrecognized parameters
		}
	}
	if len(kId) == 0 {
		err = fmt.Errorf("missing %q parameter in http signature", keyIdParameter)
	} else if len(sig) == 0 {
		err = fmt.Errorf("missing %q parameter in http signature", signatureParameter)
	} else if len(headers) == 0 { // Optional
		headers = defaultHeaders
	}
	return
}
