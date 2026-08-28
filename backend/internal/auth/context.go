package auth

import "context"

// userKeyType is unexported so no other package can write this context value — the
// acting user is set by the interceptor and only by the interceptor.
type userKeyType struct{}

var userKey userKeyType

// WithUser returns a context carrying the authenticated user id.
func WithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userKey, userID)
}

// UserFromContext returns the authenticated user id placed by the interceptor.
//
// Every authenticated handler takes the acting user from here and never from a request
// payload — a user id in a message is a claim by the caller, not a fact.
func UserFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userKey).(string)
	return id, ok && id != ""
}
