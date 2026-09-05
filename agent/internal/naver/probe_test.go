package naver

import (
	"strings"
	"testing"

	"github.com/postpilot/agent/internal/browser"
)

func validEvidence() browser.CompatibilityEvidence {
	return browser.CompatibilityEvidence{
		BrowserProduct: "Chrome/152.0.7977.76", ProtocolVersion: "1.3", TargetID: "page-1",
		TargetURL: "https://blog.naver.com/PostWriteForm.naver?blogId=alice",
		Domains:   []string{"Accessibility", "DOM", "Page", "Runtime", "Schema"}, AXRoles: []string{"RootWebArea"},
		Editor: browser.EditorSurface{EditorRoot: true, TitleEditor: true, BodyEditor: true, ImageControl: true, SettingsLayer: true, CategoryControl: true, CategoryOptions: 1, VisibilityChoice: true, TagsControl: true, FinalControl: true, ReadbackSurface: true},
	}
}

func TestCompatibilityManifestPinsTheSignedReleaseContract(t *testing.T) {
	manifest, err := Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || manifest.DriverVersion != "1.0.0" || manifest.SignatureID != "smarteditor-one-20260905-a1" || manifest.ProtocolVersion != "1.3" || manifest.ChromiumMinMajor != 136 || manifest.ChromiumMaxMajor != 152 || len(manifest.Capabilities) != 7 {
		t.Fatalf("manifest=%+v", manifest)
	}
	if err := validate(manifest, validEvidence()); err != nil {
		t.Fatal(err)
	}
}

func TestCompatibilityValidationRefusesEveryContractMismatch(t *testing.T) {
	manifest, err := Manifest()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*browser.CompatibilityEvidence){
		"browser product": func(value *browser.CompatibilityEvidence) { value.BrowserProduct = "Safari/18" },
		"browser range":   func(value *browser.CompatibilityEvidence) { value.BrowserProduct = "Chrome/153.0" },
		"protocol":        func(value *browser.CompatibilityEvidence) { value.ProtocolVersion = "1.4" },
		"domain":          func(value *browser.CompatibilityEvidence) { value.Domains = value.Domains[1:] },
		"accessibility":   func(value *browser.CompatibilityEvidence) { value.AXRoles = nil },
		"editor":          func(value *browser.CompatibilityEvidence) { value.Editor.FinalControl = false },
		"category":        func(value *browser.CompatibilityEvidence) { value.Editor.CategoryOptions = 0 },
		"target":          func(value *browser.CompatibilityEvidence) { value.TargetID = "" },
		"writer URL":      func(value *browser.CompatibilityEvidence) { value.TargetURL = "https://example.com" },
	} {
		t.Run(name, func(t *testing.T) {
			evidence := validEvidence()
			mutate(&evidence)
			if err := validate(manifest, evidence); err == nil || strings.TrimSpace(err.Error()) == "" {
				t.Fatal("mismatch was accepted")
			}
		})
	}
}
