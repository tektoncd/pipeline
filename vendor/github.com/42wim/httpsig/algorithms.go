package httpsig

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle" // Use should trigger great care
	"encoding/asn1"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
	"strings"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/sha3"
	"golang.org/x/crypto/ssh"
)

const (
	hmacPrefix        = "hmac"
	rsaPrefix         = "rsa"
	sshPrefix         = "ssh"
	ecdsaPrefix       = "ecdsa"
	ed25519Prefix     = "ed25519"
	sha224String      = "sha224"
	sha256String      = "sha256"
	sha384String      = "sha384"
	sha512String      = "sha512"
	sha3_224String    = "sha3-224"
	sha3_256String    = "sha3-256"
	sha3_384String    = "sha3-384"
	sha3_512String    = "sha3-512"
	sha512_224String  = "sha512-224"
	sha512_256String  = "sha512-256"
	blake2s_256String = "blake2s-256"
	blake2b_256String = "blake2b-256"
	blake2b_384String = "blake2b-384"
	blake2b_512String = "blake2b-512"
)

var blake2Algorithms = map[crypto.Hash]bool{
	crypto.BLAKE2s_256: true,
	crypto.BLAKE2b_256: true,
	crypto.BLAKE2b_384: true,
	crypto.BLAKE2b_512: true,
}

var hashToDef = map[crypto.Hash]struct {
	name string
	new  func(key []byte) (hash.Hash, error) // Only MACers will accept a key
}{
	// Which standard names these?
	// The spec lists the following as a canonical reference, which is dead:
	// http://www.iana.org/assignments/signature-algorithms
	//
	// Note that the forbidden hashes have an invalid 'new' function.
	crypto.SHA224:      {sha224String, func(key []byte) (hash.Hash, error) { return sha256.New224(), nil }},
	crypto.SHA256:      {sha256String, func(key []byte) (hash.Hash, error) { return sha256.New(), nil }},
	crypto.SHA384:      {sha384String, func(key []byte) (hash.Hash, error) { return sha512.New384(), nil }},
	crypto.SHA512:      {sha512String, func(key []byte) (hash.Hash, error) { return sha512.New(), nil }},
	crypto.SHA3_224:    {sha3_224String, func(key []byte) (hash.Hash, error) { return sha3.New224(), nil }},
	crypto.SHA3_256:    {sha3_256String, func(key []byte) (hash.Hash, error) { return sha3.New256(), nil }},
	crypto.SHA3_384:    {sha3_384String, func(key []byte) (hash.Hash, error) { return sha3.New384(), nil }},
	crypto.SHA3_512:    {sha3_512String, func(key []byte) (hash.Hash, error) { return sha3.New512(), nil }},
	crypto.SHA512_224:  {sha512_224String, func(key []byte) (hash.Hash, error) { return sha512.New512_224(), nil }},
	crypto.SHA512_256:  {sha512_256String, func(key []byte) (hash.Hash, error) { return sha512.New512_256(), nil }},
	crypto.BLAKE2s_256: {blake2s_256String, func(key []byte) (hash.Hash, error) { return blake2s.New256(key) }},
	crypto.BLAKE2b_256: {blake2b_256String, func(key []byte) (hash.Hash, error) { return blake2b.New256(key) }},
	crypto.BLAKE2b_384: {blake2b_384String, func(key []byte) (hash.Hash, error) { return blake2b.New384(key) }},
	crypto.BLAKE2b_512: {blake2b_512String, func(key []byte) (hash.Hash, error) { return blake2b.New512(key) }},
}

var stringToHash map[string]crypto.Hash

const (
	defaultAlgorithm        = RSA_SHA256
	defaultAlgorithmHashing = sha256String
)

func init() {
	stringToHash = make(map[string]crypto.Hash, len(hashToDef))
	for k, v := range hashToDef {
		stringToHash[v.name] = k
	}
	// This should guarantee that at runtime the defaultAlgorithm will not
	// result in errors when fetching a macer or signer (see algorithms.go)
	if ok, err := isAvailable(string(defaultAlgorithmHashing)); err != nil {
		panic(err)
	} else if !ok {
		panic(fmt.Sprintf("the default httpsig algorithm is unavailable: %q", defaultAlgorithm))
	}
}

func isForbiddenHash(h crypto.Hash) bool {
	return false
}

