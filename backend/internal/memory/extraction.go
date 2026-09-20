package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

// Extraction is explicit work (MEM-13): the user presses 기억으로 저장 on a finished post and
// a durable job reads it once. Nothing about it is automatic, and nothing it produces is
// stored — it PROPOSES (MEM-14, MEM-15).
//
// The candidates live on the job row's own payload and nowhere else. That is deliberate and
// it must stay that way: MEM-15 forbids a durable review queue, because an extraction is
// repeatable on demand and a pending table would preserve exactly what the user already
// declined. A job row is transient work state nothing re-surfaces — do not turn this into a
// table.

// ExtractionSource is the post as this context reads it: the canonical content flattened to
// prose by the caller, plus the memo. Frozen at enqueue, so an edit made while the provider
// runs cannot change what was extracted.
type ExtractionSource struct {
	PostSlug string
	Title    string
	Memo     string
	Body     string
}

// ExtractionJob is what the worker receives. Model is the account's analyze selection,
// frozen at enqueue like every other job's model.
type ExtractionJob struct {
	ID       string
	UserID   string
	PostSlug string
	Model    string
	Source   ExtractionSource
}

// Candidate is ONE proposed fact. It is not a memory and never becomes one on its own: the
// user checks the ones they want and each checked one goes through the ordinary Create,
// which is where every field rule lives (MEM-15).
type Candidate struct {
	Text string
	Kind Kind
	Tags []string
}

type candidateJSON struct {
	Text string   `json:"text"`
	Kind string   `json:"kind"`
	Tags []string `json:"tags,omitempty"`
}

type extractionPayload struct {
	PostSlug   string          `json:"post_slug"`
	Candidates []candidateJSON `json:"candidates"`
}

// ExtractionPrompt asks for atomic facts about the AUTHOR'S WORLD — not about the writing,
// and not a summary of the post. It is a code constant beside the other prompt constants
// (ARCHITECTURE §4), and the five kinds are named in it because the model has to choose one
// of exactly those (MEM-5).
const ExtractionPrompt = `글과 메모에서 글쓴이의 세계에 관한 사실만 뽑아내세요.
하나의 사실은 한 문장이고, 다른 글에서도 그대로 참일 내용이어야 합니다.
이 글의 줄거리 요약, 글에 대한 평가, 문체에 대한 의견은 사실이 아닙니다. 뽑지 마세요.
확신할 수 없는 내용은 뽑지 마세요. 뽑을 것이 없으면 빈 목록을 반환하세요.
kind는 preference(취향), persona(글쓴이 설정), place(장소), person(인물), history(이력) 중 하나입니다.
tags는 이 사실이 다시 쓰일 만한 글을 찾아낼 짧은 검색어입니다. 문장이 아니라 단어로 쓰세요.
출력은 설명이나 마크다운 없이 {"candidates":[{"text":"...","kind":"...","tags":[]}]} 형태의 JSON 객체 하나여야 합니다.`

// EncodeExtractionSource freezes the post onto the job row at enqueue, and
// DecodeExtractionSource is what the worker reads back. The source rides the job because an
// edit made while the job waits must not change what was extracted — the same freeze the
// generation payload makes for its template brief and its guideline texts (GEN-30).
func EncodeExtractionSource(source ExtractionSource) ([]byte, error) {
	raw, err := json.Marshal(sourceJSON{
		PostSlug: source.PostSlug, Title: source.Title, Memo: source.Memo, Body: source.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("encode extraction source: %w", err)
	}
	return raw, nil
}

func DecodeExtractionSource(raw []byte) (ExtractionSource, error) {
	var wire sourceJSON
	if err := json.Unmarshal(raw, &wire); err != nil {
		return ExtractionSource{}, fmt.Errorf("decode extraction source: %w", err)
	}
	return ExtractionSource{PostSlug: wire.PostSlug, Title: wire.Title, Memo: wire.Memo, Body: wire.Body}, nil
}

type sourceJSON struct {
	PostSlug string `json:"post_slug"`
	Title    string `json:"title,omitempty"`
	Memo     string `json:"memo,omitempty"`
	Body     string `json:"body,omitempty"`
}

// ExtractionInProgressError names the extraction already running on this post, so the
// surface can attach to it rather than starting a second one the guard would refuse.
type ExtractionInProgressError struct{ ActiveID string }

func (e *ExtractionInProgressError) Error() string {
	return fmt.Sprintf("memory extraction %s is already in progress", e.ActiveID)
}

// Extract runs the one provider call and stores the candidates on the job row. It writes
// nothing to `memories`, `memory_tags` or `memory_sources` — this whole path is read-only
// with respect to the memory tables, and it touches no post row either (MEM-14).
func (s *Service) Extract(ctx context.Context, found ExtractionJob, progress func(stage string, done, total int)) error {
	if s.models == nil || s.extractions == nil {
		return fmt.Errorf("memory: extraction is not wired")
	}
	ref, err := parseModelRef(found.Model)
	if err != nil {
		return err
	}
	progress("extract", 0, 1)
	request := llm.Request{
		System:   ExtractionPrompt,
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(extractionInput(found.Source))}}},
		Stage:    llm.StageNameAnalyze,
	}
	// Attach-or-fall-back, exactly as the voice analysis does (VOICE-27): a model that does
	// not declare structured output still answers the prompt, which already states the shape.
	if info, ok := s.models.Resolve(ref); ok && info.StructuredOutput {
		request.JSONSchema = MemoryCandidatesSchema()
	}
	response, err := s.models.Complete(ctx, ref, request)
	if err != nil {
		return err
	}
	candidates, err := s.parseCandidates(response.Text)
	if err != nil {
		return err
	}
	payload, err := encodeExtraction(found.PostSlug, candidates)
	if err != nil {
		return err
	}
	if err := s.extractions.SaveCandidates(ctx, found.ID, payload); err != nil {
		return fmt.Errorf("기억 후보를 저장하지 못했어요: %w", err)
	}
	progress("extract", 1, 1)
	return nil
}

