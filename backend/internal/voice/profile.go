package voice

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func (s *Service) ListVersions(ctx context.Context, userID, voiceID string) ([]ProfileVersion, error) {
	if _, err := s.ownedVoice(ctx, userID, voiceID); err != nil {
		return nil, err
	}
	return s.versions.ListProfileVersions(ctx, userID, voiceID)
}

func (s *Service) UpdateOverride(ctx context.Context, userID, voiceID string, layer RuleLayer, field string, value *string) (Profile, error) {
	if !validLayer(layer) || strings.TrimSpace(field) == "" {
		return Profile{}, fmt.Errorf("invalid voice override")
	}
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Profile{}, err
	}
	profile, err := s.Get(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, err
	}
	storedValue := value
	if value == nil {
		// Clearing reverts to the last measured/analyzed snapshot by replaying remaining overrides.
		versions, loadErr := s.versions.ListProfileVersions(ctx, userID, voiceID)
		if loadErr != nil {
			return Profile{}, loadErr
		}
		for _, version := range versions {
			if version.Origin == "analysis" {
				profile.Structured = version.Profile
				break
			}
		}
		// An analysis replays the overrides into the snapshot it publishes, so they are
		// stripped here before the remaining ones are replayed below: otherwise the one being
		// cleared stays baked in and 직접 설정 해제 does nothing.
		if err = stripOverrides(&profile.Structured); err != nil {
			return Profile{}, err
		}
	} else {
		trimmed := strings.TrimSpace(*value)
		if trimmed == "" {
			return Profile{}, fmt.Errorf("override value cannot be empty")
		}
		if err = applyOverride(&profile.Structured, layer, field, trimmed); err != nil {
			return Profile{}, err
		}
		storedValue = &trimmed
	}
	overrides, err := s.overrides.ListManualOverrides(ctx, userID, voiceID)
	if err != nil {
		return Profile{}, err
	}
	for _, override := range overrides {
		if value == nil && override.Layer == layer && override.Field == field {
			continue
		}
		if err = applyOverride(&profile.Structured, override.Layer, override.Field, override.Value); err != nil {
			return Profile{}, err
		}
	}
	now := s.now()
	if err = s.overrides.ApplyOverrideAndPublish(ctx, ManualOverride{UserID: userID, VoiceID: voiceID, Layer: layer, Field: field, UpdatedAt: now}, storedValue, profile.Structured, now); err != nil {
		return Profile{}, err
	}
	return s.Get(ctx, userID, voiceID)
}

func (s *Service) RestoreVersion(ctx context.Context, userID, voiceID string, version int64) (Profile, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Profile{}, err
	}
	found, err := s.versions.GetProfileVersion(ctx, userID, voiceID, version)
	if err != nil {
		return Profile{}, err
	}
	if _, err = s.versions.PublishProfileVersion(ctx, userID, voiceID, found.Profile, "restore", version, s.now()); err != nil {
		return Profile{}, err
	}
	return s.Get(ctx, userID, voiceID)
}

func validLayer(layer RuleLayer) bool {
	return layer == LayerLexical || layer == LayerEndings || layer == LayerSyntax || layer == LayerStructure || layer == LayerAxes
}
func manual(value string) VoiceValue { return VoiceValue{Value: value, Source: SourceManual} }

// overrideKey names one overridable field: the key its pre-override value is kept under.
func overrideKey(layer RuleLayer, field string) string { return string(layer) + "." + field }