// signer is an internally public type.
type signer interface {
	Sign(rand io.Reader, p crypto.PrivateKey, sig []byte) ([]byte, error)
	Verify(pub crypto.PublicKey, toHash, signature []byte) error
	String() string
}

// macer is an internally public type.
type macer interface {
	Sign(sig, key []byte) ([]byte, error)
	Equal(sig, actualMAC, key []byte) (bool, error)
	String() string
}

var _ macer = &hmacAlgorithm{}

type hmacAlgorithm struct {
	fn   func(key []byte) (hash.Hash, error)
	kind crypto.Hash
}

func (h *hmacAlgorithm) Sign(sig, key []byte) ([]byte, error) {
	hs, err := h.fn(key)
	if err != nil {
		return nil, err
	}
	if err = setSig(hs, sig); err != nil {
		return nil, err
	}
	return hs.Sum(nil), nil
}

func (h *hmacAlgorithm) Equal(sig, actualMAC, key []byte) (bool, error) {
	hs, err := h.fn(key)
	if err != nil {
		return false, err
	}
	defer hs.Reset()
	err = setSig(hs, sig)
	if err != nil {
		return false, err
	}
	expected := hs.Sum(nil)
	return hmac.Equal(actualMAC, expected), nil
}

func (h *hmacAlgorithm) String() string {
	return fmt.Sprintf("%s-%s", hmacPrefix, hashToDef[h.kind].name)
}

var _ signer = &rsaAlgorithm{}

type rsaAlgorithm struct {
	hash.Hash
	kind      crypto.Hash
	sshSigner ssh.Signer
}

func (r *rsaAlgorithm) setSig(b []byte) error {
	n, err := r.Write(b)
	if err != nil {
		r.Reset()
		return err
	} else if n != len(b) {
		r.Reset()
		return fmt.Errorf("could only write %d of %d bytes of signature to hash", n, len(b))
	}
	return nil
}

// Code from https://github.com/cloudtools/ssh-cert-authority/pull/49/files
// This interface provides a way to reach the exported, but not accessible SignWithOpts() method
// in x/crypto/ssh/agent. Access to this is needed to sign with more secure signing algorithms
type agentKeyringSigner interface {
	SignWithOpts(rand io.Reader, data []byte, opts crypto.SignerOpts) (*ssh.Signature, error)
}

// A struct to wrap an SSH Signer with one that will switch to SHA256 Signatures.
// Replaces the call to Sign() with a call to SignWithOpts using HashFunc() algorithm.
type Sha256Signer struct {
	ssh.Signer
}

func (s Sha256Signer) HashFunc() crypto.Hash {
	return crypto.SHA256
}

func (s Sha256Signer) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	if aks, ok := s.Signer.(agentKeyringSigner); !ok {
		return nil, fmt.Errorf("ssh: can't wrap a non ssh agentKeyringSigner")
	} else {
		return aks.SignWithOpts(rand, data, s)
	}
}

func (r *rsaAlgorithm) Sign(rand io.Reader, p crypto.PrivateKey, sig []byte) ([]byte, error) {
	if r.sshSigner != nil {
		var (
			sshsig *ssh.Signature
			err    error
		)
		// are we using an SSH Agent
		if _, ok := r.sshSigner.(agentKeyringSigner); ok {
			signer := Sha256Signer{r.sshSigner}
			sshsig, err = signer.Sign(rand, sig)
			if err != nil {
				return nil, err
			}
		} else {
			sshsig, err = r.sshSigner.(ssh.AlgorithmSigner).SignWithAlgorithm(rand, sig, ssh.SigAlgoRSASHA2256)
			if err != nil {
				return nil, err
			}
		}

		return sshsig.Blob, nil
	}
	defer r.Reset()

	if err := r.setSig(sig); err != nil {
		return nil, err
	}
	rsaK, ok := p.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("crypto.PrivateKey is not *rsa.PrivateKey")
	}
	return rsa.SignPKCS1v15(rand, rsaK, r.kind, r.Sum(nil))
}

func (r *rsaAlgorithm) Verify(pub crypto.PublicKey, toHash, signature []byte) error {
	defer r.Reset()
	rsaK, ok := pub.(*rsa.PublicKey)
	if !ok {
		return errors.New("crypto.PublicKey is not *rsa.PublicKey")
	}
	if err := r.setSig(toHash); err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(rsaK, r.kind, r.Sum(nil), signature)
}

func (r *rsaAlgorithm) String() string {
	return fmt.Sprintf("%s-%s", rsaPrefix, hashToDef[r.kind].name)
}

