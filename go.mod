module github.com/tektoncd/cli

go 1.12

require (
	cloud.google.com/go v0.37.2 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.2.0 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.1.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.9.1 // indirect
	github.com/AlecAivazis/survey/v2 v2.0.4
	github.com/Azure/azure-sdk-for-go v26.1.0+incompatible // indirect
	github.com/Azure/go-autorest v11.6.0+incompatible // indirect
	github.com/Netflix/go-expect v0.0.0-20190729225929-0e00d9168667
	github.com/aws/aws-sdk-go v1.19.11 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/census-instrumentation/opencensus-proto v0.1.0 // indirect
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/fatih/color v1.7.0
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gogo/protobuf v1.2.0 // indirect
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/google/go-cmp v0.3.1
	github.com/google/go-containerregistry v0.0.0-20190320210540-8d4083db9aa0 // indirect
	github.com/google/gofuzz v0.0.0-20161122191042-44d81051d367 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d // indirect
	github.com/hako/durafmt v0.0.0-20180520121703-7b7ae1e72ead
	github.com/hinshun/vt10x v0.0.0-20180809195222-d55458df857c
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/jonboulle/clockwork v0.1.1-0.20190114141812-62fb9bc030d1
	github.com/json-iterator/go v0.0.0-20180612202835-f2b4162afba3 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/kr/pty v1.1.8 // indirect
	github.com/markbates/inflect v1.0.4 // indirect
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742 // indirect
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829 // indirect
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/procfs v0.0.0-20190322151404-55ae3d9d5573 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	github.com/tektoncd/pipeline v0.7.0
	github.com/tektoncd/plumbing v0.0.0-20190604151109-373083123d6a
	go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v0.0.0-20180814183419-67bc79d13d15 // indirect
	golang.org/x/crypto v0.0.0-20190923035154-9ee001bba392 // indirect
	golang.org/x/net v0.0.0-20190923162816-aa69164e4478 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20190924154521-2837fb4f24fe // indirect
	golang.org/x/text v0.3.2 // indirect
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7 // indirect
	google.golang.org/appengine v1.5.0 // indirect
	google.golang.org/grpc v1.19.1 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	k8s.io/api v0.0.0-20190226173710-145d52631d00
	k8s.io/apimachinery v0.0.0-20190221084156-01f179d85dbc
	k8s.io/cli-runtime v0.0.0-20190325194458-f2b4781c3ae1
	k8s.io/client-go v0.0.0-20190226174127-78295b709ec6
	k8s.io/klog v0.2.0 // indirect
	k8s.io/kube-openapi v0.0.0-20171101183504-39a7bf85c140 // indirect
	k8s.io/kubernetes v1.13.3 // indirect
	knative.dev/pkg v0.0.0-20190719141030-e4bc08cc8ded
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace github.com/kr/pty => github.com/creack/pty v1.1.7

replace github.com/spf13/cobra => github.com/chmouel/cobra v0.0.0-20190820110723-8f09bb39b20d

replace git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999
