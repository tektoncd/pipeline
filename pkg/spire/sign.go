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
	"encoding/base64"
	"encoding/pem"
	"errors"

	spiresigning "github.com/tektoncd/pipeline/internal/spire"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// AppendStatusInternalAnnotation creates the status annotations which are used by the controller to verify the status hash
func (sc *spireControllerAPIClient) AppendStatusInternalAnnotation(ctx context.Context, tr *v1beta1.TaskRun) error {
	err := sc.setupClient(ctx)
	if err != nil {
		return err
	}

	// Add status hash
	currentHash, err := hashTaskrunStatusInternal(tr)
	if err != nil {
		return err
	}

	// Sign with controller private key
	xsvid, err := sc.fetchControllerSVID(ctx)
	if err != nil {
		return err
	}

	sig, err := spiresigning.SignWithKey(xsvid, currentHash)
	if err != nil {
		return err
	}

	if len(xsvid.Certificates) == 0 {
		return errors.New("returned controller svid does not have certificates")
	}
	// Store Controller SVID
	p := pem.EncodeToMemory(&pem.Block{
		Bytes: xsvid.Certificates[0].Raw,
		Type:  "CERTIFICATE",
	})
	if tr.Status.Annotations == nil {
		tr.Status.Annotations = map[string]string{}
	}
	tr.Status.Annotations[controllerSvidAnnotation] = string(p)
	tr.Status.Annotations[TaskRunStatusHashAnnotation] = currentHash
	tr.Status.Annotations[taskRunStatusHashSigAnnotation] = base64.StdEncoding.EncodeToString(sig)
	return nil
}
