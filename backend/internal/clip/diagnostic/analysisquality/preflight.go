package analysisquality

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/llm"
)

var labelID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
var kinds = []string{"fact", "event", "scene", "text", "number", "speech", "quality", "usability"}
var statuses = []string{"correct", "incorrect", "omitted", "unknown", "invented", "unreviewed", "not_applicable"}
var arms = []string{"reference", "native", "browser"}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func digest(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return hash(data)
}
func strict(data []byte, v any, limit int) error {
	if len(data) == 0 || len(data) > limit {
		return ErrInput
	}
	if uniqueJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return ErrInput
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return ErrInput
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrInput
	}
	return nil
}

// Reject repeated object keys (including case aliases) and excessive nesting
// before typed decoding; standard encoding/json otherwise keeps the last key.
func uniqueJSON(d *json.Decoder, depth int) error {
	if depth > 64 {
		return ErrInput
	}
	token, e := d.Token()
	if e != nil {
		return ErrInput
	}
	delim, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	if delim == '{' {
		seen := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			name, ok := key.(string)
			name = strings.ToLower(name)
			if e != nil || !ok || seen[name] {
				return ErrInput
			}
			seen[name] = true
			if uniqueJSON(d, depth+1) != nil {
				return ErrInput
			}
		}
	} else if delim == '[' {
		for d.More() {
			if uniqueJSON(d, depth+1) != nil {
				return ErrInput
			}
		}
	} else {
		return ErrInput
	}
	end, e := d.Token()
	if e != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return ErrInput
	}
	return nil
}
func text(s string, minimum, maximum int) bool {
	return clip.BoundedText(s, minimum, maximum) && (minimum == 0 || strings.TrimSpace(s) != "")
}

// Files are flat private regular files in one canonical private root. This
// intentionally excludes URLs, symlinks, devices and path traversal entirely.
func readPrivate(root string, f File, limit int64) ([]byte, error) {
	file, e := openPrivate(root, f, limit)
	if e != nil {
		return nil, e
	}
	defer file.Close()
	data, e := io.ReadAll(io.LimitReader(file, limit+1))
	if e != nil || int64(len(data)) != f.Bytes || hash(data) != f.SHA256 {
		return nil, ErrInput
	}
	return data, nil
}

func checkPrivate(root string, f File, limit int64) error {
	file, e := openPrivate(root, f, limit)
	if e != nil {
		return e
	}
	defer file.Close()
	sum := sha256.New()
	size, e := io.Copy(sum, io.LimitReader(file, limit+1))
	if e != nil || size != f.Bytes || hex.EncodeToString(sum.Sum(nil)) != f.SHA256 {
		return ErrInput
	}
	return nil
}

func openPrivate(root string, f File, limit int64) (*os.File, error) {
	if f.Path == "" || filepath.Base(f.Path) != f.Path || f.Path == "." || f.Path == ".." || strings.ContainsAny(f.Path, "/\\\x00") || f.Bytes <= 0 || f.Bytes > limit || !clip.ValidSHA256(f.SHA256) {
		return nil, ErrInput
	}
	path := filepath.Join(root, f.Path)
	stat, e := os.Lstat(path)
	if e != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || stat.Size() != f.Bytes {
		return nil, ErrInput
	}
	file, e := os.Open(path)
	if e != nil {
		return nil, ErrInput
	}
	opened, e := file.Stat()
	if e != nil || !os.SameFile(stat, opened) {
		file.Close()
		return nil, ErrInput
	}
	return file, nil
}

func privateRoot(root string) bool {
	real, e := filepath.EvalSymlinks(root)
	stat, e2 := os.Stat(root)
	return e == nil && e2 == nil && filepath.IsAbs(root) && real == root && stat.IsDir() && stat.Mode().Perm()&0077 == 0
}

func Load(path string, now time.Time) (Corpus, string, error) {
	var c Corpus
	absolute, e := filepath.Abs(path)
	if e != nil {
		return c, "", ErrInput
	}
	root := filepath.Dir(absolute)
	if !privateRoot(root) {
		return c, "", ErrInput
	}
	file, e := os.Lstat(absolute)
	if e != nil || !file.Mode().IsRegular() || file.Mode().Perm()&0077 != 0 || file.Size() <= 0 || file.Size() > MaxDocumentBytes {
		return c, "", ErrInput
	}
	opened, e := os.Open(absolute)
	if e != nil {
		return c, "", ErrInput
	}
	defer opened.Close()
	actual, e := opened.Stat()
	if e != nil || !os.SameFile(file, actual) {
		return c, "", ErrInput
	}
	data, e := io.ReadAll(io.LimitReader(opened, MaxDocumentBytes+1))
	if e != nil || strict(data, &c, MaxDocumentBytes) != nil || Validate(c, now) != nil {
		return Corpus{}, "", ErrInput
	}
	return c, root, nil
}

