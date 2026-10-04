package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"

	"github.com/hajimehoshi/go-mp3"
)

// InspectSpeechAudio validates the selected fixed MP3 format before decoding.
// This frame pass detects truncation that streaming MP3 decoders treat as EOF,
// and rejects overlong audio before any PCM decode/allocation. PCM uses a fixed
// scratch buffer; neither decoded audio nor a duration-sized buffer is retained.
func InspectSpeechAudio(ctx context.Context, encoded []byte) (audio EncodedAudio, err error) {
	defer func() {
		if recover() != nil {
			audio = EncodedAudio{}
			err = fmt.Errorf("%w: malformed MP3", ErrBadOutput)
		}
	}()
	if len(encoded) == 0 || len(encoded) > SpeechMaxAudioBytes {
		return audio, fmt.Errorf("%w: audio size", ErrBadOutput)
	}
	frames, err := speechMP3Frames(ctx, encoded)
	if err != nil {
		return audio, err
	}
	samples := frames * 1152
	if samples <= 0 || samples > int64(SpeechMaxDuration.Seconds())*44100 {
		return audio, fmt.Errorf("%w: audio duration", ErrBadOutput)
	}
	decoder, err := mp3.NewDecoder(&speechAudioReader{ctx: ctx, reader: bytes.NewReader(encoded)})
	if err != nil {
		if err := ctx.Err(); err != nil {
			return audio, err
		}
		return audio, fmt.Errorf("%w: MP3 decode: %v", ErrBadOutput, err)
	}
	if decoder.SampleRate() != 44100 {
		return audio, fmt.Errorf("%w: audio sample rate", ErrBadOutput)
	}
	var decoded int64
	var scratch [32 << 10]byte
	for {
		if err := ctx.Err(); err != nil {
			return audio, err
		}
		n, err := decoder.Read(scratch[:])
		// The decoder may translate an interrupted frame read into EOF.
		if err := ctx.Err(); err != nil {
			return audio, err
		}
		decoded += int64(n)
		if decoded > samples*4 {
			return audio, fmt.Errorf("%w: decoded audio size", ErrBadOutput)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			if err := ctx.Err(); err != nil {
				return audio, err
			}
			return audio, fmt.Errorf("%w: MP3 decode: %v", ErrBadOutput, err)
		}
		if n == 0 {
			return audio, fmt.Errorf("%w: empty MP3 frame", ErrBadOutput)
		}
	}
	if decoded != samples*4 {
		return audio, fmt.Errorf("%w: incomplete MP3 frames", ErrBadOutput)
	}
	hash := sha256.Sum256(encoded)
	return EncodedAudio{Bytes: bytes.Clone(encoded), Format: SpeechOutputFormat, SampleRate: 44100, Channels: 2, Samples: samples, SHA256: hex.EncodeToString(hash[:])}, nil
}

type speechAudioReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *speechAudioReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func speechMP3Frames(ctx context.Context, data []byte) (int64, error) {
	bad := func() (int64, error) { return 0, fmt.Errorf("%w: invalid or truncated MP3 frames", ErrBadOutput) }
	if len(data) >= 10 && string(data[:3]) == "ID3" {
		for _, b := range data[6:10] {
			if b&0x80 != 0 {
				return bad()
			}
		}
		size := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])
		if data[5]&0x10 != 0 {
			size += 10
		}
		if size+10 > len(data) {
			return bad()
		}
		data = data[size+10:]
	}
	var frames int64
	for len(data) > 0 {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if len(data) == 128 && string(data[:3]) == "TAG" {
			break
		}
		if len(data) < 4 {
			return bad()
		}
		h := binary.BigEndian.Uint32(data[:4])
		// MPEG1 Layer III, 128kbps, 44100Hz, valid emphasis (selected output).
		if h&0xfffe0000 != 0xfffa0000 || (h>>12)&15 != 9 || (h>>10)&3 != 0 || h&3 == 2 {
			return bad()
		}
		size := 144*128000/44100 + int((h>>9)&1)
		if len(data) < size {
			return bad()
		}
		data = data[size:]
		frames++
		if frames*1152 > int64(SpeechMaxDuration.Seconds())*44100 {
			return bad()
		}
	}
	return frames, nil
}

// ValidateSpeechAlignment treats absent timing as coarse timing, and malformed
// timing as bad output. Precision is never inferred from supplier prose/duration.
func ValidateSpeechAlignment(timing []CharacterTiming, audio EncodedAudio) error {
	if len(timing) > 0 && (audio.SampleRate <= 0 || audio.Samples <= 0) {
		return fmt.Errorf("%w: missing audio clock", ErrBadOutput)
	}
	var previousStart, previousEnd float64
	for i, t := range timing {
		if t.Character == "" || math.IsNaN(t.StartSeconds) || math.IsNaN(t.EndSeconds) || math.IsInf(t.StartSeconds, 0) || math.IsInf(t.EndSeconds, 0) ||
			t.StartSeconds < 0 || t.EndSeconds < t.StartSeconds || t.EndSeconds > float64(audio.Samples)/float64(audio.SampleRate) ||
			i > 0 && (t.StartSeconds < previousStart || t.EndSeconds < previousEnd) {
			return fmt.Errorf("%w: invalid speech alignment", ErrBadOutput)
		}
		previousStart, previousEnd = t.StartSeconds, t.EndSeconds
	}
	return nil
}
