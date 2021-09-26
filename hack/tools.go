// +build tools

package tools

import (
	_ "github.com/tektoncd/plumbing"
	_ "github.com/tektoncd/plumbing/cmd/combine"
	_ "github.com/tektoncd/plumbing/scripts"

	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "k8s.io/code-generator/cmd/defaulter-gen"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"

	_ "knative.dev/pkg/codegen/cmd/injection-gen"
)
