package authorization

import (
	"errors"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// AddUserToSAR adds the requisite user information to a SubjectAccessReview.
// It returns the modified SubjectAccessReview.
func AddUserToSAR(saName, saNamespace string, sar *authorizationv1.SubjectAccessReview) *authorizationv1.SubjectAccessReview {
	sar.Spec.User = fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, saName)

	return sar
}

// Authorize verifies that a given user is permitted to carry out a given
// action.  If this cannot be determined, or if the user is not permitted, an
// error is returned.
func AuthorizeSAR(sarClient authorizationclient.SubjectAccessReviewInterface, saName, saNamespace string,
	resourceAttributes *authorizationv1.ResourceAttributes) error {
	sar := AddUserToSAR(saName, saNamespace, &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: resourceAttributes,
		},
	})

	resp, err := sarClient.Create(sar)
	if err == nil && resp != nil && resp.Status.Allowed {
		return nil
	}

	if err == nil {
		err = errors.New(resp.Status.Reason)
	}
	return kerrors.NewForbidden(schema.GroupResource{Group: resourceAttributes.Group, Resource: resourceAttributes.Resource}, resourceAttributes.Name, err)
}
