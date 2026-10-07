package ai

import (
	"encoding/json"
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"io"
	"strings"
)

func clipDescriptor(mode string) llm.RequestComposition {
	c := llm.RequestComposition{Stage: "video-composition", Mode: mode, PromptVersion: "clip-" + mode + "-v1", SchemaVersion: "clip-" + mode + "-v1", Consumer: "private video attempt result; validated edit plan or reviewed storyline", Activation: "current admitted frozen video call policy; bounded validation correction only", SourceFiles: []string{"internal/clip/ai/service.go", "internal/clip/ai/response_correction.go"}}
	var schema []byte
	switch mode {
	case "observe":
		c.Stage = "video-observation"
		c.Composer = "clip/ai.Service.ObserveChunk/BuildObservePrompt"
		c.Parser = "clip/ai.parseChunk"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/prompts.go")
		schema = ChunkSchema()
	case "flow":
		c.Composer = "clip/ai.Service.Flow/BuildFlowPrompt"
		c.Parser = "clip/ai.parseFlowPlan/validatePlan"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/flow_prompt.go")
		schema = FlowSchema()
	case "flow-follow-storyline":
		c.Composer = "clip/ai.Service.Flow/BuildFlowPrompt"
		c.Parser = "clip/ai.parseFlowPlan/validatePlan"
		c.Activation += "; frozen reviewed storyline, no new storyline output"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/flow_prompt.go")
		schema = RevisionFlowSchema()
	case "flow-revision":
		c.Composer = "clip/ai.Service.Revise/buildFlowRevisionPrompt"
		c.Parser = "clip/ai.parseFlowPlan/validatePlan"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/revision_prompt.go")
		schema = RevisionFlowSchema()
	case "narration":
		c.Composer = "clip/ai.Service.Narrate/BuildNarrationPrompt"
		c.Parser = "clip/ai.parseNarration/validatePlan"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/narration_prompt.go")
		schema = NarrationSchema()
	case "narration-revision":
		c.Composer = "clip/ai.Service.Revise/buildNarrationRevisionPrompt"
		c.Parser = "clip/ai.parseNarration/validatePlan"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/revision_prompt.go")
		schema = NarrationSchema()
	case "storyline", "storyline-revision":
		c.Composer = "clip/ai.Service.Storyline/BuildStorylinePrompt"
		c.Parser = "clip/ai.parseStoryline"
		c.Consumer = "reviewed storyline only; no canonical edit plan"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/storyline_prompt.go")
		schema = StorylineSchema()
	case "spoken-script":
		c.Composer = "clip/ai.Service.SpokenScript/BuildSpokenScriptPrompt"
		c.Parser = "clip/ai.parseSpokenScript"
		c.Consumer = "private spoken draft preceding synthesis and footage flow"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/spoken_script.go")
		schema = spokenScriptSchema
	case "spoken-script-revision":
		c.Composer = "clip/ai.Service.reviseSpoken"
		c.Parser = "clip/ai.reviseSpoken spoken_lines validation"
		c.Consumer = "dedicated spoken script revision; unchanged visible captions and audio speed"
		c.SourceFiles = append(c.SourceFiles, "internal/clip/ai/spoken_revision.go")
		schema = spokenRevisionSchema
	}
	c.Output = llm.OutputContractInspection{Name: "clip-" + mode, Version: c.SchemaVersion, Schema: string(schema)}
	return c
}

