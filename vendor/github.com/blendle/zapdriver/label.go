package zapdriver

import (
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const labelsKey = "logging.googleapis.com/labels"

// Label adds an optional label to the payload.
//
// Labels are a set of user-defined (key, value) data that provides additional
// information about the log entry.
//
// Example: { "name": "wrench", "mass": "1.3kg", "count": "3" }.
func Label(key, value string) zap.Field {
	return zap.String("labels."+key, value)
}

// Labels takes Zap fields, filters the ones that have their key start with the
// string `labels.` and their value type set to StringType. It then wraps those
// key/value pairs in a top-level `labels` namespace.
func Labels(fields ...zap.Field) zap.Field {
	lbls := newLabels()

	lbls.mutex.Lock()
	for i := range fields {
		if isLabelField(fields[i]) {
			lbls.store[strings.Replace(fields[i].Key, "labels.", "", 1)] = fields[i].String
		}
	}
	lbls.mutex.Unlock()

	return labelsField(lbls)
}

func isLabelField(field zap.Field) bool {
	return strings.HasPrefix(field.Key, "labels.") && field.Type == zapcore.StringType
}

func labelsField(l *labels) zap.Field {
	return zap.Object(labelsKey, l)
}

type labels struct {
	store map[string]string
	mutex *sync.RWMutex
}

func newLabels() *labels {
	return &labels{store: map[string]string{}, mutex: &sync.RWMutex{}}
}

func (l *labels) Add(key, value string) {
	l.mutex.Lock()
	l.store[key] = value
	l.mutex.Unlock()
}

func (l *labels) reset() {
	l.mutex.Lock()
	l.store = map[string]string{}
	l.mutex.Unlock()
}

func (l labels) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	l.mutex.RLock()
	for k, v := range l.store {
		enc.AddString(k, v)
	}
	l.mutex.RUnlock()

	return nil
}