var _ signer = &ed25519Algorithm{}

type ed25519Algorithm struct {
	sshSigner ssh.Signer
}

func (r *ed25519Algorithm) Sign(rand io.Reader, p crypto.PrivateKey, sig []byte) ([]byte, error) {
	if r.sshSigner != nil {
		sshsig, err := r.sshSigner.Sign(rand, sig)
		if err != nil {
			return nil, err
		}

		return sshsig.Blob, nil
	}
	ed25519K, ok := p.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("crypto.PrivateKey is not ed25519.PrivateKey")
	}
	return ed25519.Sign(ed25519K, sig), nil
}

func (r *ed25519Algorithm) Verify(pub crypto.PublicKey, toHash, signature []byte) error {
	ed25519K, ok := pub.(ed25519.PublicKey)
	if !ok {
		return errors.New("crypto.PublicKey is not ed25519.PublicKey")
	}

	if ed25519.Verify(ed25519K, toHash, signature) {
		return nil
	}

	return errors.New("ed25519 verify failed")
}

func (r *ed25519Algorithm) String() string {
	return fmt.Sprintf("%s", ed25519Prefix)
}

var _ signer = &ecdsaAlgorithm{}

type ecdsaAlgorithm struct {
	hash.Hash
	kind crypto.Hash
}

func (r *ecdsaAlgorithm) setSig(b []byte) error {
	n, err := r.Write(b)
	if err != nil {
		r.Reset()
		return err
	} else if n != len(b) {
		r.Reset()
		return fmt.Errorf("could only write %d of %d bytes of signature to hash", n, len(b))
	}
	return nil
}

type ECDSASignature struct {
	R, S *big.Int
}

func (r *ecdsaAlgorithm) Sign(rand io.Reader, p crypto.PrivateKey, sig []byte) ([]byte, error) {
	defer r.Reset()
	if err := r.setSig(sig); err != nil {
		return nil, err
	}
	ecdsaK, ok := p.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("crypto.PrivateKey is not *ecdsa.PrivateKey")
	}
	R, S, err := ecdsa.Sign(rand, ecdsaK, r.Sum(nil))
	if err != nil {
		return nil, err
	}

	signature := ECDSASignature{R: R, S: S}
	bytes, err := asn1.Marshal(signature)

	return bytes, err
}

func (r *ecdsaAlgorithm) Verify(pub crypto.PublicKey, toHash, signature []byte) error {
	defer r.Reset()
	ecdsaK, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("crypto.PublicKey is not *ecdsa.PublicKey")
	}
	if err := r.setSig(toHash); err != nil {
		return err
	}

	sig := new(ECDSASignature)
	_, err := asn1.Unmarshal(signature, sig)
	if err != nil {
		return err
	}

	if ecdsa.Verify(ecdsaK, r.Sum(nil), sig.R, sig.S) {
		return nil
	} else {
		return errors.New("Invalid signature")
	}
}

func (r *ecdsaAlgorithm) String() string {
	return fmt.Sprintf("%s-%s", ecdsaPrefix, hashToDef[r.kind].name)
}

var _ macer = &blakeMacAlgorithm{}

type blakeMacAlgorithm struct {
	fn   func(key []byte) (hash.Hash, error)
	kind crypto.Hash
}

func (r *blakeMacAlgorithm) Sign(sig, key []byte) ([]byte, error) {
	hs, err := r.fn(key)
	if err != nil {
		return nil, err
	}
	if err = setSig(hs, sig); err != nil {
		return nil, err
	}
	return hs.Sum(nil), nil
}

func (r *blakeMacAlgorithm) Equal(sig, actualMAC, key []byte) (bool, error) {
	hs, err := r.fn(key)
	if err != nil {
		return false, err
	}
	defer hs.Reset()
	err = setSig(hs, sig)
	if err != nil {
		return false, err
	}
	expected := hs.Sum(nil)
	return subtle.ConstantTimeCompare(actualMAC, expected) == 1, nil
}

func (r *blakeMacAlgorithm) String() string {
	return fmt.Sprintf("%s", hashToDef[r.kind].name)
}

func setSig(a hash.Hash, b []byte) error {
	n, err := a.Write(b)
	if err != nil {
		a.Reset()
		return err
	} else if n != len(b) {
		a.Reset()
		return fmt.Errorf("could only write %d of %d bytes of signature to hash", n, len(b))
	}
	return nil
}

