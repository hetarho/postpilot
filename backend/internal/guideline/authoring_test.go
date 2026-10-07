package guideline

import (
	"errors"
	"testing"
)

func TestAuthoringScopeShapeKeepsKindAndRequiredLinkRulesInTheDomain(t *testing.T) {
	// Shape checks are pure even when the eventual owning-account directory has
	// no such id; publication separately proves membership.
	a := &Authoring{}
	for _, tc := range []struct {
		kind  Kind
		scope ScopePatch
		valid bool
	}{
		{KindPost, ScopePatch{Scope: ScopeGlobal}, true},
		{KindClip, ScopePatch{Scope: ScopeGlobal}, true},
		{KindPost, ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"owned", "owned"}}, true},
		{KindClip, ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"not-read-here"}}, true},
		{KindPost, ScopePatch{Scope: ScopeFields, Fields: []string{"not-read-here"}}, true},
		{KindPost, ScopePatch{}, false},
		{Kind("invalid"), ScopePatch{Scope: ScopeGlobal}, false},
		{KindPost, ScopePatch{Scope: ScopeGlobal, TemplateIDs: []string{"id"}}, false},
		{KindPost, ScopePatch{Scope: ScopeTemplates}, false},
		{KindPost, ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{" "}}, false},
		{KindPost, ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"id"}, Fields: []string{"field"}}, false},
		{KindPost, ScopePatch{Scope: ScopeFields}, false},
		{KindPost, ScopePatch{Scope: ScopeFields, Fields: []string{" "}}, false},
		{KindClip, ScopePatch{Scope: ScopeFields, Fields: []string{"field"}}, false},
	} {
		err := a.ValidateScopeShape(tc.kind, tc.scope)
		if (err == nil) != tc.valid {
			t.Fatalf("kind=%s scope=%+v err=%v valid=%v", tc.kind, tc.scope, err, tc.valid)
		}
		if !tc.valid && !errors.Is(err, ErrScopeShape) && !errors.Is(err, ErrTemplateNotFound) && !errors.Is(err, ErrFieldNotFound) {
			t.Fatalf("unexpected shape refusal=%v", err)
		}
	}
}
