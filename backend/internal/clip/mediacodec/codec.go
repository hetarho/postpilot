// Package mediacodec is the bounded, versioned persistence/transport codec shared
// by the media API and worker adapters. Domain records carry no wire tags.
package mediacodec

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

func encode(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil || len(b) > clip.MediaPayloadMaxBytes {
		return "", clip.ErrInvalid
	}
	return string(b), nil
}
func decode(raw string, value any) error {
	if len(raw) == 0 || len(raw) > clip.MediaPayloadMaxBytes {
		return clip.ErrInvalid
	}
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil {
		return clip.ErrInvalid
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return clip.ErrInvalid
	}
	return nil
}
func EncodeTask(task clip.MediaTask) (string, error) {
	if task.Version != clip.MediaContractVersion {
		return "", clip.ErrMediaIncompatible
	}
	return encode(task)
}
func DecodeTask(raw string) (clip.MediaTask, error) {
	var task clip.MediaTask
	if err := decode(raw, &task); err != nil {
		return task, err
	}
	if task.Version != clip.MediaContractVersion {
		return task, clip.ErrMediaIncompatible
	}
	return task, nil
}
func EncodeResult(result clip.MediaResult) (string, error) {
	if result.Version != clip.MediaContractVersion {
		return "", clip.ErrMediaIncompatible
	}
	return encode(result)
}
func DecodeResult(raw string) (clip.MediaResult, error) {
	var result clip.MediaResult
	if err := decode(raw, &result); err != nil {
		return result, err
	}
	if result.Version != clip.MediaContractVersion {
		return result, clip.ErrMediaIncompatible
	}
	return result, nil
}

func EncodeInfo(info clip.MediaInfo) (string, error) {
	raw, err := encode(info)
	if len(raw) > clip.MediaManifestMaxBytes {
		return "", clip.ErrInvalid
	}
	return raw, err
}
func DecodeInfo(raw string) (clip.MediaInfo, error) {
	var info clip.MediaInfo
	if len(raw) > clip.MediaManifestMaxBytes {
		return info, clip.ErrInvalid
	}
	err := decode(raw, &info)
	return info, err
}
