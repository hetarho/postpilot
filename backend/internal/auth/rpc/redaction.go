package rpc

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// SupplierRedaction clears provider prose from every response a non-master receives
// (QUOTA-66). A failure's technical detail can carry a supplier's own words — its account
// balance, its quota — and `Failure` rides inside dozens of messages, so the edge clears it
// once instead of trusting every projection to remember. It must sit inside the auth
// interceptor, which is what puts the caller's plan on the context; an unknown plan is
// redacted like any other non-master.
type SupplierRedaction struct{}

// NewSupplierRedaction returns the response-edge redaction interceptor.
func NewSupplierRedaction() SupplierRedaction { return SupplierRedaction{} }

func (SupplierRedaction) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		res, err := next(ctx, req)
		if err != nil || res == nil || auth.ActsAsMaster(ctx) {
			return res, err
		}
		if msg, ok := res.Any().(proto.Message); ok {
			clearProviderProse(msg.ProtoReflect())
		}
		return res, nil
	}
}

// The product has no streaming procedure; a stream added later passes through unredacted
// only until it is given the same treatment, which the descriptor test does not cover.
func (SupplierRedaction) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (SupplierRedaction) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

var (
	failureMessage  = (&postpilotv1.Failure{}).ProtoReflect().Descriptor()
	technicalDetail = failureMessage.Fields().ByName("technical_detail")
)

// clearProviderProse walks the populated fields only, so a response with no failure costs
// one pass over what it already carries.
func clearProviderProse(m protoreflect.Message) {
	if m.Descriptor().FullName() == failureMessage.FullName() {
		m.Clear(technicalDetail)
		return
	}
	m.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch {
		case field.IsList():
			if field.Message() != nil {
				list := value.List()
				for i := 0; i < list.Len(); i++ {
					clearProviderProse(list.Get(i).Message())
				}
			}
		case field.IsMap():
			if field.MapValue().Message() != nil {
				value.Map().Range(func(_ protoreflect.MapKey, entry protoreflect.Value) bool {
					clearProviderProse(entry.Message())
					return true
				})
			}
		case field.Message() != nil:
			clearProviderProse(value.Message())
		}
		return true
	})
}
