/*
Copyright 2022 The Tekton Authors

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

package spire

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// VerifyTaskRunResults ensures that the TaskRun results are valid and have not been tampered with
func (sc *spireControllerAPIClient) VerifyTaskRunResults(ctx context.Context, prs []v1beta1.PipelineResourceResult, tr *v1beta1.TaskRun) error {
	err := sc.setupClient(ctx)
	if err != nil {
		return err
	}

	resultMap := map[string]v1beta1.PipelineResourceResult{}
	for _, r := range prs {
		if r.ResultType == v1beta1.TaskRunResultType {
			resultMap[r.Key] = r
		}
	}

	cert, err := getSVID(resultMap)
	if err != nil {
		return err
	}

	trust, err := getTrustBundle(ctx, sc.workloadAPI)
	if err != nil {
		return err
	}

	if err := verifyManifest(resultMap); err != nil {
		return err
	}

	if err := verifyCertURI(cert, tr, sc.config.TrustDomain); err != nil {
		return err
	}

	if err := verifyCertificateTrust(cert, trust); err != nil {
		return err
	}

	for key := range resultMap {
		if strings.HasSuffix(key, KeySignatureSuffix) {
			continue
		}
		if key == KeySVID {
			continue
		}
		if err := verifyResult(cert.PublicKey, key, resultMap); err != nil {
			return err
		}
	}

	return nil
}

func getSVID(resultMap map[string]v1beta1.PipelineResourceResult) (*x509.Certificate, error) {
	svid, ok := resultMap[KeySVID]
	if !ok {
		return nil, errors.New("no SVID found")
	}
	svidValue, err := getResultValue(svid)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(svidValue))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid SVID: %w", err)
	}
	return cert, nil
}

func getTrustBundle(ctx context.Context, client *workloadapi.Client) (*x509.CertPool, error) {
	x509set, err := client.FetchX509Bundles(ctx)
	if err != nil {
		return nil, err
	}
	x509Bundle := x509set.Bundles()
	if err != nil {
		return nil, err
	}
	if len(x509Bundle) > 0 {
		trustPool := x509.NewCertPool()
		for _, c := range x509Bundle[0].X509Authorities() {
			trustPool.AddCert(c)
		}
		return trustPool, nil
	}
	return nil, errors.Wrap(err, "trust domain bundle empty")
}

func getFullPath(tr *v1beta1.TaskRun) string {
	// URI:spiffe://example.org/ns/default/taskrun/cache-image-pipelinerun-r4r22-fetch-from-git
	return fmt.Sprintf("/ns/%s/taskrun/%s", tr.Namespace, tr.Name)
}

func verifyCertURI(cert *x509.Certificate, tr *v1beta1.TaskRun, trustDomain string) error {
	path := getFullPath(tr)
	switch {
	case len(cert.URIs) == 0:
		return fmt.Errorf("cert uri missing for taskrun: %s", tr.Name)
	case len(cert.URIs) > 1:
		return fmt.Errorf("cert contains more than one URI for taskrun: %s", tr.Name)
	case len(cert.URIs) == 1:
		if cert.URIs[0].Host != trustDomain {
			return fmt.Errorf("cert uri: %s does not match trust domain: %s", cert.URIs[0].Host, trustDomain)
		}
		if cert.URIs[0].Path != path {
			return fmt.Errorf("cert uri: %s does not match taskrun: %s", cert.URIs[0].Path, path)
		}
	}
	return nil
}

func verifyCertificateTrust(cert *x509.Certificate, rootCertPool *x509.CertPool) error {
	verifyOptions := x509.VerifyOptions{
		Roots: rootCertPool,
	}
	chains, err := cert.Verify(verifyOptions)
	if len(chains) == 0 || err != nil {
		return errors.New("cert cannot be verified by provided roots")
	}
	return nil
}

func verifyManifest(results map[string]v1beta1.PipelineResourceResult) error {
	manifest, ok := results[KeyResultManifest]
	if !ok {
		return errors.New("no manifest found in results")
	}
	manifestValue, err := getResultValue(manifest)
	if err != nil {
		return err
	}
	s := strings.Split(manifestValue, ",")
	for _, key := range s {
		_, found := results[key]
		if key != "" && !found {
			return fmt.Errorf("no result found for %s but is part of the manifest %s", key, manifestValue)
		}
	}
	return nil
}

func verifyResult(pub crypto.PublicKey, key string, results map[string]v1beta1.PipelineResourceResult) error {
	signature, ok := results[key+KeySignatureSuffix]
	if !ok {
		return fmt.Errorf("no signature found for %s", key)
	}
	sigValue, err := getResultValue(signature)
	if err != nil {
		return err
	}
	resultValue, err := getResultValue(results[key])
	if err != nil {
		return err
	}
	return verifySignature(pub, sigValue, resultValue)
}

func verifySignature(pub crypto.PublicKey, signature string, value string) error {
	b, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}
	h := sha256.Sum256([]byte(value))
	// Check val against sig
	switch t := pub.(type) {
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(t, h[:], b) {
			return errors.New("invalid signature")
		}
		return nil
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(t, crypto.SHA256, h[:], b)
	case ed25519.PublicKey:
		if !ed25519.Verify(t, []byte(value), b) {
			return errors.New("invalid signature")
		}
		return nil
	default:
		return fmt.Errorf("unsupported key type: %s", t)
	}
}

func getResultValue(result v1beta1.PipelineResourceResult) (string, error) {
	aos := v1beta1.ArrayOrString{}
	err := aos.UnmarshalJSON([]byte(result.Value))
	valList := []string{}
	if err != nil {
		return "", fmt.Errorf("unmarshal error for key: %s", result.Key)
	}
	switch aos.Type {
	case v1beta1.ParamTypeString:
		return aos.StringVal, nil
	case v1beta1.ParamTypeArray:
		valList = append(valList, aos.ArrayVal...)
		return strings.Join(valList, ","), nil
	case v1beta1.ParamTypeObject:
		for _, v := range aos.ObjectVal {
			valList = append(valList, v)
		}
		return strings.Join(valList, ","), nil
	}
	return "", fmt.Errorf("invalid result type for key: %s", result.Key)
}
