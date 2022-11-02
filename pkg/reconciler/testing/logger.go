package testing

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	filteredinformerfactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/injection"
	logtesting "knative.dev/pkg/logging/testing"

	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	// Import for creating fake filtered factory in the test
	_ "knative.dev/pkg/client/injection/kube/informers/factory/filtered/fake"
)

// SetupFakeContext sets up the Context and the fake filtered informers for the tests.
func SetupFakeContext(t *testing.T) (context.Context, []controller.Informer) {
	ctx, _, informer := setupFakeContextWithLabelKey(t)
	return WithLogger(ctx, t), informer
}

// SetupFakeCloudClientContext sets up the fakeclient to context
func SetupFakeCloudClientContext(ctx context.Context, expectedEventCount int) context.Context {
	cloudEventClientBehaviour := cloudevent.FakeClientBehaviour{
		SendSuccessfully: true,
	}
	return cloudevent.WithClient(ctx, &cloudEventClientBehaviour, expectedEventCount)
}

// SetupDefaultContext sets up the Context and the default filtered informers for the tests.
func SetupDefaultContext(t *testing.T) (context.Context, []controller.Informer) {
	ctx, _, informer := setupDefaultContextWithLabelKey(t)
	return WithLogger(ctx, t), informer
}

// WithLogger returns the the Logger
func WithLogger(ctx context.Context, t *testing.T) context.Context {
	return logging.WithLogger(ctx, TestLogger(t))
}

// TestLogger sets up the the Logger
func TestLogger(t *testing.T) *zap.SugaredLogger {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
	return logger.Sugar().Named(t.Name())
}

// setupFakeContextWithLabelKey sets up the Context and the fake informers for the tests
// The provided context includes the FilteredInformerFactory LabelKey.
func setupFakeContextWithLabelKey(t zaptest.TestingT) (context.Context, context.CancelFunc, []controller.Informer) {
	ctx, c := context.WithCancel(logtesting.TestContextWithLogger(t))
	ctx = controller.WithEventRecorder(ctx, record.NewFakeRecorder(1000))
	ctx = filteredinformerfactory.WithSelectors(ctx, v1beta1.ManagedByLabelKey)
	ctx, is := injection.Fake.SetupInformers(ctx, &rest.Config{})
	return ctx, c, is
}

// setupDefaultContextWithLabelKey sets up the Context and the default informers for the tests
// The provided context includes the FilteredInformerFactory LabelKey.
func setupDefaultContextWithLabelKey(t zaptest.TestingT) (context.Context, context.CancelFunc, []controller.Informer) {
	ctx, c := context.WithCancel(logtesting.TestContextWithLogger(t))
	ctx = filteredinformerfactory.WithSelectors(ctx, v1beta1.ManagedByLabelKey)
	ctx, is := injection.Default.SetupInformers(ctx, &rest.Config{})
	return ctx, c, is
}
