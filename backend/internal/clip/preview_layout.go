package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// PreviewLayoutKey names the layout every read of one browser render draws:
// the render, the project revision it was admitted at and the draft the export
// sent. A render's grounds are fixed once sampled (CLIP-192), and a revision
// fixes the plan and the design the draft is applied to, so the same key is the
// same laid-out composition however many caption-frame and asset requests ask
// for it.
type PreviewLayoutKey struct {
	Render   string
	Revision int
	Draft    string
}

type previewLayoutKey struct{}

// WithPreviewLayout marks a read as drawing the layout key names, so a renderer
// that keeps laid-out compositions may answer it from one it already made.
func WithPreviewLayout(ctx context.Context, key PreviewLayoutKey) context.Context {
	return context.WithValue(ctx, previewLayoutKey{}, key)
}

// PreviewLayoutFrom is the layout a read names; a read that names none is laid
// out afresh.
func PreviewLayoutFrom(ctx context.Context) (PreviewLayoutKey, bool) {
	key, ok := ctx.Value(previewLayoutKey{}).(PreviewLayoutKey)
	return key, ok && key.Render != "" && key.Revision > 0 && key.Draft != ""
}

// DraftDigest is SHA-256 over a correction draft's canonical encoding: the same
// draft always encodes to the same bytes, and any change to it changes them.
func DraftDigest(draft CorrectionPlan) (string, error) {
	raw, err := json.Marshal(draft)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