// RequestCompositions assembles only explicitly synthetic material through the real
// prompt builders. It has no service, store, media opener, admission or provider.
func RequestCompositions() []llm.RequestComposition {
	cfg := DefaultConfig(clip.Environment{})
	document := clip.NoTemplateComposition()
	source := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "synthetic-source", Info: clip.MediaInfo{DurationMS: 15000, Width: 1080, Height: 1920}}, Filename: "synthetic.mp4"}
	in := clip.PlanningInput{Language: "en", Composition: &document, Ratio: "vertical", TargetDurationMS: 15000, Instruction: "Synthetic owner direction", Guidelines: clip.VideoGuidelines{Defaults: []string{"Synthetic stock video rule"}, Owner: []string{"Synthetic owner video rule"}}, Analyses: []clip.SourceAnalysis{{Source: source, Segments: []clip.Segment{{StartMS: 0, EndMS: 15000, Event: "Synthetic observed scene", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}}}}}
	limits := compositionLimits(cfg, in)
	story := clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "Synthetic reviewed storyline", ObservationIDs: []string{clip.ObservationID(source.ID, 0)}}}}
	plan := clip.EditPlan{Portable: &clip.PortablePlan{}, DurationMS: 15000, Ratio: "vertical", Cuts: []clip.EditCut{{ID: "synthetic-cut", SourceID: source.ID, StartMS: 0, EndMS: 15000, PlaybackRatePermille: 1000}}, Storyline: &story}
	revision := clip.RevisionInput{PlanningInput: in, Current: plan, Request: "Synthetic explicit revision", Target: clip.RevisionBoth}
	var out []llm.RequestComposition
	add := func(mode, system, user string) {
		c := clipComposition(mode, system, user, in.Guidelines, planningSourceRefs(in))
		out = append(out, *c)
		feedback := "\nThe previous candidate failed validation. Produce a fresh complete response correcting this validation feedback, while preserving the original contract and factual input:\n" + promptJSON(map[string]any{"check": "output_shape", "phase": "decode", "measurements": map[string]int{"retry": 1, "retry_limit": 3}})
		out = append(out, *correctedClipComposition(c, feedback))
	}
	system, user := BuildObservePrompt(clip.ChunkInput{Language: "en", Source: source, DurationMS: 15000})
	add("observe", system, user)
	system, user = BuildFlowPrompt(in, cfg.Render.FadeMS, limits)
	add("flow", system, user)
	following := in
	following.FollowStoryline = &story
	system, user = BuildFlowPrompt(following, cfg.Render.FadeMS, limits)
	add("flow-follow-storyline", system, user)
	system, user = buildFlowRevisionPrompt(revision, cfg.Render.FadeMS, limits)
	add("flow-revision", system, user)
	system, user = BuildNarrationPrompt(clip.NarrationInput{PlanningInput: in, Flow: plan}, limits)
	add("narration", system, user)
	system, user = buildNarrationRevisionPrompt(revision, plan, limits)
	add("narration-revision", system, user)
	system, user = BuildStorylinePrompt(clip.StorylineInput{PlanningInput: in}, limits)
	add("storyline", system, user)
	system, user = BuildStorylinePrompt(clip.StorylineInput{PlanningInput: in, Current: &story, Request: revision.Request}, limits)
	add("storyline-revision", system, user)
	system, user = BuildSpokenScriptPrompt(in, limits)
	add("spoken-script", system, user)
	revision.Current.Narration = &clip.NarrationPlan{Enabled: true, Segments: []clip.SpokenSegment{{ID: "synthetic-segment", Text: "Synthetic reviewed spoken text", StartMS: 0, EndMS: 1000}}}
	system, user = buildSpokenRevisionPrompt(revision, revision.Current, limits)
	add("spoken-script-revision", system, user)
	return out
}

