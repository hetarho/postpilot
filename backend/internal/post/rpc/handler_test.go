package rpc

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/post"
)

// Every domain error the service can raise must have a code. An unmapped one becomes
// Internal, which tells the client to retry something that will never succeed.
func TestToConnectErrorMapsEveryDomainError(t *testing.T) {
	cases := map[error]connect.Code{
		post.ErrNotFound:          connect.CodeNotFound,
		post.ErrForbidden:         connect.CodePermissionDenied,
		post.ErrDuplicateFilename: connect.CodeAlreadyExists,
		post.ErrObjectMissing:     connect.CodeFailedPrecondition,
		post.ErrInvalidImage:      connect.CodeInvalidArgument,
		post.ErrPostBusy:          connect.CodeFailedPrecondition,
	}

	for domainErr, want := range cases {
		t.Run(domainErr.Error(), func(t *testing.T) {
			// Wrapped, as the service actually returns them.
			got := toConnectError("op", errors.Join(errors.New("context"), domainErr))
			if connect.CodeOf(got) != want {
				t.Errorf("code = %v, want %v", connect.CodeOf(got), want)
			}
		})
	}
}

// An unexpected failure must not put a SQL string or a bucket name on the wire.
func TestToConnectErrorHidesUnexpectedDetail(t *testing.T) {
	got := toConnectError("save draft", errors.New("no such table: posts (file /data/postpilot.db)"))

	if connect.CodeOf(got) != connect.CodeInternal {
		t.Errorf("code = %v, want internal", connect.CodeOf(got))
	}
	if got.Error() != "internal: save draft failed" {
		t.Errorf("message = %q, want it to carry no detail", got.Error())
	}
}
