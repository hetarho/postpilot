package clip

import "errors"

var (
	ErrRenderOverloaded  = errors.New("server render capacity is full")
	ErrRenderAccountBusy = errors.New("account already has a server render")
)

// RenderCapacity bounds durable admission and native execution independently
// from analysis verification and browser work.
type RenderCapacity struct{ Active, Waiting, PerAccount int }

func (l RenderCapacity) Validate() error {
	if l.Active < 1 || l.Active > ServerRenderActiveMax || l.Waiting < 0 || l.Waiting > ServerRenderWaitingMax || l.PerAccount < 1 || l.PerAccount > l.Active+l.Waiting {
		return errors.New("invalid CLIP_SERVER_RENDER_ACTIVE/WAITING/PER_ACCOUNT limits")
	}
	return nil
}
