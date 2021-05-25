module github.com/tektoncd/pipeline

go 1.13

require (
	github.com/cloudevents/sdk-go/v2 v2.1.0
	github.com/docker/cli v20.10.2+incompatible // indirect
	github.com/docker/docker v20.10.2+incompatible // indirect
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.20.2
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.5.5
	github.com/google/go-containerregistry v0.4.1-0.20210128200529-19c2b639fab1
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20210129212729-5c4818de4025
	github.com/google/uuid v1.2.0
	github.com/googleapis/gnostic v0.5.3 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/jenkins-x/go-scm v1.5.117
	github.com/mitchellh/go-homedir v1.1.0
	github.com/nbio/st v0.0.0-20140626010706-e9e8d9816f32 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/tektoncd/plumbing v0.0.0-20210514044347-f8a9689d5bd5
	go.opencensus.io v0.23.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/oauth2 v0.0.0-20210126194326-f9ce19ea3013
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	golang.org/x/tools v0.1.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/code-generator v0.19.7
	k8s.io/gengo v0.0.0-20201214224949-b6c5ce23f027 // indirect
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20210113233702-8566a335510f
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	knative.dev/pkg v0.0.0-20210331065221-952fdd90dbb0
)

// Knative deps (release-0.20)
replace (
	contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.13.4
	github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v38.2.0+incompatible
)
