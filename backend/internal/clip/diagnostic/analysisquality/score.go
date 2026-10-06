package analysisquality

import (
	"slices"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip"
	"golang.org/x/text/unicode/norm"
)

func normalize(s, rule string) string {
	if rule != "exact" {
		s = norm.NFC.String(s)
	}
	if rule == "nfc_spaces" {
		s = strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, s)
	}
	return s
}
func field(s clip.Segment, name string) (string, bool) {
	switch name {
	case "event":
		return s.Event, true
	case "action":
		return s.Action, true
	case "motion":
		return s.Motion, true
	case "speech":
		return s.Speech, true
	case "quality":
		return s.Quality, true
	case "scene":
		return s.Scene, true
	case "usability":
		return s.Usability, true
	case "certainty":
		return s.Certainty, true
	case "subjects":
		return strings.Join(s.Subjects, "\n"), true
	}
	return "", false
}

func score(r *Report) error {
	cs := map[string]Case{}
	ins := map[string]Input{}
	annotations := map[string]Assessment{}
	for _, c := range r.Corpus.Cases {
		cs[c.ID] = c
	}
	for _, in := range r.Corpus.Inputs {
		ins[in.ID] = in
	}
	for _, a := range r.Corpus.Assessments {
		annotations[key(a.InputID, a.Replicate)+"/"+a.LabelID] = a
	}
	var editWork int64
	for _, at := range r.Attempts {
		in := ins[at.InputID]
		m := Metric{InputID: in.ID, Replicate: at.Replicate, Families: map[string]Counts{}, LabelStatuses: map[string]string{}}
		for _, family := range kinds {
			m.Families[family] = Counts{}
		}
		for _, l := range cs[in.CaseID].Labels {
			if l.EndMS <= in.Copy.OffsetMS || l.StartMS >= in.Copy.OffsetMS+in.Copy.DurationMS {
				continue
			}
			counts := m.Families[l.Kind]
			if l.Known {
				counts.Known++
			} else {
				counts.TruthUnknown++
			}
			if l.Required {
				counts.Required++
			}
			status := "unreviewed"
			a, reviewed := annotations[key(in.ID, at.Replicate)+"/"+l.ID]
			var segment *clip.Segment
			observed := ""
			if reviewed {
				if a.Status != "unreviewed" && (a.ResponseSHA256 != at.ResponseSHA256 || a.TruthLabelDigest != digest(l)) {
					return ErrInput
				}
				status = a.Status
				if a.Segment >= 0 {
					if at.Parsed == nil || a.Segment >= len(at.Parsed.Segments) {
						return ErrInput
					}
					segment = &at.Parsed.Segments[a.Segment]
					value, ok := field(*segment, a.Field)
					runes := []rune(value)
					if !ok || a.EndRune > len(runes) || segment.EndMS <= l.StartMS || segment.StartMS >= l.EndMS {
						return ErrInput
					}
					observed = string(runes[a.StartRune:a.EndRune])
					if l.Kind == "speech" && (a.Field != "speech" || a.StartRune != 0 || a.EndRune != len(runes)) {
						return ErrInput
					}
					if l.Kind == "usability" && (a.Field != "usability" || a.StartRune != 0 || a.EndRune != len(runes)) {
						return ErrInput
					}
					if slices.Contains([]string{"correct", "incorrect", "invented"}, status) && observed == "" && !(l.Kind == "speech" && l.Expected == "") {
						return ErrInput
					}
				} else if slices.Contains([]string{"correct", "incorrect", "invented"}, status) {
					return ErrInput
				}
				if !l.Known && (status == "correct" || status == "incorrect") {
					return ErrInput
				}
				if l.Required && status == "not_applicable" {
					return ErrInput
				}
			}
			if l.Known && segment != nil && slices.Contains([]string{"text", "number", "speech", "usability"}, l.Kind) {
				rawExact := observed == l.Expected
				normalExact := normalize(observed, l.Normalization) == normalize(l.Expected, l.Normalization)
				if rawExact {
					counts.RawExact++
				}
				if normalExact {
					counts.NormalizedExact++
				}
				if status == "correct" && !normalExact {
					status = "incorrect"
				}
			}
			if l.Kind == "speech" && l.Known && l.Expected != "" {
				want, got := []rune(normalize(l.Expected, l.Normalization)), []rune(normalize(observed, l.Normalization))
				if status == "unreviewed" || status == "unknown" || status == "not_applicable" {
					counts.SpeechUnscoredReferenceRunes += len(want)
				} else {
					editWork += int64(len(want)) * int64(len(got))
					if editWork > 10_000_000 {
						return ErrInput
					}
					counts.SpeechEdits += editDistance(want, got)
					counts.SpeechReferenceRunes += len(want)
				}
			}
			// Silence checks inspect all intersecting parsed speech, so a reviewer
			// cannot hide hallucinated speech by selecting an empty span elsewhere.
			if l.Kind == "speech" && l.Known && l.Expected == "" && at.Parsed != nil {
				hallucination := false
				for _, s := range at.Parsed.Segments {
					if s.EndMS > l.StartMS && s.StartMS < l.EndMS && strings.TrimSpace(s.Speech) != "" {
						hallucination = true
					}
				}
				if hallucination {
					status = "invented"
					counts.SilenceHallucination++
				} else {
					counts.SilenceCorrect++
				}
			}
			if slices.Contains([]string{"event", "scene", "speech"}, l.Kind) {
				if segment == nil || !l.Known {
					m.UnknownBoundaries++
				} else {
					start, end := max(l.StartMS, in.Copy.OffsetMS), min(l.EndMS, in.Copy.OffsetMS+in.Copy.DurationMS)
					b := BoundaryError{LabelID: l.ID, StartSignedMS: segment.StartMS - start, EndSignedMS: segment.EndMS - end, ToleranceMS: l.ToleranceMS}
					b.StartAbsoluteMS = abs(b.StartSignedMS)
					b.EndAbsoluteMS = abs(b.EndSignedMS)
					b.OutsideTolerance = b.StartAbsoluteMS > l.ToleranceMS || b.EndAbsoluteMS > l.ToleranceMS
					m.Boundaries = append(m.Boundaries, b)
					if b.OutsideTolerance {
						r.Qualification.CriticalRegressions = append(r.Qualification.CriticalRegressions, key(in.ID, at.Replicate)+"/"+l.ID+"/time")
					}
				}
			}
			switch status {
			case "correct":
				counts.Correct++
			case "incorrect":
				counts.Incorrect++
			case "omitted":
				counts.Omitted++
			case "unknown":
				counts.Unknown++
			case "invented":
				counts.Invented++
			case "unreviewed":
				counts.Unreviewed++
			case "not_applicable":
				counts.NotApplicable++
			}
			m.Families[l.Kind] = counts
			m.LabelStatuses[l.ID] = status
			if l.Critical && (status == "invented" || l.Known && status != "correct") {
				m.CriticalFailures = append(m.CriticalFailures, l.ID)
				r.Qualification.CriticalRegressions = append(r.Qualification.CriticalRegressions, key(in.ID, at.Replicate)+"/"+l.ID)
			}
		}
		r.Metrics = append(r.Metrics, m)
	}
	metrics := map[string]Metric{}
	parsed := map[string]bool{}
	for _, at := range r.Attempts {
		parsed[key(at.InputID, at.Replicate)] = at.Status == "parsed"
	}
	for _, m := range r.Metrics {
		metrics[key(m.InputID, m.Replicate)] = m
	}
	for _, c := range r.Corpus.Cases {
		for index := 0; index <= (c.Source.Info.DurationMS-1)/60000; index++ {
			for rep := 0; rep < r.Corpus.Replicates; rep++ {
				for _, direction := range [][2]string{{"reference", "native"}, {"reference", "browser"}, {"native", "browser"}} {
					p := Pair{CaseID: c.ID, Index: index, Replicate: rep, From: direction[0], To: direction[1]}
					var from, to *Metric
					for _, in := range r.Corpus.Inputs {
						if in.CaseID == c.ID && in.Copy.Index == index {
							m, ok := metrics[key(in.ID, rep)]
							if ok && parsed[key(in.ID, rep)] {
								if in.Arm == p.From {
									from = &m
								}
								if in.Arm == p.To {
									to = &m
								}
							}
						}
					}
					if from == nil || to == nil {
						p.Missing = true
					} else {
						for _, l := range c.Labels {
							if from.LabelStatuses[l.ID] == "correct" && to.LabelStatuses[l.ID] != "correct" {
								p.Regressions = append(p.Regressions, l.ID)
							}
						}
						for _, b := range to.Boundaries {
							for _, previous := range from.Boundaries {
								if previous.LabelID == b.LabelID && !previous.OutsideTolerance && b.OutsideTolerance {
									p.Regressions = append(p.Regressions, b.LabelID+"/time")
								}
							}
						}
					}
					r.Pairs = append(r.Pairs, p)
					for _, id := range p.Regressions {
						r.Qualification.CriticalRegressions = append(r.Qualification.CriticalRegressions, c.ID+"/"+p.From+"/"+p.To+"/"+id)
					}
				}
			}
		}
	}
	for _, in := range r.Corpus.Inputs {
		v := Variance{InputID: in.ID, Planned: r.Corpus.Replicates, Status: "unmeasured", LabelDistributions: map[string]map[string]int{}}
		boundaryStarts, boundaryEnds := map[string][]int{}, map[string][]int{}
		for _, at := range r.Attempts {
			if at.InputID != in.ID {
				continue
			}
			if at.Status == "parsed" {
				v.Completed++
			} else if at.Status == "unrun" {
				v.Unrun++
			} else {
				v.Failures++
			}
			m := metrics[key(in.ID, at.Replicate)]
			for id, status := range m.LabelStatuses {
				if v.LabelDistributions[id] == nil {
					v.LabelDistributions[id] = map[string]int{}
				}
				v.LabelDistributions[id][status]++
			}
			for _, b := range m.Boundaries {
				boundaryStarts[b.LabelID] = append(boundaryStarts[b.LabelID], b.StartSignedMS)
				boundaryEnds[b.LabelID] = append(boundaryEnds[b.LabelID], b.EndSignedMS)
			}
		}
		if v.Planned > 1 {
			v.Status = "incomplete"
			if v.Completed == v.Planned {
				v.Status = "measured_offline"
			}
		}
		for id, statuses := range v.LabelDistributions {
			if len(statuses) > 1 {
				v.Disagreements = append(v.Disagreements, id)
			}
		}
		slices.Sort(v.Disagreements)
		for id, start := range boundaryStarts {
			if len(start) > 1 {
				v.MaxStartSpreadMS = max(v.MaxStartSpreadMS, slices.Max(start)-slices.Min(start))
				end := boundaryEnds[id]
				v.MaxEndSpreadMS = max(v.MaxEndSpreadMS, slices.Max(end)-slices.Min(end))
			}
		}
		r.Variance = append(r.Variance, v)
	}
	if r.Corpus.HumanReview != nil {
		h := r.Corpus.HumanReview
		if !text(h.Reviewer, 1, 128) || h.ReviewedAt.IsZero() || h.ReviewedAt.After(r.CreatedAt) || h.PlanDigest != r.PlanDigest || h.TruthDigest != r.TruthDigest || h.OutputsDigest != r.OutputsDigest {
			return ErrInput
		}
		if h.Complete {
			for _, m := range r.Metrics {
				for _, counts := range m.Families {
					if counts.Unreviewed != 0 {
						return ErrInput
					}
				}
			}
			for _, at := range r.Attempts {
				if at.Status != "parsed" {
					return ErrInput
				}
			}
			for _, p := range r.Pairs {
				if p.Missing {
					return ErrInput
				}
			}
		}
		// An attached offline review records arithmetic/evidence review only.
		// It cannot satisfy actual current-provider semantic review.
	}
	slices.Sort(r.Qualification.CriticalRegressions)
	r.Qualification.CriticalRegressions = slices.Compact(r.Qualification.CriticalRegressions)
	return nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func editDistance(a, b []rune) int {
	row := make([]int, len(b)+1)
	for i := range row {
		row[i] = i
	}
	for i, x := range a {
		previous := row[0]
		row[0] = i + 1
		for j, y := range b {
			old := row[j+1]
			sub := 0
			if x != y {
				sub = 1
			}
			row[j+1] = min(row[j+1]+1, row[j]+1, previous+sub)
			previous = old
		}
	}
	return row[len(b)]
}
