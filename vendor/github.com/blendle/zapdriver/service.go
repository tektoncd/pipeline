package zapdriver

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const serviceContextKey = "serviceContext"

// ServiceContext adds the correct service information adding the log line
// It is a required field if an error needs to be reported.
//
// see: https://cloud.google.com/error-reporting/reference/rest/v1beta1/ServiceContext
// see: https://cloud.google.com/error-reporting/docs/formatting-error-messages
func ServiceContext(name string) zap.Field {
	return zap.Object(serviceContextKey, newServiceContext(name))
}

// serviceContext describes a running service that sends errors.
// Currently it only describes a service name.
type serviceContext struct {
	Name string `json:"service"`
}

// MarshalLogObject implements zapcore.ObjectMarshaller interface.
func (service_context serviceContext) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("service", service_context.Name)

	return nil
}

func newServiceContext(name string) *serviceContext {
	return &serviceContext{
		Name: name,
	}
}
