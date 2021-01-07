package zapdriver

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const operationKey = "logging.googleapis.com/operation"

// Operation adds the correct Stackdriver "operation" field.
//
// Additional information about a potentially long-running operation with which
// a log entry is associated.
//
// see: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogEntryOperation
func Operation(id, producer string, first, last bool) zap.Field {
	op := &operation{
		ID:       id,
		Producer: producer,
		First:    first,
		Last:     last,
	}

	return zap.Object(operationKey, op)
}

// OperationStart is a convenience function for `Operation`. It should be called
// for the first operation log.
func OperationStart(id, producer string) zap.Field {
	return Operation(id, producer, true, false)
}

// OperationCont is a convenience function for `Operation`. It should be called
// for any non-start/end operation log.
func OperationCont(id, producer string) zap.Field {
	return Operation(id, producer, false, false)
}

// OperationEnd is a convenience function for `Operation`. It should be called
// for the last operation log.
func OperationEnd(id, producer string) zap.Field {
	return Operation(id, producer, false, true)
}

// operation is the complete payload that can be interpreted by Stackdriver as
// an operation.
type operation struct {
	// Optional. An arbitrary operation identifier. Log entries with the same
	// identifier are assumed to be part of the same operation.
	ID string `json:"id"`

	// Optional. An arbitrary producer identifier. The combination of id and
	// producer must be globally unique. Examples for producer:
	// "MyDivision.MyBigCompany.com", "github.com/MyProject/MyApplication".
	Producer string `json:"producer"`

	// Optional. Set this to True if this is the first log entry in the operation.
	First bool `json:"first"`

	// Optional. Set this to True if this is the last log entry in the operation.
	Last bool `json:"last"`
}

// MarshalLogObject implements zapcore.ObjectMarshaller interface.
func (op operation) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("id", op.ID)
	enc.AddString("producer", op.Producer)
	enc.AddBool("first", op.First)
	enc.AddBool("last", op.Last)

	return nil
}