// Extraction reads one finished job's candidates for their owner. Ownership is the port's
// to enforce — a job of another account reads as missing there — and an unparsable payload
// is an error rather than an empty list, because "no candidates" is a real answer the user
// acts on and must not be indistinguishable from a broken read.
func (s *Service) Extraction(ctx context.Context, userID, jobID string) (string, []Candidate, error) {
	if s.extractions == nil {
		return "", nil, fmt.Errorf("memory: extraction is not wired")
	}
	raw, err := s.extractions.Candidates(ctx, userID, jobID)
	if err != nil {
		return "", nil, err
	}
	var payload extractionPayload
	if len(raw) == 0 {
		return "", nil, ErrExtractionNotReady
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", nil, ErrExtractionNotReady
	}
	out := make([]Candidate, 0, len(payload.Candidates))
	for _, c := range payload.Candidates {
		kind, err := ParseKind(c.Kind)
		if err != nil {
			continue
		}
		out = append(out, Candidate{Text: c.Text, Kind: kind, Tags: append([]string(nil), c.Tags...)})
	}
	return payload.PostSlug, out, nil
}

// extractionInput is what the model reads: the post's own title, memo and prose. No
// observations and no photos — a fact about the author's world is something they wrote or
// told us, not something a model inferred from a picture.
func extractionInput(source ExtractionSource) string {
	var out strings.Builder
	out.WriteString("제목: " + source.Title)
	out.WriteString("\n메모: " + source.Memo)
	out.WriteString("\n본문:\n" + source.Body)
	return out.String()
}

// parseCandidates applies the SAME bounds a create enforces, and drops whatever cannot pass
// them rather than proposing something the user could not save: a candidate whose text is
// too long, whose kind is not one of the five, or whose tags overflow would be a checkbox
// that answers with a refusal.
func (s *Service) parseCandidates(text string) ([]Candidate, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("기억 추출 결과가 비어 있어요")
	}
	var answer struct {
		Candidates []candidateJSON `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(trimmed), &answer); err != nil {
		return nil, fmt.Errorf("기억 추출 결과가 올바른 JSON이 아니에요: %w", err)
	}
	out := make([]Candidate, 0, len(answer.Candidates))
	seen := make(map[string]struct{}, len(answer.Candidates))
	for _, raw := range answer.Candidates {
		candidate, ok := s.validCandidate(raw)
		if !ok {
			continue
		}
		// The same exact-after-trim rule the store applies (MEM-9): two identical proposals
		// are one checkbox, not two.
		if _, duplicate := seen[candidate.Text]; duplicate {
			continue
		}
		seen[candidate.Text] = struct{}{}
		out = append(out, candidate)
	}
	return out, nil
}

func (s *Service) validCandidate(raw candidateJSON) (Candidate, bool) {
	text := strings.TrimSpace(raw.Text)
	if text == "" || utf8.RuneCountInString(text) > s.limits.TextMaxChars {
		return Candidate{}, false
	}
	kind, err := ParseKind(strings.TrimSpace(raw.Kind))
	if err != nil {
		return Candidate{}, false
	}
	tags, err := s.validTags(raw.Tags)
	if err != nil {
		// A proposal with an unusable tag keeps its fact and loses the tags, rather than
		// disappearing: the user can tag it themselves, and the fact is the valuable half.
		tags = nil
	}
	if len(tags) > s.limits.TagsMax {
		tags = tags[:s.limits.TagsMax]
	}
	return Candidate{Text: text, Kind: kind, Tags: tags}, true
}

// parseModelRef reads the frozen "provider/model" the job carries. The same two-part shape
// every context's job row uses; a malformed one is a refusal rather than a call.
func parseModelRef(value string) (llm.ModelRef, error) {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return llm.ModelRef{}, fmt.Errorf("기억 추출 모델 정보가 올바르지 않아요")
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}, nil
}

func encodeExtraction(postSlug string, candidates []Candidate) ([]byte, error) {
	wire := extractionPayload{PostSlug: postSlug, Candidates: make([]candidateJSON, 0, len(candidates))}
	for _, c := range candidates {
		wire.Candidates = append(wire.Candidates, candidateJSON{Text: c.Text, Kind: string(c.Kind), Tags: c.Tags})
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode memory candidates: %w", err)
	}
	return raw, nil
}