// voiceField is the text field an override of (layer, field) replaces.
func voiceField(p *StructuredProfile, layer RuleLayer, field string) (*VoiceValue, error) {
	switch layer {
	case LayerLexical:
		if field == "description" {
			return &p.Lexical.Description, nil
		}
		return nil, fmt.Errorf("unsupported lexical field")
	case LayerEndings:
		if field == "base_register" {
			return &p.Endings.BaseRegister, nil
		}
		return nil, fmt.Errorf("unsupported endings field")
	case LayerSyntax:
		switch field {
		case "sentence_length":
			return &p.Syntax.SentenceLength, nil
		case "connective_style":
			return &p.Syntax.ConnectiveStyle, nil
		case "nominalization":
			return &p.Syntax.Nominalization, nil
		case "passive_tendency":
			return &p.Syntax.PassiveTendency, nil
		}
		return nil, fmt.Errorf("unsupported syntax field")
	case LayerStructure:
		switch field {
		case "intro_pattern":
			return &p.Structure.IntroPattern, nil
		case "closing_pattern":
			return &p.Structure.ClosingPattern, nil
		case "heading_habit":
			return &p.Structure.HeadingHabit, nil
		case "list_habit":
			return &p.Structure.ListHabit, nil
		case "emoji_use":
			return &p.Structure.EmojiUse, nil
		}
		return nil, fmt.Errorf("unsupported structure field")
	}
	return nil, fmt.Errorf("unsupported voice layer")
}

// axisField is the axis an override of (LayerAxes, field) replaces.
func axisField(p *StructuredProfile, field string) (**int, error) {
	switch field {
	case "involvement":
		return &p.Axes.Involvement, nil
	case "narrativity":
		return &p.Axes.Narrativity, nil
	case "persuasion_overtness":
		return &p.Axes.PersuasionOvertness, nil
	case "abstractness":
		return &p.Axes.Abstractness, nil
	case "addressee_focus":
		return &p.Axes.AddresseeFocus, nil
	case "humor":
		return &p.Axes.Humor, nil
	}
	return nil, fmt.Errorf("unsupported axis")
}

// rememberBase keeps the value a field held before its first override, so clearing the
// override can return it (VOICE-28). A field overridden again keeps its first base. The map is
// copied before it is written, since a profile value may share it with the snapshot it came from.
func (p *StructuredProfile) rememberBase(key string, current VoiceValue) {
	if _, kept := p.OverrideBase[key]; kept {
		return
	}
	base := make(map[string]VoiceValue, len(p.OverrideBase)+1)
	for k, v := range p.OverrideBase {
		base[k] = v
	}
	base[key] = current
	p.OverrideBase = base
}

func applyOverride(p *StructuredProfile, layer RuleLayer, field, value string) error {
	key := overrideKey(layer, field)
	if layer == LayerAxes {
		target, err := axisField(p, field)
		if err != nil {
			return err
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < -3 || parsed > 3 {
			return fmt.Errorf("axis must be between -3 and 3")
		}
		p.rememberBase(key, axisBase(*target))
		*target = &parsed
		return nil
	}
	target, err := voiceField(p, layer, field)
	if err != nil {
		return err
	}
	p.rememberBase(key, *target)
	*target = manual(value)
	return nil
}

// stripOverrides returns every field an override replaced to the value it held before, so a
// snapshot that baked overrides in reads as the analysis alone (VOICE-28).
func stripOverrides(p *StructuredProfile) error {
	for key, base := range p.OverrideBase {
		layer, field, _ := strings.Cut(key, ".")
		if RuleLayer(layer) == LayerAxes {
			target, err := axisField(p, field)
			if err != nil {
				return err
			}
			*target = axisFromBase(base)
			continue
		}
		target, err := voiceField(p, RuleLayer(layer), field)
		if err != nil {
			return err
		}
		*target = base
	}
	p.OverrideBase = nil
	return nil
}

// An axis carries no provenance of its own, so its base is kept as the text value it held.
func axisBase(value *int) VoiceValue {
	if value == nil {
		return VoiceValue{Unknown: true, Source: SourceUnknown}
	}
	return VoiceValue{Value: strconv.Itoa(*value), Source: SourceAnalyzed}
}

func axisFromBase(base VoiceValue) *int {
	parsed, err := strconv.Atoi(base.Value)
	if base.Unknown || err != nil {
		return nil
	}
	return &parsed
}
