package clip

import (
	"context"
	"time"
)

type BrowserUploadStore interface {
	BrowserRenderStore
	ReserveBrowserRenderUpload(context.Context, string, string, int64, time.Time) error
	CompleteBrowserRender(context.Context, string, string, time.Time) (string, error)
	CancelBrowserRender(context.Context, string, string, time.Time) (bool, error)
}
