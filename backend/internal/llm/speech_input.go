package llm

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"strconv"
	"unicode/utf8"
)

// SpeechInput is the validated, exact request identity used at the paid-call
// boundary. Private handles, punctuation and settings all participate; text is
// never normalized after approval. InputCharacters counts the billed text, not
// returned candidates. A design request is one operation for three auditions.
type SpeechInput struct {
	Ref                 ModelRef
	Operation, Digest   string
	InputCharacters     int
	AuxiliaryCharacters int
	ParametersDigest    string
}

func speechDigest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		writeSpeechPart(h, p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeSpeechPart(h hash.Hash, p string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(p)))
	h.Write(size[:])
	h.Write([]byte(p))
}

func (r VoiceDesignRequest) Input() (SpeechInput, error) {
	if err := r.Validate(); err != nil {
		return SpeechInput{}, err
	}
	return SpeechInput{Ref: r.Model, Operation: "voice_design", Digest: speechDigest("speech-input-v1", "voice_design", r.Model.String(), r.Description, r.PreviewText), InputCharacters: utf8.RuneCountInString(r.PreviewText), AuxiliaryCharacters: utf8.RuneCountInString(r.Description)}, nil
}

func (r VoiceConfirmationRequest) Input() (SpeechInput, error) {
	if err := r.Validate(); err != nil {
		return SpeechInput{}, err
	}
	return SpeechInput{Ref: r.DesignModel, Operation: "voice_confirm", Digest: speechDigest("speech-input-v1", "voice_confirm", r.DesignModel.String(), string(r.Candidate), r.Name, r.Description), AuxiliaryCharacters: utf8.RuneCountInString(r.Description)}, nil
}

func (r SpeechRequest) Input() (SpeechInput, error) {
	if err := r.Validate(); err != nil {
		return SpeechInput{}, err
	}
	s := r.Settings
	return SpeechInput{Ref: r.Model, Operation: "speech", Digest: speechDigest("speech-input-v1", "speech", r.Model.String(), string(r.Voice), r.Text, s.Digest()), InputCharacters: utf8.RuneCountInString(r.Text), ParametersDigest: s.Digest()}, nil
}

func (s SpeechSettings) Digest() string {
	return speechDigest("speech-settings-v1", strconv.FormatFloat(s.Stability, 'g', -1, 64), strconv.FormatFloat(s.SimilarityBoost, 'g', -1, 64), strconv.FormatFloat(s.Style, 'g', -1, 64), strconv.FormatBool(s.SpeakerBoost), strconv.FormatFloat(s.Speed, 'g', -1, 64))
}
