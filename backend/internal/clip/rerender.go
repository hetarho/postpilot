package clip

import (
	"context"
)

type CaptionSizer interface {
	CaptionSize(context.Context, string, Caption) (float64, float64, error)
}
