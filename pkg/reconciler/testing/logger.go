package testing

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"go.uber.org/zap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func SetupFakeContext(t *testing.T) (context.Context, []controller.Informer) {
	ctx, informer := rtesting.SetupFakeContext(t)
	cloudEventClientBehaviour := cloudevent.FakeClientBehaviour{
		SendSuccessfully: true,
	}
	ctx = cloudevent.WithClient(ctx, &cloudEventClientBehaviour)
	return WithLogger(ctx, t), informer
}

func WithLogger(ctx context.Context, t *testing.T) context.Context {
	return logging.WithLogger(ctx, TestLogger(t))
}

func TestLogger(t *testing.T) *zap.SugaredLogger {
	logger, err := zap.NewDevelopment(zap.AddCaller())
	if err != nil {
		t.Fatalf("failed to create logger: %s", err)
	}
	return logger.Sugar().Named(t.Name())
}
