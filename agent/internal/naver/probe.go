// Package naver owns the reviewed, versioned compatibility contract and deterministic
// publisher. The server can report its opaque version but cannot supply selectors or code.
package naver

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/postpilot/agent/internal/browser"
)

//go:embed compatibility.json
var manifestJSON []byte

type CompatibilityManifest struct {
	SchemaVersion    int      `json:"schema_version"`
	DriverVersion    string   `json:"driver_version"`
	SignatureID      string   `json:"signature_id"`
	ProtocolVersion  string   `json:"protocol_version"`
	ChromiumMinMajor int      `json:"chromium_min_major"`
	ChromiumMaxMajor int      `json:"chromium_max_major"`
	RequiredDomains  []string `json:"required_domains"`
	RequiredAXRoles  []string `json:"required_ax_roles"`
	Capabilities     []string `json:"capabilities"`
}

type Result struct {
	Identity        browser.NaverIdentity
	ExecutorVersion string
	BrowserVersion  string
	SignatureID     string
}

var chromiumProduct = regexp.MustCompile(`^(?:Chrome|Chromium|HeadlessChrome)/(\d+)(?:\.|$)`)

func Manifest() (CompatibilityManifest, error) {
	var manifest CompatibilityManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return CompatibilityManifest{}, fmt.Errorf("decode Naver compatibility manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 || manifest.DriverVersion == "" || manifest.SignatureID == "" ||
		manifest.ProtocolVersion == "" || manifest.ChromiumMinMajor <= 0 || manifest.ChromiumMaxMajor < manifest.ChromiumMinMajor ||
		len(manifest.RequiredDomains) == 0 || len(manifest.RequiredAXRoles) == 0 || len(manifest.Capabilities) == 0 {
		return CompatibilityManifest{}, errors.New("Naver compatibility manifest is incomplete")
	}
	return manifest, nil
}

// Probe performs no publishing mutation. ObserveNaverIdentity may open the local publish
// settings and category list; InspectCompatibility then verifies the same sole target and
// read-only surface against the manifest embedded in this signed agent release.
func Probe(ctx context.Context, cdpURL string) (Result, error) {
	manifest, err := Manifest()
	if err != nil {
		return Result{}, err
	}
	identity, err := browser.ObserveNaverIdentity(ctx, cdpURL)
	if err != nil {
		return Result{}, fmt.Errorf("verify Naver identity: %w", err)
	}
	evidence, err := browser.InspectCompatibility(ctx, cdpURL)
	if err != nil {
		return Result{}, fmt.Errorf("inspect Naver compatibility: %w", err)
	}
	if err := validate(manifest, evidence); err != nil {
		return Result{}, err
	}
	return Result{
		Identity: identity, BrowserVersion: evidence.BrowserProduct, SignatureID: manifest.SignatureID,
		ExecutorVersion: "postpilot-naver/" + manifest.DriverVersion + "-" + manifest.SignatureID,
	}, nil
}

func validate(manifest CompatibilityManifest, evidence browser.CompatibilityEvidence) error {
	match := chromiumProduct.FindStringSubmatch(evidence.BrowserProduct)
	if len(match) != 2 {
		return fmt.Errorf("unsupported Chromium product %q", evidence.BrowserProduct)
	}
	major, _ := strconv.Atoi(match[1])
	if major < manifest.ChromiumMinMajor || major > manifest.ChromiumMaxMajor {
		return fmt.Errorf("Chromium %d is outside the reviewed range %d-%d", major, manifest.ChromiumMinMajor, manifest.ChromiumMaxMajor)
	}
	if evidence.ProtocolVersion != manifest.ProtocolVersion {
		return fmt.Errorf("CDP protocol %q is not the reviewed %q", evidence.ProtocolVersion, manifest.ProtocolVersion)
	}
	for _, domain := range manifest.RequiredDomains {
		if !slices.Contains(evidence.Domains, domain) {
			return fmt.Errorf("required CDP domain %s is unavailable", domain)
		}
	}
	for _, role := range manifest.RequiredAXRoles {
		if !slices.Contains(evidence.AXRoles, role) {
			return fmt.Errorf("required accessibility role %s is unavailable", role)
		}
	}
	surface := evidence.Editor
	if !surface.EditorRoot || !surface.TitleEditor || !surface.BodyEditor || !surface.ImageControl ||
		!surface.SettingsLayer || !surface.CategoryControl || surface.CategoryOptions < 1 ||
		!surface.VisibilityChoice || !surface.TagsControl || !surface.FinalControl || !surface.ReadbackSurface {
		return errors.New("Naver editor does not match the reviewed compatibility signature")
	}
	if strings.TrimSpace(evidence.TargetID) == "" || !strings.HasPrefix(evidence.TargetURL, "https://blog.naver.com/PostWriteForm.naver") {
		return errors.New("compatibility evidence is not bound to the Naver writer target")
	}
	return nil
}