func Validate(c Corpus, now time.Time) error {
	if c.Version != Version || c.Format != Format || !text(c.CorpusVersion, 1, 64) || !text(c.TruthVersion, 1, 64) || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(c.GitCommit) || !slices.Contains([]string{"synthetic_mock", "recorded_replay"}, c.Origin) || !slices.Contains([]string{"ko", "en"}, c.Language) || !c.Policy.Valid() || !c.Policy.Pricing.Valid() || c.Policy.Pricing.Delivery != llm.ExecutionInlineStatic || c.Policy.Stage != llm.StageNameObserve || c.Policy.CompletionTokens != ai.DefaultConfig(clip.Environment{}).ObserveCompletionTokens || c.Policy.ResponseRetries != 0 || c.Replicates < 1 || c.Replicates > MaxReplicates || len(c.Cases) < 1 || len(c.Cases) > MaxCases || len(c.Inputs) < 1 || len(c.Inputs) > MaxInputs || len(c.Replays) > len(c.Inputs)*c.Replicates || len(c.Assessments) > len(c.Inputs)*c.Replicates*MaxLabels {
		return ErrInput
	}
	if _, ok := c.Policy.QuoteMicrousd(); !ok {
		return ErrInput
	}
	cases, inputs := map[string]Case{}, map[string]Input{}
	sources := []clip.AnalysisSource{}
	for _, cs := range c.Cases {
		if !labelID.MatchString(cs.ID) || cases[cs.ID].ID != "" || !clip.ValidSHA256(cs.Source.Fingerprint) || cs.Source.Fingerprint != cs.Original.SHA256 || !labelID.MatchString(cs.Source.ID) || !text(cs.Source.Filename, 1, 255) || !text(cs.MeasurementProvenance, 1, 128) || !text(cs.Annotator, 1, 128) || cs.AnnotatedAt.IsZero() || cs.AnnotatedAt.After(now) || cs.GroundedOriginalSHA256 != cs.Original.SHA256 || len(cs.Labels) == 0 || len(cs.Labels) > MaxLabels || len(cs.Tags) > 32 || !slices.Contains([]string{"operator_owned", "explicit_license"}, cs.Rights.Basis) || !text(cs.Rights.Issuer, 1, 128) || !clip.ValidSHA256(cs.Rights.EvidenceSHA256) || !cs.Rights.ProviderProcessing || !cs.Rights.PrivateRetention || !cs.Rights.ExpiresAt.After(now) {
			return ErrInput
		}
		if cs.Source.Info.Width > 8192 || cs.Source.Info.Height > 8192 || cs.Source.Info.FrameRateNumerator <= 0 || cs.Source.Info.FrameRateNumerator > 1000000 || cs.Source.Info.FrameRateDenominator <= 0 || cs.Source.Info.FrameRateDenominator > 1000000 || cs.Source.Info.DecodedFrames < 1 || cs.Source.Info.DecodedFrames > 100000000 {
			return ErrInput
		}
		if cs.Source.Info.HasAudio && (cs.Source.Info.AudioRate < 8000 || cs.Source.Info.AudioRate > 192000 || cs.Source.Info.AudioChannels < 1 || cs.Source.Info.AudioChannels > 16) || !cs.Source.Info.HasAudio && (cs.Source.Info.AudioRate != 0 || cs.Source.Info.AudioChannels != 0) {
			return ErrInput
		}
		seen := map[string]bool{}
		for _, l := range cs.Labels {
			if !labelID.MatchString(l.ID) || seen[l.ID] || !slices.Contains(kinds, l.Kind) || l.StartMS < 0 || l.EndMS <= l.StartMS || l.EndMS > cs.Source.Info.DurationMS || l.ToleranceMS < 0 || l.ToleranceMS > 60000 || !text(l.Expected, 0, 2000) || !text(l.Evidence, 1, 512) || !slices.Contains([]string{"exact", "nfc", "nfc_spaces"}, l.Normalization) || l.Known && l.UnknownReason != "" || !l.Known && (!text(l.UnknownReason, 1, 512) || l.Expected != "") {
				return ErrInput
			}
			if l.Kind != "speech" && l.Known && !text(l.Expected, 1, 2000) {
				return ErrInput
			}
			seen[l.ID] = true
		}
		for _, tag := range cs.Tags {
			if !labelID.MatchString(tag) {
				return ErrInput
			}
		}
		cases[cs.ID] = cs
		sources = append(sources, cs.Source)
	}
	if clip.ValidateAnalysisSources(clip.DefaultAnalysisLimits(), sources) != nil {
		return ErrInput
	}
	seenSlots := map[string]bool{}
	for _, in := range c.Inputs {
		cs, ok := cases[in.CaseID]
		copy := in.Copy
		if !ok || !labelID.MatchString(in.ID) || inputs[in.ID].ID != "" || !slices.Contains(arms, in.Arm) || in.Profile != clip.BrowserAnalysisProfileVersion || !text(in.PreparationVersion, 1, 128) || !text(in.Encoder.Name, 1, 128) || !text(in.Encoder.Version, 1, 128) || !text(in.Encoder.BrowserDeviceBuild, 1, 512) || len(in.Encoder.Settings) > 32 || in.Encoder.TargetVideoBitrate < 0 || in.Encoder.TargetVideoBitrate > 100000000 || in.Encoder.ActualVideoBitrate < 0 || in.Encoder.ActualVideoBitrate > 100000000 || in.Encoder.ActualAudioBitrate < 0 || in.Encoder.ActualAudioBitrate > 10000000 || copy.SourceID != cs.Source.ID || copy.Fingerprint != cs.Source.Fingerprint || copy.Bytes != in.File.Bytes || copy.Digest != in.File.SHA256 || copy.HasAudio != cs.Source.Info.HasAudio {
			return ErrInput
		}
		for k, v := range in.Encoder.Settings {
			if !labelID.MatchString(k) || !text(v, 1, 256) {
				return ErrInput
			}
		}
		if (clip.AnalysisVerificationTask{Version: 1, ProfileVersion: in.Profile, ManifestDigest: strings.Repeat("a", 64), Copies: []clip.AnalysisCopy{copy}}).Validate() != nil {
			return ErrInput
		}
		ci := clip.ChunkInput{Source: cs.Source, Index: copy.Index, OffsetMS: copy.OffsetMS, DurationMS: copy.DurationMS}
		if clip.ValidateChunkInput(clip.DefaultAnalysisLimits(), ci) != nil {
			return ErrInput
		}
		w, h := clip.BrowserAnalysisGeometry(cs.Source.Info, 720)
		if copy.Width != w || copy.Height != h {
			return ErrInput
		}
		slot := in.CaseID + "/" + in.Arm + "/" + copy.Slot
		if seenSlots[slot] {
			return ErrInput
		}
		seenSlots[slot] = true
		inputs[in.ID] = in
	}
	seen := map[string]bool{}
	for _, r := range c.Replays {
		if inputs[r.InputID].ID == "" || r.Replicate < 0 || r.Replicate >= c.Replicates || seen[key(r.InputID, r.Replicate)] {
			return ErrInput
		}
		seen[key(r.InputID, r.Replicate)] = true
	}
	seen = map[string]bool{}
	for _, a := range c.Assessments {
		in := inputs[a.InputID]
		cs := cases[in.CaseID]
		valid := false
		for _, l := range cs.Labels {
			if l.ID == a.LabelID && l.EndMS > in.Copy.OffsetMS && l.StartMS < in.Copy.OffsetMS+in.Copy.DurationMS {
				valid = true
			}
		}
		id := key(a.InputID, a.Replicate) + "/" + a.LabelID
		if !valid || a.Replicate < 0 || a.Replicate >= c.Replicates || seen[id] || !slices.Contains(statuses, a.Status) || a.Segment < -1 || a.Segment >= 60 || a.StartRune < 0 || a.EndRune < a.StartRune || a.EndRune > 2000 || !text(a.Reason, 0, 512) || a.Status != "unreviewed" && (!clip.ValidSHA256(a.ResponseSHA256) || !clip.ValidSHA256(a.TruthLabelDigest) || !text(a.Reviewer, 1, 128) || a.ReviewedAt.IsZero() || a.ReviewedAt.After(now)) {
			return ErrInput
		}
		seen[id] = true
	}
	return nil
}

