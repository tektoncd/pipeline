// Implements HTTP request and response signing and verification. Supports the
// major MAC and asymmetric key signature algorithms. It has several safety
// restrictions: One, none of the widely known non-cryptographically safe
// algorithms are permitted; Two, the RSA SHA256 algorithms must be available in
// the binary (and it should, barring export restrictions); Finally, the library
// assumes either the 'Authorizationn' or 'Signature' headers are to be set (but
// not both).
package httpsig

import (
	"crypto"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Algorithm specifies a cryptography secure algorithm for signing HTTP requests
// and responses.
type Algorithm string

const (
	// MAC-based algoirthms.
	HMAC_SHA224      Algorithm = hmacPrefix + "-" + sha224String
	HMAC_SHA256      Algorithm = hmacPrefix + "-" + sha256String
	HMAC_SHA384      Algorithm = hmacPrefix + "-" + sha384String
	HMAC_SHA512      Algorithm = hmacPrefix + "-" + sha512String
	HMAC_RIPEMD160   Algorithm = hmacPrefix + "-" + ripemd160String
	HMAC_SHA3_224    Algorithm = hmacPrefix + "-" + sha3_224String
	HMAC_SHA3_256    Algorithm = hmacPrefix + "-" + sha3_256String
	HMAC_SHA3_384    Algorithm = hmacPrefix + "-" + sha3_384String
	HMAC_SHA3_512    Algorithm = hmacPrefix + "-" + sha3_512String
	HMAC_SHA512_224  Algorithm = hmacPrefix + "-" + sha512_224String
	HMAC_SHA512_256  Algorithm = hmacPrefix + "-" + sha512_256String
	HMAC_BLAKE2S_256 Algorithm = hmacPrefix + "-" + blake2s_256String
	HMAC_BLAKE2B_256 Algorithm = hmacPrefix + "-" + blake2b_256String
	HMAC_BLAKE2B_384 Algorithm = hmacPrefix + "-" + blake2b_384String
	HMAC_BLAKE2B_512 Algorithm = hmacPrefix + "-" + blake2b_512String
	BLAKE2S_256      Algorithm = blake2s_256String
	BLAKE2B_256      Algorithm = blake2b_256String
	BLAKE2B_384      Algorithm = blake2b_384String
	BLAKE2B_512      Algorithm = blake2b_512String
	// RSA-based algorithms.
	RSA_SHA1   Algorithm = rsaPrefix + "-" + sha1String
	RSA_SHA224 Algorithm = rsaPrefix + "-" + sha224String
	// RSA_SHA256 is the default algorithm.
	RSA_SHA256    Algorithm = rsaPrefix + "-" + sha256String
	RSA_SHA384    Algorithm = rsaPrefix + "-" + sha384String
	RSA_SHA512    Algorithm = rsaPrefix + "-" + sha512String
	RSA_RIPEMD160 Algorithm = rsaPrefix + "-" + ripemd160String
	// ECDSA algorithms
	ECDSA_SHA224    Algorithm = ecdsaPrefix + "-" + sha224String
	ECDSA_SHA256    Algorithm = ecdsaPrefix + "-" + sha256String
	ECDSA_SHA384    Algorithm = ecdsaPrefix + "-" + sha384String
	ECDSA_SHA512    Algorithm = ecdsaPrefix + "-" + sha512String
	ECDSA_RIPEMD160 Algorithm = ecdsaPrefix + "-" + ripemd160String
	// ED25519 algorithms
	// can only be SHA512
	ED25519 Algorithm = ed25519Prefix

	// Just because you can glue things together, doesn't mean they will
	// work. The following options are not supported.
	rsa_SHA3_224    Algorithm = rsaPrefix + "-" + sha3_224String
	rsa_SHA3_256    Algorithm = rsaPrefix + "-" + sha3_256String
	rsa_SHA3_384    Algorithm = rsaPrefix + "-" + sha3_384String
	rsa_SHA3_512    Algorithm = rsaPrefix + "-" + sha3_512String
	rsa_SHA512_224  Algorithm = rsaPrefix + "-" + sha512_224String
	rsa_SHA512_256  Algorithm = rsaPrefix + "-" + sha512_256String
	rsa_BLAKE2S_256 Algorithm = rsaPrefix + "-" + blake2s_256String
	rsa_BLAKE2B_256 Algorithm = rsaPrefix + "-" + blake2b_256String
	rsa_BLAKE2B_384 Algorithm = rsaPrefix + "-" + blake2b_384String
	rsa_BLAKE2B_512 Algorithm = rsaPrefix + "-" + blake2b_512String
)

// HTTP Signatures can be applied to different HTTP headers, depending on the
// expected application behavior.
type SignatureScheme string

const (
	// Signature will place the HTTP Signature into the 'Signature' HTTP
	// header.
	Signature SignatureScheme = "Signature"
	// Authorization will place the HTTP Signature into the 'Authorization'
	// HTTP header.
	Authorization SignatureScheme = "Authorization"
)

const (
	// The HTTP Signatures specification uses the "Signature" auth-scheme
	// for the Authorization header. This is coincidentally named, but not
	// semantically the same, as the "Signature" HTTP header value.
	signatureAuthScheme = "Signature"
)

// There are subtle differences to the values in the header. The Authorization
// header has an 'auth-scheme' value that must be prefixed to the rest of the
// key and values.
func (s SignatureScheme) authScheme() string {
	switch s {
	case Authorization:
		return signatureAuthScheme
	default:
		return ""
	}
}

// Signers will sign HTTP requests or responses based on the algorithms and
// headers selected at creation time.
//
// Signers are not safe to use between multiple goroutines.
//
// Note that signatures do set the deprecated 'algorithm' parameter for
// backwards compatibility.
type Signer interface {
	// SignRequest signs the request using a private key. The public key id
	// is used by the HTTP server to identify which key to use to verify the
	// signature.
	//
	// If the Signer was created using a MAC based algorithm, then the key
	// is expected to be of type []byte. If the Signer was created using an
	// RSA based algorithm, then the private key is expected to be of type
	// *rsa.PrivateKey.
	//
	// A Digest (RFC 3230) will be added to the request. The body provided
	// must match the body used in the request, and is allowed to be nil.
	// The Digest ensures the request body is not tampered with in flight,
	// and if the signer is created to also sign the "Digest" header, the
	// HTTP Signature will then ensure both the Digest and body are not both
	// modified to maliciously represent different content.
	SignRequest(pKey crypto.PrivateKey, pubKeyId string, r *http.Request, body []byte) error
	// SignResponse signs the response using a private key. The public key
	// id is used by the HTTP client to identify which key to use to verify
	// the signature.
	//
	// If the Signer was created using a MAC based algorithm, then the key
	// is expected to be of type []byte. If the Signer was created using an
	// RSA based algorithm, then the private key is expected to be of type
	// *rsa.PrivateKey.
	//
	// A Digest (RFC 3230) will be added to the response. The body provided
	// must match the body written in the response, and is allowed to be
	// nil. The Digest ensures the response body is not tampered with in
	// flight, and if the signer is created to also sign the "Digest"
	// header, the HTTP Signature will then ensure both the Digest and body
	// are not both modified to maliciously represent different content.
	SignResponse(pKey crypto.PrivateKey, pubKeyId string, r http.ResponseWriter, body []byte) error
}

// NewSigner creates a new Signer with the provided algorithm preferences to
// make HTTP signatures. Only the first available algorithm will be used, which
// is returned by this function along with the Signer. If none of the preferred
// algorithms were available, then the default algorithm is used. The headers
// specified will be included into the HTTP signatures.
//
// The Digest will also be calculated on a request's body using the provided
// digest algorithm, if "Digest" is one of the headers listed.
//
// The provided scheme determines which header is populated with the HTTP
// Signature.
//
// An error is returned if an unknown or a known cryptographically insecure
// Algorithm is provided.
func NewSigner(prefs []Algorithm, dAlgo DigestAlgorithm, headers []string, scheme SignatureScheme, expiresIn int64) (Signer, Algorithm, error) {
	for _, pref := range prefs {
		s, err := newSigner(pref, dAlgo, headers, scheme, expiresIn)
		if err != nil {
			continue
		}
		return s, pref, err
	}
	s, err := newSigner(defaultAlgorithm, dAlgo, headers, scheme, expiresIn)
	return s, defaultAlgorithm, err
}

// Signers will sign HTTP requests or responses based on the algorithms and
// headers selected at creation time.
//
// Signers are not safe to use between multiple goroutines.
//
// Note that signatures do set the deprecated 'algorithm' parameter for
// backwards compatibility.
type SSHSigner interface {
	// SignRequest signs the request using ssh.Signer.
	// The public key id is used by the HTTP server to identify which key to use
	// to verify the signature.
	//
	// A Digest (RFC 3230) will be added to the request. The body provided
	// must match the body used in the request, and is allowed to be nil.
	// The Digest ensures the request body is not tampered with in flight,
	// and if the signer is created to also sign the "Digest" header, the
	// HTTP Signature will then ensure both the Digest and body are not both
	// modified to maliciously represent different content.
	SignRequest(pubKeyId string, r *http.Request, body []byte) error
	// SignResponse signs the response using ssh.Signer. The public key
	// id is used by the HTTP client to identify which key to use to verify
	// the signature.
	//
	// A Digest (RFC 3230) will be added to the response. The body provided
	// must match the body written in the response, and is allowed to be
	// nil. The Digest ensures the response body is not tampered with in
	// flight, and if the signer is created to also sign the "Digest"
	// header, the HTTP Signature will then ensure both the Digest and body
	// are not both modified to maliciously represent different content.
	SignResponse(pubKeyId string, r http.ResponseWriter, body []byte) error
}

// NewwSSHSigner creates a new Signer using the specified ssh.Signer
// At the moment only ed25519 ssh keys are supported.
// The headers specified will be included into the HTTP signatures.
//
// The Digest will also be calculated on a request's body using the provided
// digest algorithm, if "Digest" is one of the headers listed.
//
// The provided scheme determines which header is populated with the HTTP
// Signature.
func NewSSHSigner(s ssh.Signer, dAlgo DigestAlgorithm, headers []string, scheme SignatureScheme, expiresIn int64) (SSHSigner, Algorithm, error) {
	sshAlgo := getSSHAlgorithm(s.PublicKey().Type())
	if sshAlgo == "" {
		return nil, "", fmt.Errorf("key type: %s not supported yet.", s.PublicKey().Type())
	}

	signer, err := newSSHSigner(s, sshAlgo, dAlgo, headers, scheme, expiresIn)
	if err != nil {
		return nil, "", err
	}

	return signer, sshAlgo, nil
}

func getSSHAlgorithm(pkType string) Algorithm {
	switch {
	case strings.HasPrefix(pkType, sshPrefix+"-"+ed25519Prefix):
		return ED25519
	case strings.HasPrefix(pkType, sshPrefix+"-"+rsaPrefix):
		return RSA_SHA1
	}

	return ""
}

// Verifier verifies HTTP Signatures.
//
// It will determine which of the supported headers has the parameters
// that define the signature.
//
// Verifiers are not safe to use between multiple goroutines.
//
// Note that verification ignores the deprecated 'algorithm' parameter.
type Verifier interface {
	// KeyId gets the public key id that the signature is signed with.
	//
	// Note that the application is expected to determine the algorithm
	// used based on metadata or out-of-band information for this key id.
	KeyId() string
	// Verify accepts the public key specified by KeyId and returns an
	// error if verification fails or if the signature is malformed. The
	// algorithm must be the one used to create the signature in order to
	// pass verification. The algorithm is determined based on metadata or
	// out-of-band information for the key id.
	//
	// If the signature was created using a MAC based algorithm, then the
	// key is expected to be of type []byte. If the signature was created
	// using an RSA based algorithm, then the public key is expected to be
	// of type *rsa.PublicKey.
	Verify(pKey crypto.PublicKey, algo Algorithm) error
}

const (
	// host is treated specially because golang may not include it in the
	// request header map on the server side of a request.
	hostHeader = "Host"
)

// NewVerifier verifies the given request. It returns an error if the HTTP
// Signature parameters are not present in any headers, are present in more than
// one header, are malformed, or are missing required parameters. It ignores
// unknown HTTP Signature parameters.
func NewVerifier(r *http.Request) (Verifier, error) {
	h := r.Header
	if _, hasHostHeader := h[hostHeader]; len(r.Host) > 0 && !hasHostHeader {
		h[hostHeader] = []string{r.Host}
	}
	return newVerifier(h, func(h http.Header, toInclude []string, created int64, expires int64) (string, error) {
		return signatureString(h, toInclude, addRequestTarget(r), created, expires)
	})
}

// NewResponseVerifier verifies the given response. It returns errors under the
// same conditions as NewVerifier.
func NewResponseVerifier(r *http.Response) (Verifier, error) {
	return newVerifier(r.Header, func(h http.Header, toInclude []string, created int64, expires int64) (string, error) {
		return signatureString(h, toInclude, requestTargetNotPermitted, created, expires)
	})
}

func newSSHSigner(sshSigner ssh.Signer, algo Algorithm, dAlgo DigestAlgorithm, headers []string, scheme SignatureScheme, expiresIn int64) (SSHSigner, error) {
	var expires, created int64 = 0, 0

	if expiresIn != 0 {
		created = time.Now().Unix()
		expires = created + expiresIn
	}

	s, err := signerFromSSHSigner(sshSigner, string(algo))
	if err != nil {
		return nil, fmt.Errorf("no crypto implementation available for ssh algo %q", algo)
	}

	a := &asymmSSHSigner{
		asymmSigner: &asymmSigner{
			s:            s,
			dAlgo:        dAlgo,
			headers:      headers,
			targetHeader: scheme,
			prefix:       scheme.authScheme(),
			created:      created,
			expires:      expires,
		},
	}

	return a, nil
}

func newSigner(algo Algorithm, dAlgo DigestAlgorithm, headers []string, scheme SignatureScheme, expiresIn int64) (Signer, error) {

	var expires, created int64 = 0, 0
	if expiresIn != 0 {
		created = time.Now().Unix()
		expires = created + expiresIn
	}

	s, err := signerFromString(string(algo))
	if err == nil {
		a := &asymmSigner{
			s:            s,
			dAlgo:        dAlgo,
			headers:      headers,
			targetHeader: scheme,
			prefix:       scheme.authScheme(),
			created:      created,
			expires:      expires,
		}
		return a, nil
	}
	m, err := macerFromString(string(algo))
	if err != nil {
		return nil, fmt.Errorf("no crypto implementation available for %q", algo)
	}
	c := &macSigner{
		m:            m,
		dAlgo:        dAlgo,
		headers:      headers,
		targetHeader: scheme,
		prefix:       scheme.authScheme(),
		created:      created,
		expires:      expires,
	}
	return c, nil
}
