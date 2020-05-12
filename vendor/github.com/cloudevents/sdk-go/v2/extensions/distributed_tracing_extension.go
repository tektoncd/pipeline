package extensions

import (
	"context"
	"reflect"
	"strings"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/cloudevents/sdk-go/v2/types"
	"github.com/lightstep/tracecontext.go/traceparent"
	"github.com/lightstep/tracecontext.go/tracestate"
	"go.opencensus.io/trace"
	octs "go.opencensus.io/trace/tracestate"
)

const (
	TraceParentExtension = "traceparent"
	TraceStateExtension  = "tracestate"
)

// DistributedTracingExtension represents the extension for cloudevents context
type DistributedTracingExtension struct {
	TraceParent string `json:"traceparent"`
	TraceState  string `json:"tracestate"`
}

// AddTracingAttributes adds the tracing attributes traceparent and tracestate to the cloudevents context
func (d DistributedTracingExtension) AddTracingAttributes(e event.EventWriter) {
	if d.TraceParent != "" {
		value := reflect.ValueOf(d)
		typeOf := value.Type()

		for i := 0; i < value.NumField(); i++ {
			k := strings.ToLower(typeOf.Field(i).Name)
			v := value.Field(i).Interface()
			if k == TraceStateExtension && v == "" {
				continue
			}
			e.SetExtension(k, v)
		}
	}
}

func GetDistributedTracingExtension(event event.Event) (DistributedTracingExtension, bool) {
	if tp, ok := event.Extensions()[TraceParentExtension]; ok {
		if tpStr, err := types.ToString(tp); err == nil {
			var tsStr string
			if ts, ok := event.Extensions()[TraceStateExtension]; ok {
				tsStr, _ = types.ToString(ts)
			}
			return DistributedTracingExtension{TraceParent: tpStr, TraceState: tsStr}, true
		}
	}
	return DistributedTracingExtension{}, false
}

func (d *DistributedTracingExtension) ReadTransformer() binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		tp := reader.GetExtension(TraceParentExtension)
		if tp != nil {
			tpFormatted, err := types.Format(tp)
			if err != nil {
				return err
			}
			d.TraceParent = tpFormatted
		}
		ts := reader.GetExtension(TraceStateExtension)
		if ts != nil {
			tsFormatted, err := types.Format(ts)
			if err != nil {
				return err
			}
			d.TraceState = tsFormatted
		}
		return nil
	}
}

func (d *DistributedTracingExtension) WriteTransformer() binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		err := writer.SetExtension(TraceParentExtension, d.TraceParent)
		if err != nil {
			return nil
		}
		if d.TraceState != "" {
			return writer.SetExtension(TraceStateExtension, d.TraceState)
		}
		return nil
	}
}

// FromSpanContext populates DistributedTracingExtension from a SpanContext.
func FromSpanContext(sc trace.SpanContext) DistributedTracingExtension {
	tp := traceparent.TraceParent{
		TraceID: sc.TraceID,
		SpanID:  sc.SpanID,
		Flags: traceparent.Flags{
			Recorded: sc.IsSampled(),
		},
	}

	entries := make([]string, 0, len(sc.Tracestate.Entries()))
	for _, entry := range sc.Tracestate.Entries() {
		entries = append(entries, strings.Join([]string{entry.Key, entry.Value}, "="))
	}

	return DistributedTracingExtension{
		TraceParent: tp.String(),
		TraceState:  strings.Join(entries, ","),
	}
}

// ToSpanContext creates a SpanContext from a DistributedTracingExtension instance.
func (d DistributedTracingExtension) ToSpanContext() (trace.SpanContext, error) {
	tp, err := traceparent.ParseString(d.TraceParent)
	if err != nil {
		return trace.SpanContext{}, err
	}
	sc := trace.SpanContext{
		TraceID: tp.TraceID,
		SpanID:  tp.SpanID,
	}
	if tp.Flags.Recorded {
		sc.TraceOptions |= 1
	}

	if ts, err := tracestate.ParseString(d.TraceState); err == nil {
		entries := make([]octs.Entry, 0, len(ts))
		for _, member := range ts {
			var key string
			if member.Tenant != "" {
				// Due to github.com/lightstep/tracecontext.go/issues/6,
				// the meaning of Vendor and Tenant are swapped here.
				key = member.Vendor + "@" + member.Tenant
			} else {
				key = member.Vendor
			}
			entries = append(entries, octs.Entry{Key: key, Value: member.Value})
		}
		sc.Tracestate, _ = octs.New(nil, entries...)
	}

	return sc, nil
}

func (d DistributedTracingExtension) StartChildSpan(ctx context.Context, name string, opts ...trace.StartOption) (context.Context, *trace.Span) {
	if sc, err := d.ToSpanContext(); err == nil {
		tSpan := trace.FromContext(ctx)
		ctx, span := trace.StartSpanWithRemoteParent(ctx, name, sc, opts...)
		if tSpan != nil {
			// Add link to the previous in-process trace.
			tsc := tSpan.SpanContext()
			span.AddLink(trace.Link{
				TraceID: tsc.TraceID,
				SpanID:  tsc.SpanID,
				Type:    trace.LinkTypeParent,
			})
		}
		return ctx, span
	}
	return ctx, nil
}
