package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/postpilot/backend/internal/voice/spoken"
)

func TestSpokenStorageIsPrivateImmutableAndBounded(t *testing.T) {
	key := spoken.AudioPrefix + strings.Repeat("a", 32) + ".mp3"
	f := &clipS3{}
	b := &Bucket{ops: f, name: "private"}
	if err := b.PutSpokenAudio(t.Context(), key, []byte("audio")); err != nil {
		t.Fatal(err)
	}
	if aws.ToString(f.put.ContentType) != "audio/mpeg" || aws.ToString(f.put.IfNoneMatch) != "*" || aws.ToString(f.put.CacheControl) != "private, no-store" || aws.ToString(f.put.Key) != key {
		t.Fatal(f.put)
	}
	for _, bad := range []string{"photos/shared.png", "../private/spoken/secret.mp3", spoken.AudioPrefix + "foreign.mp3"} {
		if err := b.PutSpokenAudio(t.Context(), bad, []byte("data")); !errors.Is(err, spoken.ErrInvalid) {
			t.Fatal(err)
		}
		if err := b.DeleteSpokenAudio(t.Context(), bad); !errors.Is(err, spoken.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		body     string
		declared int64
		bad      bool
	}{{"1234", 4, false}, {"12345", 5, true}, {"123", 4, true}, {"12345", 4, true}} {
		f.body = &trackedBody{Reader: strings.NewReader(tc.body)}
		f.size = tc.declared
		data, err := b.ReadSpokenAudio(context.Background(), key, 4)
		if (err != nil) != tc.bad || !f.body.closed || len(data) > 4 {
			t.Fatal(tc, data, err, f.body.closed)
		}
	}
}