func planDigest(c Corpus) string {
	c.Replays = nil
	c.Assessments = nil
	c.HumanReview = nil
	return digest(c)
}

func promptFor(c Corpus, in Input, cs Case) Prompt {
	s, u := ai.BuildObservePrompt(clip.ChunkInput{Language: c.Language, Source: cs.Source, Index: in.Copy.Index, OffsetMS: in.Copy.OffsetMS, DurationMS: in.Copy.DurationMS})
	p := Prompt{SystemSHA256: hash([]byte(s)), UserSHA256: hash([]byte(u)), SchemaSHA256: hash(ai.ChunkSchema())}
	// A different fixture invalidates its own rows. Independent compatible
	// cases remain replayable; truth/review binding is checked separately.
	p.ReplayKey = digest(struct{ Input, Source, Policy, Contract, System, User, Schema string }{digest(in), digest(cs.Source), digest(c.Policy), clip.AnalysisContractVersion, p.SystemSHA256, p.UserSHA256, p.SchemaSHA256})
	return p
}

// ReplayKeys is an offline planning helper. It performs no model resolution.
func ReplayKeys(c Corpus, now time.Time) (map[string]Prompt, error) {
	if Validate(c, now) != nil {
		return nil, ErrInput
	}
	result := map[string]Prompt{}
	for _, in := range c.Inputs {
		for _, cs := range c.Cases {
			if cs.ID == in.CaseID {
				result[in.ID] = promptFor(c, in, cs)
			}
		}
	}
	return result, nil
}
