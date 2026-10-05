package storage

import (
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/postpilot/backend/internal/clip"
	"strings"
	"testing"
)

func TestClipSpeechStorageHasPrivateImmutableSeparateNamespace(t *testing.T) {
	key := clip.SpeechAudioPrefix + strings.Repeat("a", 32) + ".mp3"
	f := &clipS3{}
	b := &Bucket{ops: f, name: "private"}
	if e := b.PutClipSpeechAudio(t.Context(), key, []byte("audio")); e != nil {
		t.Fatal(e)
	}
	if aws.ToString(f.put.ContentType) != "audio/mpeg" || aws.ToString(f.put.IfNoneMatch) != "*" || aws.ToString(f.put.CacheControl) != "private, no-store" {
		t.Fatal(f.put)
	}
	for _, bad := range []string{"clip/speech/audio.mp3", "private/spoken/" + strings.Repeat("a", 32) + ".mp3", "../" + key} {
		if e := b.PutClipSpeechAudio(t.Context(), bad, []byte("audio")); !errors.Is(e, clip.ErrInvalid) {
			t.Fatal(bad, e)
		}
		if e := b.DeleteClipSpeechAudio(t.Context(), bad); !errors.Is(e, clip.ErrInvalid) {
			t.Fatal(bad, e)
		}
	}
	f.body = &trackedBody{Reader: strings.NewReader("audio")}
	f.size = 5
	if data, e := b.ReadClipSpeechAudio(t.Context(), key, 5); e != nil || string(data) != "audio" || !f.body.closed {
		t.Fatal(string(data), e)
	}
	if _, e := b.ReadClipSpeechAudio(t.Context(), key, 0); !errors.Is(e, clip.ErrInvalid) {
		t.Fatal(e)
	}
}
