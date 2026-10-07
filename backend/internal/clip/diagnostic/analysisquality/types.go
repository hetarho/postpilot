// Package analysisquality runs private offline observation diagnostics. Replay
// results and human-supplied annotations never authorize production or live work.
package analysisquality

import (
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

const (
	Format           = "postpilot-analysis-quality"
	Version          = 1
	MaxDocumentBytes = 4 << 20
	MaxResponseBytes = 2 << 20
	MaxOriginalBytes = 128 << 20
	MaxCases         = 6
	MaxInputs        = 147
	MaxLabels        = 256
	MaxReplicates    = 4
)

var (
	ErrInput     = errors.New("analysis_quality_invalid_input")
	ErrLive      = errors.New("analysis_quality_live_admission_unavailable")
	ErrBudget    = errors.New("analysis_quality_budget_refused")
	ErrUncertain = errors.New("analysis_quality_usage_uncertain")
	ErrOutput    = errors.New("analysis_quality_private_output_failed")
)

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Rights struct {
	Basis              string    `json:"basis"` // operator_owned or explicit_license
	Issuer             string    `json:"issuer"`
	EvidenceSHA256     string    `json:"evidenceSha256"`
	ProviderProcessing bool      `json:"providerProcessing"`
	PrivateRetention   bool      `json:"privateRetention"`
	ExpiresAt          time.Time `json:"expiresAt"`
}

type Label struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"` // fact,event,scene,text,number,speech,quality,usability
	StartMS       int    `json:"startMs"`
	EndMS         int    `json:"endMs"`
	ToleranceMS   int    `json:"toleranceMs"`
	Required      bool   `json:"required"`
	Critical      bool   `json:"critical"`
	Known         bool   `json:"known"`
	Expected      string `json:"expected"`
	UnknownReason string `json:"unknownReason"`
	Normalization string `json:"normalization"` // exact, nfc, nfc_spaces
	Evidence      string `json:"evidence"`
}

type Case struct {
	ID                     string              `json:"id"`
	Source                 clip.AnalysisSource `json:"source"`
	Original               File                `json:"original"`
	MeasurementProvenance  string              `json:"measurementProvenance"`
	Rights                 Rights              `json:"rights"`
	Annotator              string              `json:"annotator"`
	AnnotatedAt            time.Time           `json:"annotatedAt"`
	GroundedOriginalSHA256 string              `json:"groundedOriginalSha256"`
	Tags                   []string            `json:"tags"`
	Labels                 []Label             `json:"labels"`
}

type Encoder struct {
	Name               string            `json:"name"`
	Version            string            `json:"version"`
	Settings           map[string]string `json:"settings"`
	TargetVideoBitrate int               `json:"targetVideoBitrate"`
	ActualVideoBitrate int               `json:"actualVideoBitrate"`
	ActualAudioBitrate int               `json:"actualAudioBitrate"`
	BrowserDeviceBuild string            `json:"browserDeviceBuild"`
}

type Input struct {
	ID                 string            `json:"id"`
	CaseID             string            `json:"caseId"`
	Arm                string            `json:"arm"` // reference,native,browser
	Profile            string            `json:"profile"`
	PreparationVersion string            `json:"preparationVersion"`
	Encoder            Encoder           `json:"encoder"`
	Copy               clip.AnalysisCopy `json:"copy"`
	File               File              `json:"file"`
}

type Replay struct {
	InputID   string `json:"inputId"`
	Replicate int    `json:"replicate"`
	File      File   `json:"file"`
}

// Record binds a raw response to the exact production prompt/input/policy key.
// Recorded usage is retained evidence, never spend in the current offline run.
type Record struct {
	Version  int          `json:"version"`
	Origin   string       `json:"origin"`
	Key      string       `json:"key"`
	Response llm.Response `json:"response"`
	Failure  string       `json:"failure"` // empty or provider_failure
}

