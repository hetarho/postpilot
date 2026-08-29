package voice

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func (s *Service) ListVersions(ctx context.Context, userID string) ([]ProfileVersion, error) {
	return s.personalization.ListProfileVersions(ctx, userID)
}

func (s *Service) UpdateOverride(ctx context.Context, userID string, layer RuleLayer, field string, value *string) (Profile, error) {
	if !validLayer(layer) || strings.TrimSpace(field) == "" {
		return Profile{}, fmt.Errorf("invalid voice override")
	}
	profile, err := s.Get(ctx, userID)
	if err != nil {
		return Profile{}, err
	}
	storedValue := value
	if value == nil {
		// Clearing reverts to the last measured/analyzed snapshot by replaying remaining overrides.
		versions, loadErr := s.personalization.ListProfileVersions(ctx, userID)
		if loadErr != nil {
			return Profile{}, loadErr
		}
		for _, version := range versions {
			if version.Origin == "analysis" {
				profile.Structured = version.Profile
				break
			}
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
	overrides, err := s.personalization.ListManualOverrides(ctx, userID)
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
	if err = s.personalization.ApplyOverrideAndPublish(ctx, ManualOverride{UserID: userID, Layer: layer, Field: field, UpdatedAt: now}, storedValue, profile.Structured, now); err != nil {
		return Profile{}, err
	}
	return s.Get(ctx, userID)
}

func (s *Service) RestoreVersion(ctx context.Context, userID string, version int64) (Profile, error) {
	found, err := s.personalization.GetProfileVersion(ctx, userID, version)
	if err != nil {
		return Profile{}, err
	}
	if _, err = s.personalization.PublishProfileVersion(ctx, userID, found.Profile, "restore", version, s.now()); err != nil {
		return Profile{}, err
	}
	return s.Get(ctx, userID)
}

func validLayer(layer RuleLayer) bool {
	return layer == LayerLexical || layer == LayerEndings || layer == LayerSyntax || layer == LayerStructure || layer == LayerAxes
}
func manual(value string) VoiceValue { return VoiceValue{Value: value, Source: SourceManual} }
func applyOverride(p *StructuredProfile, layer RuleLayer, field, value string) error {
	switch layer {
	case LayerLexical:
		if field != "description" {
			return fmt.Errorf("unsupported lexical field")
		}
		p.Lexical.Description = manual(value)
	case LayerEndings:
		if field != "base_register" {
			return fmt.Errorf("unsupported endings field")
		}
		p.Endings.BaseRegister = manual(value)
	case LayerSyntax:
		switch field {
		case "sentence_length":
			p.Syntax.SentenceLength = manual(value)
		case "connective_style":
			p.Syntax.ConnectiveStyle = manual(value)
		case "nominalization":
			p.Syntax.Nominalization = manual(value)
		case "passive_tendency":
			p.Syntax.PassiveTendency = manual(value)
		default:
			return fmt.Errorf("unsupported syntax field")
		}
	case LayerStructure:
		switch field {
		case "intro_pattern":
			p.Structure.IntroPattern = manual(value)
		case "closing_pattern":
			p.Structure.ClosingPattern = manual(value)
		case "heading_habit":
			p.Structure.HeadingHabit = manual(value)
		case "list_habit":
			p.Structure.ListHabit = manual(value)
		case "emoji_use":
			p.Structure.EmojiUse = manual(value)
		default:
			return fmt.Errorf("unsupported structure field")
		}
	case LayerAxes:
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < -3 || parsed > 3 {
			return fmt.Errorf("axis must be between -3 and 3")
		}
		switch field {
		case "involvement":
			p.Axes.Involvement = &parsed
		case "narrativity":
			p.Axes.Narrativity = &parsed
		case "persuasion_overtness":
			p.Axes.PersuasionOvertness = &parsed
		case "abstractness":
			p.Axes.Abstractness = &parsed
		case "addressee_focus":
			p.Axes.AddresseeFocus = &parsed
		case "humor":
			p.Axes.Humor = &parsed
		default:
			return fmt.Errorf("unsupported axis")
		}
	default:
		return fmt.Errorf("unsupported voice layer")
	}
	return nil
}
