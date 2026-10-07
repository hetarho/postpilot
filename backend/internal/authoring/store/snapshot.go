package store

import (
	"encoding/json"

	"github.com/postpilot/backend/internal/authoring"
)

type artifactSnapshot struct {
	TargetLength *string  `json:"target_length,omitempty"`
	TagCount     *string  `json:"tag_count,omitempty"`
	Scope        *string  `json:"scope,omitempty"`
	TemplateIDs  []string `json:"template_ids,omitempty"`
	Fields       []string `json:"fields,omitempty"`
	BuilderState string   `json:"builder_state,omitempty"`
	Revision     uint32   `json:"revision"`
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Body         string   `json:"body"`
	TitleArea    string   `json:"title_area"`
}
type turnSnapshot struct {
	ID      string `json:"id"`
	Request string `json:"request"`
	Reply   string `json:"reply"`
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
}
type savedSnapshot struct {
	Outcome string `json:"outcome,omitempty"`
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}
type publicationSnapshot struct {
	Key           string           `json:"key"`
	UserID        string           `json:"user_id"`
	SessionID     string           `json:"session_id"`
	Kind          string           `json:"kind"`
	Revision      uint32           `json:"revision"`
	Artifact      artifactSnapshot `json:"artifact"`
	TargetID      string           `json:"target_id"`
	TargetVersion string           `json:"target_version"`
	MakeDefault   bool             `json:"make_default"`
	WriteModel    string           `json:"write_model"`
}
type sessionSnapshot struct {
	ReferencePost   string               `json:"reference_post,omitempty"`
	Version         int                  `json:"version"`
	Candidates      []artifactSnapshot   `json:"candidates"`
	Selected        *artifactSnapshot    `json:"selected"`
	Turns           []turnSnapshot       `json:"turns"`
	Saved           *savedSnapshot       `json:"saved"`
	TargetVersion   string               `json:"target_version"`
	ActiveJobID     string               `json:"active_job_id"`
	ActiveRequestID string               `json:"active_request_id"`
	FailureReason   string               `json:"failure_reason"`
	PendingRequest  string               `json:"pending_request"`
	ForkVoice       bool                 `json:"fork_voice"`
	SourceContext   string               `json:"source_context"`
	Purpose         string               `json:"purpose"`
	WriteModel      string               `json:"write_model"`
	Publication     *publicationSnapshot `json:"publication"`
}

func artifactToSnapshot(a authoring.Artifact) artifactSnapshot {
	return artifactSnapshot{Revision: a.Revision, ID: a.ID, Name: a.Name, Description: a.Description, Body: a.Body, TitleArea: a.TitleArea, TargetLength: a.TargetLength, TagCount: a.TagCount, Scope: a.Scope, TemplateIDs: a.TemplateIDs, Fields: a.Fields, BuilderState: a.BuilderState}
}
func artifactFromSnapshot(a artifactSnapshot) authoring.Artifact {
	return authoring.Artifact{Revision: a.Revision, ID: a.ID, Name: a.Name, Description: a.Description, Body: a.Body, TitleArea: a.TitleArea, TargetLength: a.TargetLength, TagCount: a.TagCount, Scope: a.Scope, TemplateIDs: a.TemplateIDs, Fields: a.Fields, BuilderState: a.BuilderState}
}
func encodeSession(s authoring.Session) (string, error) {
	p := sessionSnapshot{Version: 1, TargetVersion: s.TargetVersion, ActiveJobID: s.ActiveJobID, ActiveRequestID: s.ActiveRequestID, FailureReason: s.FailureReason, PendingRequest: s.PendingRequest, ForkVoice: s.ForkVoice, SourceContext: s.SourceContext, ReferencePost: s.ReferencePost, Purpose: s.Purpose, WriteModel: s.WriteModel}
	for _, a := range s.Candidates {
		p.Candidates = append(p.Candidates, artifactToSnapshot(a))
	}
	if s.Selected != nil {
		a := artifactToSnapshot(*s.Selected)
		p.Selected = &a
	}
	for _, t := range s.Turns {
		p.Turns = append(p.Turns, turnSnapshot{t.ID, t.Request, t.Reply, t.JobID, t.Status})
	}
	if s.Saved != nil {
		p.Saved = &savedSnapshot{Kind: string(s.Saved.Kind), ID: s.Saved.ID, Name: s.Saved.Name, Outcome: s.Saved.Outcome}
	}
	if q := s.Publication; q != nil {
		p.Publication = &publicationSnapshot{Key: q.Key, UserID: q.UserID, SessionID: q.SessionID, Kind: string(q.Kind), Revision: q.Revision, Artifact: artifactToSnapshot(q.Artifact), TargetID: q.TargetID, TargetVersion: q.TargetVersion, MakeDefault: q.MakeDefault, WriteModel: q.WriteModel}
	}
	b, e := json.Marshal(p)
	return string(b), e
}
func decodeSession(raw string) (authoring.Session, error) {
	var p sessionSnapshot
	if e := json.Unmarshal([]byte(raw), &p); e != nil {
		return authoring.Session{}, e
	}
	if p.Version != 1 {
		return authoring.Session{}, authoring.ErrInvalid
	}
	s := authoring.Session{Candidates: []authoring.Artifact{}, Turns: []authoring.Turn{}, TargetVersion: p.TargetVersion, ActiveJobID: p.ActiveJobID, ActiveRequestID: p.ActiveRequestID, FailureReason: p.FailureReason, PendingRequest: p.PendingRequest, ForkVoice: p.ForkVoice, SourceContext: p.SourceContext, ReferencePost: p.ReferencePost, Purpose: p.Purpose, WriteModel: p.WriteModel}
	for _, a := range p.Candidates {
		s.Candidates = append(s.Candidates, artifactFromSnapshot(a))
	}
	if p.Selected != nil {
		a := artifactFromSnapshot(*p.Selected)
		s.Selected = &a
	}
	for _, t := range p.Turns {
		s.Turns = append(s.Turns, authoring.Turn{ID: t.ID, Request: t.Request, Reply: t.Reply, JobID: t.JobID, Status: t.Status})
	}
	if p.Saved != nil {
		s.Saved = &authoring.SavedRef{Kind: authoring.Kind(p.Saved.Kind), ID: p.Saved.ID, Name: p.Saved.Name, Outcome: p.Saved.Outcome}
	}
	if q := p.Publication; q != nil {
		s.Publication = &authoring.Publication{Key: q.Key, UserID: q.UserID, SessionID: q.SessionID, Kind: authoring.Kind(q.Kind), Revision: q.Revision, Artifact: artifactFromSnapshot(q.Artifact), TargetID: q.TargetID, TargetVersion: q.TargetVersion, MakeDefault: q.MakeDefault, WriteModel: q.WriteModel}
	}
	return s, nil
}