func clipComposition(mode, system, user string, g clip.VideoGuidelines, refs []string) *llm.RequestComposition {
	c := clipDescriptor(mode)
	occurrences := map[string]int{}
	add := func(role llm.InspectionRole, author llm.FragmentAuthorship, material, text string) {
		id := fmt.Sprintf("%s-%d", material, occurrences[material])
		occurrences[material]++
		c.Fragments = append(c.Fragments, llm.RequestFragment{ID: id, Role: role, Authorship: author, MaterialRole: material, Text: text, SourceFiles: append([]string(nil), c.SourceFiles...)})
	}
	// Owner rules keep their actual System role, independent of authorship. Split
	// only the explicitly rendered owner block, never infer rule classes from text.
	rest := system
	ownerHeading := "\n" + videoOwnerGuidelinesLabel
	ownerBlock := ownerHeading
	for _, line := range g.Owner {
		ownerBlock += "\n- " + strings.ReplaceAll(line, "\n", "\n  ")
	}
	if at := strings.LastIndex(rest, ownerBlock); len(g.Owner) > 0 && at >= 0 {
		add(llm.InspectionRoleSystem, llm.FragmentAuthorshipCode, "video-contract-and-stock-rules", rest[:at+len(ownerHeading)])
		rest = rest[at+len(ownerHeading):]
		for _, line := range g.Owner {
			rendered := "\n- " + strings.ReplaceAll(line, "\n", "\n  ")
			if !strings.HasPrefix(rest, rendered) {
				break
			}
			add(llm.InspectionRoleSystem, llm.FragmentAuthorshipCode, "owner-rule-framing", rendered[:3])
			add(llm.InspectionRoleSystem, llm.FragmentAuthorshipAccount, "frozen-owner-video-rule", rendered[3:])
			rest = rest[len(rendered):]
		}
	}
	if rest != "" {
		add(llm.InspectionRoleSystem, llm.FragmentAuthorshipCode, "video-stage-contract", rest)
	}
	// Decode only the exact already assembled text. Field responsibility is declared
	// here, not guessed from its words; JSON punctuation remains code-owned.
	decoder := json.NewDecoder(strings.NewReader(user))
	last := int64(0)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil || token != json.Delim('{') {
			break
		}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				break
			}
			key, ok := keyToken.(string)
			if !ok {
				break
			}
			start := decoder.InputOffset()
			for start < int64(len(user)) && (user[start] == ':' || user[start] == ' ' || user[start] == '\n' || user[start] == '\t') {
				start++
			}
			var value json.RawMessage
			if err := decoder.Decode(&value); err != nil {
				break
			}
			end := decoder.InputOffset()
			add(llm.InspectionRoleUser, llm.FragmentAuthorshipCode, "input-envelope", user[last:start])
			author, material := clipInputRole(key)
			add(llm.InspectionRoleUser, author, material, user[start:end])
			last = end
		}
		if _, err := decoder.Token(); err != nil {
			break
		}
	}
	if last < int64(len(user)) {
		add(llm.InspectionRoleUser, llm.FragmentAuthorshipCode, "input-envelope", user[last:])
	}
	if len(refs) > 0 {
		c.Fragments = append(c.Fragments, llm.RequestFragment{ID: "ordered-media", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "source-media-identifiers", SourceRefs: append([]string(nil), refs...), Activation: "frozen source order; media transport body deliberately omitted"})
	}
	return &c
}
func clipInputRole(key string) (llm.FragmentAuthorship, string) {
	switch key {
	case "project_instruction", "request", "revision_request":
		return llm.FragmentAuthorshipAccount, "explicit-owner-direction"
	case "global_values", "item_groups", "item_hints":
		return llm.FragmentAuthorshipAccount, "frozen-composition-inputs"
	case "template_outline", "declared_captions", "intro_outro":
		return llm.FragmentAuthorshipAccount, "frozen-template-and-reviewed-slot-context"
	case "source_order", "source_name":
		return llm.FragmentAuthorshipAccount, "selected-source-order-and-label"
	case "current_plan", "current_storyline", "following_storyline", "storyline", "current_spoken_script", "measured_narration":
		return llm.FragmentAuthorshipAccount, "reviewed-document-unconfirmed-text-origin"
	case "analyses":
		return llm.FragmentAuthorshipCode, "prior-observation-results-not-owner-experience"
	default:
		return llm.FragmentAuthorshipCode, "resolved-product-constraints-and-timeline"
	}
}
func planningSourceRefs(in clip.PlanningInput) []string {
	var refs []string
	seen := map[string]bool{}
	for _, a := range in.Analyses {
		if a.Source.ID != "" && !seen[a.Source.ID] {
			refs = append(refs, a.Source.ID)
			seen[a.Source.ID] = true
		}
	}
	return refs
}
func correctedClipComposition(base *llm.RequestComposition, feedback string) *llm.RequestComposition {
	if base == nil {
		return nil
	}
	c := *base
	c.Mode += "/correction"
	c.Composer += "/completeValidated"
	c.Fragments = append([]llm.RequestFragment(nil), base.Fragments...)
	feedbackFragment := llm.RequestFragment{ID: "validation-feedback", Role: llm.InspectionRoleSystem, Authorship: llm.FragmentAuthorshipCode, MaterialRole: "allowlisted-validation-feedback", Text: feedback, SourceFiles: []string{"internal/clip/ai/response_correction.go"}, Activation: "bounded permitted correction; raw candidate output omitted"}
	index := 0
	for index < len(c.Fragments) && c.Fragments[index].Role == llm.InspectionRoleSystem {
		index++
	}
	c.Fragments = append(c.Fragments[:index], append([]llm.RequestFragment{feedbackFragment}, c.Fragments[index:]...)...)
	return &c
}
