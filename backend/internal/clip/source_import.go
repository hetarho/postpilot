package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// SourceImportStore is limited to a new project's initial, idle upload. Confirmed
// originals keep their identity, bytes and retention when pending siblings fail.
type SourceImportStore interface {
	ImportSourceBatch(context.Context, string, string, string, time.Time) (SourceBatch, error)
	CompleteSourceImport(context.Context, string, string, string, string, time.Time) (*SourceBatch, error)
}

// The same request has the same ready-batch receipt, including after a lost ack.
// Ownership and the selected batch are still checked by the store.
func CompletedSourceImportID(user, project, batch string) string {
	digest := sha256.Sum256([]byte("clip-source-import-v1\x00" + user + "\x00" + project + "\x00" + batch))
	return hex.EncodeToString(digest[:16])
}