// IsSupportedHttpSigAlgorithm returns true if the string is supported by this
// library, is not a hash known to be weak, and is supported by the hardware.
func IsSupportedHttpSigAlgorithm(algo string) bool {
	a, err := isAvailable(algo)
	return a && err == nil
}

// isAvailable is an internally public function
func isAvailable(algo string) (bool, error) {
	c, ok := stringToHash[algo]
	if !ok {
		return false, fmt.Errorf("no match for %q", algo)
	}
	if isForbiddenHash(c) {
		return false, fmt.Errorf("forbidden hash type in %q", algo)
	}
	return c.Available(), nil
}

func newAlgorithmConstructor(algo string) (fn func(k []byte) (hash.Hash, error), c crypto.Hash, e error) {
	ok := false
	c, ok = stringToHash[algo]
	if !ok {
		e = fmt.Errorf("no match for %q", algo)
		return
	}
	if isForbiddenHash(c) {
		e = fmt.Errorf("forbidden hash type in %q", algo)
		return
	}
	algoDef, ok := hashToDef[c]
	if !ok {
		e = fmt.Errorf("have crypto.Hash %v but no definition", c)
		return
	}
	fn = func(key []byte) (hash.Hash, error) {
		h, err := algoDef.new(key)
		if err != nil {
			return nil, err
		}
		return h, nil
	}
	return
}

func newAlgorithm(algo string, key []byte) (hash.Hash, crypto.Hash, error) {
	fn, c, err := newAlgorithmConstructor(algo)
	if err != nil {
		return nil, c, err
	}
	h, err := fn(key)
	return h, c, err
}

func signerFromSSHSigner(sshSigner ssh.Signer, s string) (signer, error) {
	switch {
	case strings.HasPrefix(s, rsaPrefix):
		return &rsaAlgorithm{
			sshSigner: sshSigner,
		}, nil
	case strings.HasPrefix(s, ed25519Prefix):
		return &ed25519Algorithm{
			sshSigner: sshSigner,
		}, nil
	default:
		return nil, fmt.Errorf("no signer matching %q", s)
	}
}

// signerFromString is an internally public method constructor
func signerFromString(s string) (signer, error) {
	s = strings.ToLower(s)
	isEcdsa := false
	isEd25519 := false
	var algo string = ""
	if strings.HasPrefix(s, ecdsaPrefix) {
		algo = strings.TrimPrefix(s, ecdsaPrefix+"-")
		isEcdsa = true
	} else if strings.HasPrefix(s, rsaPrefix) {
		algo = strings.TrimPrefix(s, rsaPrefix+"-")
	} else if strings.HasPrefix(s, ed25519Prefix) {
		isEd25519 = true
		algo = "sha512"
	} else {
		return nil, fmt.Errorf("no signer matching %q", s)
	}
	hash, cHash, err := newAlgorithm(algo, nil)
	if err != nil {
		return nil, err
	}
	if isEd25519 {
		return &ed25519Algorithm{}, nil
	}
	if isEcdsa {
		return &ecdsaAlgorithm{
			Hash: hash,
			kind: cHash,
		}, nil
	}
	return &rsaAlgorithm{
		Hash: hash,
		kind: cHash,
	}, nil
}

// macerFromString is an internally public method constructor
func macerFromString(s string) (macer, error) {
	s = strings.ToLower(s)
	if strings.HasPrefix(s, hmacPrefix) {
		algo := strings.TrimPrefix(s, hmacPrefix+"-")
		hashFn, cHash, err := newAlgorithmConstructor(algo)
		if err != nil {
			return nil, err
		}
		// Ensure below does not panic
		_, err = hashFn(nil)
		if err != nil {
			return nil, err
		}
		return &hmacAlgorithm{
			fn: func(key []byte) (hash.Hash, error) {
				return hmac.New(func() hash.Hash {
					h, e := hashFn(nil)
					if e != nil {
						panic(e)
					}
					return h
				}, key), nil
			},
			kind: cHash,
		}, nil
	} else if bl, ok := stringToHash[s]; ok && blake2Algorithms[bl] {
		hashFn, cHash, err := newAlgorithmConstructor(s)
		if err != nil {
			return nil, err
		}
		return &blakeMacAlgorithm{
			fn:   hashFn,
			kind: cHash,
		}, nil
	} else {
		return nil, fmt.Errorf("no MACer matching %q", s)
	}
}