type Assessment struct {
	InputID          string    `json:"inputId"`
	Replicate        int       `json:"replicate"`
	LabelID          string    `json:"labelId"`
	ResponseSHA256   string    `json:"responseSha256"`
	TruthLabelDigest string    `json:"truthLabelDigest"`
	Status           string    `json:"status"`
	Segment          int       `json:"segment"` // zero based; -1 if no response match
	Field            string    `json:"field"`
	StartRune        int       `json:"startRune"`
	EndRune          int       `json:"endRune"`
	Reviewer         string    `json:"reviewer"`
	ReviewedAt       time.Time `json:"reviewedAt"`
	Reason           string    `json:"reason"`
}

type HumanReview struct {
	Reviewer      string    `json:"reviewer"`
	ReviewedAt    time.Time `json:"reviewedAt"`
	PlanDigest    string    `json:"planDigest"`
	TruthDigest   string    `json:"truthDigest"`
	OutputsDigest string    `json:"outputsDigest"`
	Complete      bool      `json:"complete"`
}

type Corpus struct {
	Version       int            `json:"version"`
	Format        string         `json:"format"`
	CorpusVersion string         `json:"corpusVersion"`
	TruthVersion  string         `json:"truthVersion"`
	GitCommit     string         `json:"gitCommit"`
	Origin        string         `json:"origin"`
	Language      string         `json:"language"`
	Policy        llm.CallPolicy `json:"policy"`
	Replicates    int            `json:"replicates"`
	Cases         []Case         `json:"cases"`
	Inputs        []Input        `json:"inputs"`
	Replays       []Replay       `json:"replays"`
	Assessments   []Assessment   `json:"assessments"`
	HumanReview   *HumanReview   `json:"humanReview,omitempty"`
}

type Prompt struct {
	SystemSHA256 string `json:"systemSha256"`
	UserSHA256   string `json:"userSha256"`
	SchemaSHA256 string `json:"schemaSha256"`
	ReplayKey    string `json:"replayKey"`
}

type Attempt struct {
	InputID        string                  `json:"inputId"`
	Replicate      int                     `json:"replicate"`
	Origin         string                  `json:"origin"`
	Prompt         Prompt                  `json:"prompt"`
	ResponseSHA256 string                  `json:"responseSha256"`
	Status         string                  `json:"status"`
	FinishReason   string                  `json:"finishReason"`
	RecordedUsage  llm.Usage               `json:"recordedUsage"`
	Diagnostic     *clip.AttemptDiagnostic `json:"diagnostic,omitempty"`
	Parsed         *clip.ChunkAnalysis     `json:"parsed,omitempty"`
	ParsedSHA256   string                  `json:"parsedSha256"`
}

type Counts struct {
	Known                        int `json:"known"`
	Required                     int `json:"required"`
	Correct                      int `json:"correct"`
	Incorrect                    int `json:"incorrect"`
	Omitted                      int `json:"omitted"`
	Unknown                      int `json:"unknown"`
	TruthUnknown                 int `json:"truthUnknown"`
	Invented                     int `json:"invented"`
	Unreviewed                   int `json:"unreviewed"`
	NotApplicable                int `json:"notApplicable"`
	RawExact                     int `json:"rawExact"`
	NormalizedExact              int `json:"normalizedExact"`
	SpeechEdits                  int `json:"speechEdits"`
	SpeechReferenceRunes         int `json:"speechReferenceRunes"`
	SpeechUnscoredReferenceRunes int `json:"speechUnscoredReferenceRunes"`
	SilenceCorrect               int `json:"silenceCorrect"`
	SilenceHallucination         int `json:"silenceHallucination"`
}

type BoundaryError struct {
	LabelID          string `json:"labelId"`
	StartSignedMS    int    `json:"startSignedMs"`
	EndSignedMS      int    `json:"endSignedMs"`
	StartAbsoluteMS  int    `json:"startAbsoluteMs"`
	EndAbsoluteMS    int    `json:"endAbsoluteMs"`
	ToleranceMS      int    `json:"toleranceMs"`
	OutsideTolerance bool   `json:"outsideTolerance"`
}

