// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"crypto"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/42wim/httpsig"
	legacyhttpsig "github.com/go-fed/httpsig"
	"golang.org/x/crypto/ssh"
)

// HTTPSign contains the signer used for signing requests
type HTTPSign struct {
	ssh.Signer
	cert bool
}

// HTTPSignConfig contains the configuration for creating a HTTPSign
type HTTPSignConfig struct {
	fingerprint string
	principal   string
	pubkey      bool
	cert        bool
	sshKey      string
	passphrase  string
}

// NewHTTPSignWithPubkey can be used to create a HTTPSign with a public key
// if no fingerprint is specified it returns the first public key found
func NewHTTPSignWithPubkey(fingerprint, sshKey, passphrase string) (*HTTPSign, error) {
	return newHTTPSign(&HTTPSignConfig{
		fingerprint: fingerprint,
		pubkey:      true,
		sshKey:      sshKey,
		passphrase:  passphrase,
	})
}

// NewHTTPSignWithCert can be used to create a HTTPSign with a certificate
// if no principal is specified it returns the first certificate found
func NewHTTPSignWithCert(principal, sshKey, passphrase string) (*HTTPSign, error) {
	return newHTTPSign(&HTTPSignConfig{
		principal:  principal,
		cert:       true,
		sshKey:     sshKey,
		passphrase: passphrase,
	})
}

// NewHTTPSign returns a new HTTPSign
// It will check the ssh-agent or a local file is config.sshKey is set.
// Depending on the configuration it will either use a certificate or a public key
func newHTTPSign(config *HTTPSignConfig) (*HTTPSign, error) {
	var signer ssh.Signer

	if config.sshKey != "" {
		priv, err := os.ReadFile(config.sshKey)
		if err != nil {
			return nil, err
		}

		if config.passphrase == "" {
			signer, err = ssh.ParsePrivateKey(priv)
			if err != nil {
				return nil, err
			}
		} else {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(priv, []byte(config.passphrase))
			if err != nil {
				return nil, err
			}
		}

		if config.cert {
			certbytes, err := os.ReadFile(config.sshKey + "-cert.pub")
			if err != nil {
				return nil, err
			}

			pub, _, _, _, err := ssh.ParseAuthorizedKey(certbytes)
			if err != nil {
				return nil, err
			}

			cert, ok := pub.(*ssh.Certificate)
			if !ok {
				return nil, fmt.Errorf("failed to parse certificate")
			}

			signer, err = ssh.NewCertSigner(cert, signer)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// if no sshKey is specified, check if we have a ssh-agent and use it
		agent, err := GetAgent()
		if err != nil {
			return nil, err
		}

		signers, err := agent.Signers()
		if err != nil {
			return nil, err
		}

		if len(signers) == 0 {
			return nil, fmt.Errorf("no signers found")
		}

		if config.cert {
			signer = findCertSigner(signers, config.principal)
			if signer == nil {
				return nil, fmt.Errorf("no certificate found for %s", config.principal)
			}
		}

		if config.pubkey {
			signer = findPubkeySigner(signers, config.fingerprint)
			if signer == nil {
				return nil, fmt.Errorf("no public key found for %s", config.fingerprint)
			}
		}
	}

	return &HTTPSign{
		Signer: signer,
		cert:   config.cert,
	}, nil
}

// SignRequest signs a HTTP request
func (c *Client) SignRequest(r *http.Request) error {
	var contents []byte

	headersToSign := []string{httpsig.RequestTarget, "(created)", "(expires)"}

	if c.httpsigner.cert {
		// add our certificate to the headers to sign
		pubkey, _ := ssh.ParsePublicKey(c.httpsigner.PublicKey().Marshal())
		if cert, ok := pubkey.(*ssh.Certificate); ok {
			certString := base64.RawStdEncoding.EncodeToString(cert.Marshal())
			r.Header.Add("x-ssh-certificate", certString)

			headersToSign = append(headersToSign, "x-ssh-certificate")
		} else {
			return fmt.Errorf("no ssh certificate found")
		}
	}

	// if we have a body, the Digest header will be added and we'll include this also in
	// our signature.
	if r.Body != nil {
		body, err := r.GetBody()
		if err != nil {
			return fmt.Errorf("getBody() failed: %s", err)
		}

		contents, err = io.ReadAll(body)
		if err != nil {
			return fmt.Errorf("failed reading body: %s", err)
		}

		headersToSign = append(headersToSign, "Digest")
	}

	// create a signer for the request and headers, the signature will be valid for 10 seconds
	var err error

	// use legacyhttpsig to sign with RSA-SHA1 on older gitea releases
	if err = c.checkServerVersionGreaterThanOrEqual(version1_23_0); err != nil {
		// Legacy signer
		legacySigner, _, err := legacyhttpsig.NewSSHSigner(c.httpsigner, httpsig.DigestSha512, headersToSign, legacyhttpsig.Signature, 10)
		if err != nil {
			return fmt.Errorf("legacy httpsig.NewSSHSigner failed: %s", err)
		}

		// sign the request, use the fingerprint if we don't have a certificate
		keyID := "gitea"
		if !c.httpsigner.cert {
			keyID = ssh.FingerprintSHA256(c.httpsigner.PublicKey())
		}

		return legacySigner.SignRequest(keyID, r, contents)
	}
	// Modern signer
	modernSigner, _, err := httpsig.NewSSHSigner(c.httpsigner, httpsig.DigestSha512, headersToSign, httpsig.Signature, 10)
	if err != nil {
		return fmt.Errorf("httpsig.NewSSHSigner failed: %s", err)
	}

	// sign the request, use the fingerprint if we don't have a certificate
	keyID := "gitea"
	if !c.httpsigner.cert {
		keyID = ssh.FingerprintSHA256(c.httpsigner.PublicKey())
	}

	return modernSigner.SignRequest(keyID, r, contents)
}

// findCertSigner returns the Signer containing a valid certificate
// if no principal is specified it returns the first certificate found
func findCertSigner(sshsigners []ssh.Signer, principal string) ssh.Signer {
	for _, s := range sshsigners {
		// Check if the key is a certificate
		if !strings.Contains(s.PublicKey().Type(), "cert-v01@openssh.com") {
			continue
		}

		// convert the ssh.Signer to a ssh.Certificate
		mpubkey, _ := ssh.ParsePublicKey(s.PublicKey().Marshal())
		cryptopub := mpubkey.(crypto.PublicKey)
		cert := cryptopub.(*ssh.Certificate)
		t := time.Unix(int64(cert.ValidBefore), 0)

		// make sure the certificate is at least 10 seconds valid
		if time.Until(t) <= time.Second*10 {
			continue
		}

		if principal == "" {
			return s
		}

		for _, p := range cert.ValidPrincipals {
			if p == principal {
				return s
			}
		}
	}

	return nil
}

// findPubkeySigner returns the Signer containing a valid public key
// if no fingerprint is specified it returns the first public key found
func findPubkeySigner(sshsigners []ssh.Signer, fingerprint string) ssh.Signer {
	for _, s := range sshsigners {
		// Check if the key is a certificate
		if strings.Contains(s.PublicKey().Type(), "cert-v01@openssh.com") {
			continue
		}

		if fingerprint == "" {
			return s
		}

		if strings.TrimSpace(string(ssh.MarshalAuthorizedKey(s.PublicKey()))) == fingerprint {
			return s
		}

		if ssh.FingerprintSHA256(s.PublicKey()) == fingerprint {
			return s
		}
	}

	return nil
}
