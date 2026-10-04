package store

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice/spoken"
	"github.com/postpilot/backend/internal/voice/spoken/store/sqlc"
)

type evidenceRecord struct {
	RequestID, ReportedUSD string
	Units                  []unitRecord
}
type unitRecord struct{ Unit, Quantity string }
type timingRecord struct {
	Character  string
	Start, End float64
}
type operationRecord struct {
	Version                                                int
	ProfileJSON                                            string
	DraftID, VoiceID                                       string
	ExpectedRevision                                       int64
	Name, Description, PreviewText, QualificationSessionID string
	CandidateHandle, VoiceHandle                           string
	Texts, SpeechInputs, AssetIDs                          [2]string
	Calling                                                [2]bool
	Evidence                                               []evidenceRecord
	FailureReason, ResultID                                string
}

func evidenceToRecord(e llm.SpeechEvidence) evidenceRecord {
	r := evidenceRecord{RequestID: e.RequestID, ReportedUSD: e.ReportedUSD}
	for _, u := range e.Units {
		r.Units = append(r.Units, unitRecord{string(u.Unit), u.Quantity})
	}
	return r
}
func evidenceFromRecord(r evidenceRecord) llm.SpeechEvidence {
	e := llm.SpeechEvidence{RequestID: r.RequestID, ReportedUSD: r.ReportedUSD}
	for _, u := range r.Units {
		e.Units = append(e.Units, llm.SpeechUnitEvidence{Unit: llm.SpeechUnit(u.Unit), Quantity: u.Quantity})
	}
	return e
}
func encodeOperation(o spoken.Operation) (string, error) {
	p, err := encodeProfile(o.Profile)
	if err != nil {
		return "", err
	}
	r := operationRecord{Version: 1, ProfileJSON: p, DraftID: o.DraftID, VoiceID: o.VoiceID, ExpectedRevision: o.ExpectedRevision, Name: o.Name, Description: o.Description, PreviewText: o.PreviewText, QualificationSessionID: o.QualificationSessionID, CandidateHandle: string(o.CandidateHandle), VoiceHandle: string(o.VoiceHandle), Texts: o.Texts, SpeechInputs: o.SpeechInputs, AssetIDs: o.AssetIDs, Calling: o.Calling, FailureReason: o.FailureReason, ResultID: o.ResultID}
	for _, e := range o.Evidence {
		r.Evidence = append(r.Evidence, evidenceToRecord(e))
	}
	b, err := json.Marshal(r)
	return string(b), err
}
func operationFromRow(row sqlc.SpokenVoiceOperation) (spoken.Operation, error) {
	var r operationRecord
	if err := json.Unmarshal([]byte(row.SnapshotJson), &r); err != nil {
		return spoken.Operation{}, err
	}
	if r.Version != 1 {
		return spoken.Operation{}, spoken.ErrInvalid
	}
	p, err := decodeProfile(r.ProfileJSON)
	if err != nil {
		return spoken.Operation{}, err
	}
	o := spoken.Operation{SampleAssetID: row.SampleAssetID.String, ID: row.ID, OwnerID: row.OwnerID, Kind: row.Kind, State: row.State, JobID: row.JobID, IdempotencyKey: row.IdempotencyKey, RequestDigest: row.RequestDigest, ScopeDigest: row.ScopeDigest, CandidateID: row.CandidateID, ReceivedHandle: llm.VoiceHandle(row.ReceivedHandle), DraftID: r.DraftID, VoiceID: r.VoiceID, ExpectedRevision: r.ExpectedRevision, Profile: p, Name: r.Name, Description: r.Description, PreviewText: r.PreviewText, QualificationSessionID: r.QualificationSessionID, CandidateHandle: llm.CandidateHandle(r.CandidateHandle), VoiceHandle: llm.VoiceHandle(r.VoiceHandle), Texts: r.Texts, SpeechInputs: r.SpeechInputs, AssetIDs: r.AssetIDs, Calling: r.Calling, FailureReason: r.FailureReason, ResultID: r.ResultID, CreatedAt: parse(row.CreatedAt), UpdatedAt: parse(row.UpdatedAt)}
	for _, e := range r.Evidence {
		o.Evidence = append(o.Evidence, evidenceFromRecord(e))
	}
	return o, nil
}
func encodeProbe(p spoken.ProbeAudio) (string, string, error) {
	e, err := json.Marshal(evidenceToRecord(p.Evidence))
	if err != nil {
		return "", "", err
	}
	ts := make([]timingRecord, 0, len(p.Timing))
	for _, t := range p.Timing {
		ts = append(ts, timingRecord{t.Character, t.StartSeconds, t.EndSeconds})
	}
	timing, err := json.Marshal(ts)
	return string(e), string(timing), err
}
func decodeProbe(r sqlc.SpokenProbeAudio) (spoken.ProbeAudio, error) {
	var e evidenceRecord
	var ts []timingRecord
	if err := json.Unmarshal([]byte(r.EvidenceJson), &e); err != nil {
		return spoken.ProbeAudio{}, err
	}
	if err := json.Unmarshal([]byte(r.TimingJson), &ts); err != nil {
		return spoken.ProbeAudio{}, err
	}
	p := spoken.ProbeAudio{OperationID: r.OriginOperationID, JobID: r.OriginJobID, OwnerID: r.OwnerID, VoiceID: r.VoiceID, InputDigest: r.InputDigest, AssetID: r.AssetID, Evidence: evidenceFromRecord(e)}
	for _, t := range ts {
		p.Timing = append(p.Timing, llm.CharacterTiming{Character: t.Character, StartSeconds: t.Start, EndSeconds: t.End})
	}
	return p, nil
}