type Metric struct {
	InputID           string            `json:"inputId"`
	Replicate         int               `json:"replicate"`
	Families          map[string]Counts `json:"families"`
	Boundaries        []BoundaryError   `json:"boundaries"`
	UnknownBoundaries int               `json:"unknownBoundaries"`
	CriticalFailures  []string          `json:"criticalFailures"`
	LabelStatuses     map[string]string `json:"labelStatuses"`
}

type Pair struct {
	CaseID      string   `json:"caseId"`
	Index       int      `json:"index"`
	Replicate   int      `json:"replicate"`
	From        string   `json:"from"`
	To          string   `json:"to"`
	Missing     bool     `json:"missing"`
	Regressions []string `json:"regressions"`
}

type Variance struct {
	InputID            string                    `json:"inputId"`
	Planned            int                       `json:"planned"`
	Completed          int                       `json:"completed"`
	Failures           int                       `json:"failures"`
	Unrun              int                       `json:"unrun"`
	Status             string                    `json:"status"`
	LabelDistributions map[string]map[string]int `json:"labelDistributions"`
	Disagreements      []string                  `json:"disagreements"`
	MaxStartSpreadMS   int                       `json:"maxStartSpreadMs"`
	MaxEndSpreadMS     int                       `json:"maxEndSpreadMs"`
}

type Qualification struct {
	SemanticEvidence    bool     `json:"semanticEvidence"`
	Qualified           bool     `json:"qualified"`
	ProfileEnabled      bool     `json:"profileEnabled"`
	MissingGates        []string `json:"missingGates"`
	CriticalRegressions []string `json:"criticalRegressions"`
}

type Report struct {
	Version           int                                      `json:"version"`
	Format            string                                   `json:"format"`
	Mode              string                                   `json:"mode"`
	Status            string                                   `json:"status"`
	CreatedAt         time.Time                                `json:"createdAt"`
	PlanDigest        string                                   `json:"planDigest"`
	TruthDigest       string                                   `json:"truthDigest"`
	OutputsDigest     string                                   `json:"outputsDigest"`
	AnalysisContract  string                                   `json:"analysisContract"`
	Corpus            Corpus                                   `json:"corpus"`
	Prompts           map[string]Prompt                        `json:"prompts"`
	Verifications     map[string]clip.AnalysisCopyVerification `json:"verifications"`
	Attempts          []Attempt                                `json:"attempts"`
	Metrics           []Metric                                 `json:"metrics"`
	Pairs             []Pair                                   `json:"pairs"`
	Variance          []Variance                               `json:"variance"`
	ProviderControls  map[string]string                        `json:"providerControls"`
	ProviderCallsSent int                                      `json:"providerCallsSent"`
	MeasuredLiveSpend *int64                                   `json:"measuredLiveSpend"`
	Qualification     Qualification                            `json:"qualification"`
	Limits            []string                                 `json:"limits"`
}

// Summary contains no arbitrary text from input/provider/reviewer records.
type Summary struct {
	Version           int      `json:"version"`
	Mode              string   `json:"mode"`
	Status            string   `json:"status"`
	Inputs            int      `json:"inputs"`
	Attempts          int      `json:"attempts"`
	ProviderCallsSent int      `json:"providerCallsSent"`
	Qualified         bool     `json:"qualified"`
	ProfileEnabled    bool     `json:"profileEnabled"`
	MissingGates      []string `json:"missingGates"`
}

func (r Report) Summary() Summary {
	return Summary{Version: Version, Mode: r.Mode, Status: r.Status, Inputs: len(r.Corpus.Inputs), Attempts: len(r.Attempts), MissingGates: r.Qualification.MissingGates}
}
