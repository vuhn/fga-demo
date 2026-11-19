// Package grpcutil is what the API's gRPC clients (Engine, later Agent) share: the connection options, the
// deadline helper, the x-request-id metadata and the mapping from a gRPC status (or an in-band error event) to
// the API's errors. The only request metadata is the opaque request id; no tenant, person or actor identity is
// ever sent (docs/service-contracts.md).
package grpcutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	commonv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/common/v1"
)

// MaxMessage is the largest message either side accepts (the services' limit, 32 MB).
const MaxMessage = 32 << 20

const (
	// RequestIDKey is the one metadata key a client sends.
	RequestIDKey = "x-request-id"
	// ErrorDetailKey is the trailing metadata key carrying the binary ErrorDetail of a refusal.
	ErrorDetailKey = "error-detail-bin"
)

type requestIDKey struct{}

// WithRequestID returns a context whose calls carry id as x-request-id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID is the id set by WithRequestID, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// WithDeadline bounds ctx by d unless it already ends sooner. The caller must call the cancel func.
func WithDeadline(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) <= d {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

// Dial opens a client connection to addr (plaintext: the services share a private network) that sends
// x-request-id on every call and bounds a unary call without a deadline by defaultTimeout.
func Dial(addr string, defaultTimeout time.Duration, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	base := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessage), grpc.MaxCallSendMsgSize(MaxMessage)),
		grpc.WithChainUnaryInterceptor(unaryInterceptor(defaultTimeout)),
		grpc.WithChainStreamInterceptor(streamInterceptor()),
	}
	return grpc.NewClient(addr, append(base, opts...)...)
}

func withMetadata(ctx context.Context) context.Context {
	id := RequestID(ctx)
	if id == "" {
		id = newID()
	}
	return metadata.AppendToOutgoingContext(ctx, RequestIDKey, id)
}

func unaryInterceptor(defaultTimeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, cancel := WithDeadline(withMetadata(ctx), defaultTimeout)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func streamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(withMetadata(ctx), desc, cc, method, opts...)
	}
}

// FromStatus maps the error of a gRPC call to the API's errors. A refusal (INVALID_ARGUMENT) is ErrValidation
// with the engine's message; everything else (UNAVAILABLE, a missed deadline, a cancelled call, an unknown
// code) is ErrEngineUnavailable. service names the callee for the message ("engine", "agent"). detail is the
// ErrorDetail read from the call's trailer, or nil.
func FromStatus(service string, err error, detail *commonv1.ErrorDetail) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w: %s: %v", model.ErrEngineUnavailable, service, err)
	}
	if st.Code() == codes.InvalidArgument {
		msg := st.Message()
		if detail != nil && detail.Message != "" {
			msg = detail.Message
		}
		return model.Invalid("%s", msg)
	}
	msg := st.Message()
	if detail != nil && detail.Message != "" {
		msg = detail.Message
	}
	return fmt.Errorf("%w: %s %s: %s", model.ErrEngineUnavailable, service, st.Code(), msg)
}

// InBand maps an ErrorDetail that arrived as a stream event (after the first message, when a status can no
// longer be sent) the same way FromStatus maps a refusal or an outage.
func InBand(service string, d *commonv1.ErrorDetail) error {
	if d == nil {
		return nil
	}
	switch d.Code {
	case commonv1.ErrorCode_ERROR_CODE_DEPENDENCY_UNAVAILABLE:
		return fmt.Errorf("%w: %s: %s", model.ErrEngineUnavailable, service, d.Message)
	default:
		return model.Invalid("%s", d.Message)
	}
}

// Trailer returns the ErrorDetail in a call's trailing metadata, or nil.
func Trailer(md metadata.MD) *commonv1.ErrorDetail {
	vals := md.Get(ErrorDetailKey)
	if len(vals) == 0 {
		return nil
	}
	var d commonv1.ErrorDetail
	if proto.Unmarshal([]byte(vals[0]), &d) != nil {
		return nil
	}
	return &d
}

// Call runs a unary RPC and maps its error: it passes a trailer option so the ErrorDetail is read.
func Call[R any](service string, invoke func(trailer *metadata.MD) (R, error)) (R, error) {
	var tr metadata.MD
	out, err := invoke(&tr)
	if err != nil {
		var zero R
		return zero, FromStatus(service, err, Trailer(tr))
	}
	return out, nil
}

// ChecksJSON is the plan checks in the shape the HTTP engine always returned (room_id null when absent), so
// stored results and the web panel see no difference.
func ChecksJSON(cs []*commonv1.Check) json.RawMessage {
	type check struct {
		Rule     string   `json:"rule"`
		Severity string   `json:"severity"`
		OK       bool     `json:"ok"`
		Message  string   `json:"message"`
		RoomID   *string  `json:"room_id"`
		Category string   `json:"category"`
		IssueID  string   `json:"issue_id"`
		Elements []string `json:"elements"`
		Hint     string   `json:"hint"`
	}
	out := make([]check, 0, len(cs))
	for _, c := range cs {
		ck := check{Rule: c.Rule, Severity: c.Severity, OK: c.Ok, Message: c.Message, Category: c.Category,
			IssueID: c.IssueId, Elements: c.Elements, Hint: c.Hint}
		if ck.Elements == nil {
			ck.Elements = []string{}
		}
		if c.RoomId != "" {
			id := c.RoomId
			ck.RoomID = &id
		}
		out = append(out, ck)
	}
	b, _ := json.Marshal(out)
	return b
}
