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
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/result"
	"go.uber.org/zap"
)

// VerifyTaskRunResults ensures that the TaskRun results are valid and have not been tampered with
func (sc *spireControllerAPIClient) VerifyTaskRunResults(ctx context.Context, prs []result.RunResult, tr *v1beta1.TaskRun) error {
	err := sc.setupClient(ctx)
	if err != nil {
		return err
	}

	resultMap := map[string]result.RunResult{}
	for _, r := range prs {
		if r.ResultType == result.TaskRunResultType {
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

// VerifyStatusInternalAnnotation run multuple verification steps to ensure that the spire status annotations are valid
func (sc *spireControllerAPIClient) VerifyStatusInternalAnnotation(ctx context.Context, tr *v1beta1.TaskRun, logger *zap.SugaredLogger) error {
	err := sc.setupClient(ctx)
	if err != nil {
		return err
	}

	if !sc.CheckSpireVerifiedFlag(tr) {
		return errors.New("annotation tekton.dev/not-verified = yes failed spire verification")
	}

	annotations := tr.Status.Annotations

	// get trust bundle from spire server
	trust, err := getTrustBundle(ctx, sc.workloadAPI)
	if err != nil {
		return err
	}

	// verify controller SVID
	svid, ok := annotations[controllerSvidAnnotation]
	if !ok {
		return errors.New("No SVID found")
	}
	block, _ := pem.Decode([]byte(svid))
	if block == nil {
		return fmt.Errorf("invalid SVID: %w", err)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("invalid SVID: %w", err)
	}

	// verify certificate root of trust
	if err := verifyCertificateTrust(cert, trust); err != nil {
		return err
	}
	logger.Infof("Successfully verified certificate %s against SPIRE", svid)

	if err := verifyAnnotation(cert.PublicKey, annotations); err != nil {
		return err
	}
	logger.Info("Successfully verified signature")

	// CheckStatusInternalAnnotation check current status hash vs annotation status hash by controller
	if err := CheckStatusInternalAnnotation(tr); err != nil {
		return err
	}
	logger.Info("Successfully verified status annotation hash matches the current taskrun status")

	return nil
}

// CheckSpireVerifiedFlag checks if the verified status annotation is set which would result in spire verification failed
func (sc *spireControllerAPIClient) CheckSpireVerifiedFlag(tr *v1beta1.TaskRun) bool {
	if _, ok := tr.Status.Annotations[VerifiedAnnotation]; !ok {
		return true
	}
	return false
}

func hashTaskrunStatusInternal(tr *v1beta1.TaskRun) (string, error) {
	s, err := json.Marshal(tr.Status.TaskRunStatusFields)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(s)), nil
}

// CheckStatusInternalAnnotation ensures that the internal status annotation hash and current status hash match
func CheckStatusInternalAnnotation(tr *v1beta1.TaskRun) error {
	// get stored hash of status
	annotations := tr.Status.Annotations
	hash, ok := annotations[TaskRunStatusHashAnnotation]
	if !ok {
		return fmt.Errorf("no annotation status hash found for %s", TaskRunStatusHashAnnotation)
	}
	// get current hash of status
	current, err := hashTaskrunStatusInternal(tr)
	if err != nil {
		return err
	}
	if hash != current {
		return fmt.Errorf("current status hash and stored annotation hash does not match! Annotation Hash: %s, Current Status Hash: %s", hash, current)
	}

	return nil
}

func getSVID(resultMap map[string]result.RunResult) (*x509.Certificate, error) {
	svid, ok := resultMap[KeySVID]
	if !ok {
		return nil, errors.New("no SVID found")
	}
	svidValue, err := getResultValue(svid)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(svidValue))
	if block == nil {
		return nil, fmt.Errorf("invalid SVID: %w", err)
	}
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
		for _, bundle := range x509Bundle {
			for _, c := range bundle.X509Authorities() {
				trustPool.AddCert(c)
			}
			return trustPool, nil
		}
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

func verifyManifest(results map[string]result.RunResult) error {
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

func verifyAnnotation(pub interface{}, annotations map[string]string) error {
	signature, ok := annotations[taskRunStatusHashSigAnnotation]
	if !ok {
		return fmt.Errorf("no signature found for %s", taskRunStatusHashSigAnnotation)
	}
	hash, ok := annotations[TaskRunStatusHashAnnotation]
	if !ok {
		return fmt.Errorf("no annotation status hash found for %s", TaskRunStatusHashAnnotation)
	}
	return verifySignature(pub, signature, hash)
}

func verifyResult(pub crypto.PublicKey, key string, results map[string]result.RunResult) error {
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

func getResultValue(result result.RunResult) (string, error) {
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
		keys := make([]string, 0, len(aos.ObjectVal))
		for k := range aos.ObjectVal {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			valList = append(valList, k)
			valList = append(valList, aos.ObjectVal[k])
		}
		return strings.Join(valList, ","), nil
	}
	return "", fmt.Errorf("invalid result type for key: %s", result.Key)
}
