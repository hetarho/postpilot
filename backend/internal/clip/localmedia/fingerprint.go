package localmedia

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"github.com/postpilot/backend/internal/clip"
	"hash"
)

const fingerprintSliceBytes = 64 << 10

// Fingerprint implements the browser's v1 identity: metadata + first/last 64KiB.
// It deliberately is not a full-file SHA; artifacts use a separate full digest.
type Fingerprint struct {
	prefix      hash.Hash
	first, last []byte
}

func newFingerprint(m clip.SourceMetadata) *Fingerprint {
	h := sha256.New()
	h.Write([]byte{1})
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(m.Bytes))
	h.Write(n[:])
	binary.BigEndian.PutUint32(n[:4], uint32(len(m.ContentType)))
	h.Write(n[:4])
	h.Write([]byte(m.ContentType))
	binary.BigEndian.PutUint64(n[:], uint64(m.DurationMS))
	h.Write(n[:])
	return &Fingerprint{prefix: h}
}
func (f *Fingerprint) Write(b []byte) {
	f.first = append(f.first, b[:min(len(b), fingerprintSliceBytes-len(f.first))]...)
	if len(b) >= fingerprintSliceBytes {
		f.last = append(f.last[:0], b[len(b)-fingerprintSliceBytes:]...)
		return
	}
	keep := min(len(f.last), fingerprintSliceBytes-len(b))
	copy(f.last, f.last[len(f.last)-keep:])
	f.last = append(f.last[:keep], b...)
}
func (f *Fingerprint) Sum() string {
	// Called once, after EOF.
	f.prefix.Write(f.first)
	f.prefix.Write(f.last)
	return hex.EncodeToString(f.prefix.Sum(nil))
}
func SameIdentity(a, b clip.MediaInfo) bool {
	return a.DurationMS == b.DurationMS && a.Width == b.Width && a.Height == b.Height && a.HasAudio == b.HasAudio
}
