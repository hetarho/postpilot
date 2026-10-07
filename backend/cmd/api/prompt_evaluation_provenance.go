package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

// This command reads repository source, never platform configuration or account
// data. Run it from a checkout: runtime deployment containers need not ship code.
type promptSourceLink struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Symbol string `json:"symbol,omitempty"`
	Line   int    `json:"line,omitempty"`
}

type promptProvenance struct {
	SourceStatus      string             `json:"source_status"`
	UnavailableReason string             `json:"unavailable_reason,omitempty"`
	GitRevision       string             `json:"git_revision"`
	GitTrackedDirty   *bool              `json:"git_tracked_dirty"`
	SourceSHA256      string             `json:"source_sha256"`
	Sources           []promptSourceLink `json:"sources"`
	IdentityNote      string             `json:"identity_note"`
}

type promptCodeLinks struct {
	Composer []promptSourceLink `json:"composer"`
	Schema   []promptSourceLink `json:"schema"`
	Parser   []promptSourceLink `json:"parser"`
	Consumer []promptSourceLink `json:"consumer"`
}

type promptTextMessage struct {
	Role llm.InspectionRole `json:"role"`
	Text string             `json:"text"`
}

type promptApplicationText struct {
	System   string                   `json:"system"`
	Messages []promptTextMessage      `json:"messages"`
	Native   []llm.RequestNativeField `json:"native_fields"`
	Scope    string                   `json:"scope"`
}

type promptInventoryEntry struct {
	Key                   string                `json:"key"`
	FixtureID             string                `json:"fixture_id,omitempty"`
	OriginProtocolVersion *int                  `json:"origin_protocol_version,omitempty"`
	Operation             string                `json:"operation"`
	CompositionSHA256     string                `json:"composition_sha256"`
	SchemaSHA256          string                `json:"schema_sha256"`
	TextSHA256            string                `json:"text_sha256"`
	Links                 promptCodeLinks       `json:"links"`
	Prepared              llm.RequestInspection `json:"prepared"`
	ApplicationText       promptApplicationText `json:"application_text"`
}

type promptInventoryReport struct {
	Version      int                      `json:"version"`
	Kind         string                   `json:"kind"`
	Material     string                   `json:"material"`
	SourceRoot   string                   `json:"source_root"`
	Provenance   promptProvenance         `json:"provenance"`
	Compositions []llm.RequestComposition `json:"compositions"`
	Entries      []promptInventoryEntry   `json:"entries"`
	Execution    promptOfflineExecution   `json:"execution"`
}

type promptOfflineExecution struct {
	Mode          string `json:"mode"`
	ProviderCalls int    `json:"provider_calls"`
	CustomerReads int    `json:"customer_reads"`
	PlatformBoot  bool   `json:"platform_boot"`
}

func promptHash(raw []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
}

func promptJSONHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return promptHash(raw), nil
}

func promptBackendRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		for _, candidate := range []string{current, filepath.Join(current, "backend")} {
			module, err := os.ReadFile(filepath.Join(candidate, "go.mod"))
			if err == nil && strings.HasPrefix(string(module), "module github.com/postpilot/backend\n") {
				return candidate, nil
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("prompt source provenance requires the Postpilot source checkout")
		}
		current = parent
	}
}

// Consumer links identify the execution owner, not a second prompt authority.
// These supplement the already authoritative descriptors with final consumption
// seams when their human-readable Consumer field does not name a Go symbol.
func promptConsumerSources(c llm.RequestComposition) []string {
	switch c.Stage {
	case "post-writing":
		return []string{"internal/generation/generate_handler.go", "internal/generation/writing_test_run.go"}
	case "post-observation":
		return []string{"internal/generation/generate_handler.go", "internal/generation/observe.go"}
	case "post-storyline":
		return []string{"internal/generation/storyline.go"}
	case "post-revision":
		return []string{"internal/generation/revise_handler.go"}
	case "setting-authoring":
		return []string{"internal/authoring/run.go"}
	case "writing-style":
		switch c.Mode {
		case "analyze":
			return []string{"internal/voice/analyze_handler.go", "internal/voice/analysis.go"}
		case "recommend":
			return []string{"internal/voice/candidates.go"}
		default:
			return []string{"internal/voice/check_handler.go", "internal/voice/reflection.go"}
		}
	case "memory-extraction":
		return []string{"internal/memory/extraction.go"}
	case "template-request":
		return []string{"internal/template/request.go"}
	case "video-composition", "video-observation":
		return []string{"internal/clip/ai/service.go"}
	case "spoken-voice":
		return []string{"internal/voice/spoken/app/generation.go", "internal/voice/spoken/app/probe.go", "internal/llm/speech.go"}
	case "video-speech":
		return []string{"internal/clip/app/speech.go", "internal/clip/app/initial_speech.go", "internal/llm/speech.go"}
	default:
		return nil
	}
}

func promptParserSources(c llm.RequestComposition) []string {
	switch c.Stage {
	case "post-writing", "post-revision":
		return []string{"internal/generation/blocks.go", "internal/generation/attach.go"}
	case "post-observation":
		return []string{"internal/generation/parse.go"}
	case "video-observation":
		return []string{"internal/clip/ai/parse.go"}
	case "video-composition":
		return []string{"internal/clip/ai/parse.go", "internal/clip/ai/storyline_parse.go"}
	case "spoken-voice", "video-speech":
		return []string{"internal/llm/speech.go", "internal/llm/speech_audio.go"}
	default:
		return nil
	}
}

func promptSourcePaths(compositions []llm.RequestComposition) []string {
	seen := map[string]bool{"go.mod": true, "go.sum": true, "cmd/api/commands.go": true, "cmd/api/prompt_inventory.go": true, "cmd/api/prompt_evaluation.go": true, "cmd/api/prompt_evaluation_provenance.go": true, "internal/llm/inspection.go": true, "internal/llm/request_evidence.go": true, "internal/generation/prompt_evaluation_compiler.go": true, "internal/generation/prompt_evaluation_fixtures.go": true, "internal/generation/prompt_evaluation_checks.go": true}
	for _, c := range compositions {
		for _, files := range [][]string{c.SourceFiles, promptConsumerSources(c), promptParserSources(c)} {
			for _, path := range files {
				seen[path] = true
			}
		}
		for _, fragment := range c.Fragments {
			for _, path := range fragment.SourceFiles {
				seen[path] = true
			}
		}
		for _, field := range c.NativeFields {
			for _, path := range field.SourceFiles {
				seen[path] = true
			}
		}
		for _, omission := range c.Omissions {
			for _, path := range omission.SourceFiles {
				seen[path] = true
			}
		}
	}
	var paths []string
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

type promptSourceIndex struct {
	files   map[string]promptSourceLink
	symbols map[string][]promptSourceLink
}

func loadPromptSources(root string, paths []string) (promptProvenance, promptSourceIndex, error) {
	provenance := promptProvenance{SourceStatus: "available", GitRevision: "unavailable", IdentityNote: "Git HEAD identifies the checkout; declared-source and composition SHA256 identify the inspected working source and executing compiler output, including uncommitted changes. No timestamp or runtime execution claim."}
	index := promptSourceIndex{files: map[string]promptSourceLink{}, symbols: map[string][]promptSourceLink{}}
	seen := map[string]bool{}
	var ordered []string
	for _, path := range paths {
		if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") {
			return provenance, index, fmt.Errorf("invalid repository source path %q", path)
		}
		if !seen[path] {
			seen[path] = true
			ordered = append(ordered, path)
		}
	}
	// Bind literal embedded output schemas as files as well as their composing
	// Go sources. Schema bytes remain independently hashed per actual request.
	for _, path := range append([]string(nil), ordered...) {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return provenance, index, fmt.Errorf("read prompt source %s: %w", path, err)
		}
		for _, directive := range promptEmbedDirectives.FindAllStringSubmatch(string(raw), -1) {
			for _, embedded := range strings.Fields(directive[1]) {
				if !strings.HasSuffix(embedded, ".json") || strings.Contains(embedded, "..") || filepath.IsAbs(embedded) {
					continue
				}
				matches, err := filepath.Glob(filepath.Join(root, filepath.Dir(path), embedded))
				if err != nil || len(matches) == 0 {
					return provenance, index, fmt.Errorf("embedded prompt schema %s/%s is unavailable", filepath.Dir(path), embedded)
				}
				for _, match := range matches {
					relative, err := filepath.Rel(root, match)
					if err != nil {
						return provenance, index, err
					}
					relative = filepath.ToSlash(relative)
					if !seen[relative] {
						seen[relative] = true
						ordered = append(ordered, relative)
					}
				}
			}
		}
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") {
			return provenance, index, fmt.Errorf("invalid repository source path %q", path)
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return provenance, index, fmt.Errorf("read prompt source %s: %w", path, err)
		}
		link := promptSourceLink{Path: "backend/" + path, SHA256: promptHash(raw)}
		index.files[path] = link
		provenance.Sources = append(provenance.Sources, link)
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		positions := token.NewFileSet()
		source, err := parser.ParseFile(positions, path, raw, 0)
		if err != nil {
			return provenance, index, fmt.Errorf("parse prompt source %s: %w", path, err)
		}
		for _, declaration := range source.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			location := link
			location.Symbol, location.Line = function.Name.Name, positions.Position(function.Pos()).Line
			index.symbols[path] = append(index.symbols[path], location)
		}
	}
	var err error
	provenance.SourceSHA256, err = promptJSONHash(provenance.Sources)
	if err != nil {
		return provenance, index, err
	}
	// Git metadata is optional and read-only. Missing Git or .git never prevents
	// source-file identities from being emitted and performs no network access.
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = root
	if output, err := command.Output(); err == nil {
		provenance.GitRevision = strings.TrimSpace(string(output))
		command = exec.Command("git", "diff", "--quiet", "HEAD", "--")
		command.Dir = root
		if err := command.Run(); err == nil {
			value := false
			provenance.GitTrackedDirty = &value
		} else if status, ok := err.(*exec.ExitError); ok && status.ExitCode() == 1 {
			value := true
			provenance.GitTrackedDirty = &value
		}
	}
	return provenance, index, nil
}

var promptSymbolWords = regexp.MustCompile(`[A-Za-z_][A-Za-z_0-9]*`)
var promptEmbedDirectives = regexp.MustCompile(`(?m)^//go:embed (.+)$`)

func (index promptSourceIndex) symbolLinks(description string, files []string) []promptSourceLink {
	words := map[string]bool{}
	for _, word := range promptSymbolWords.FindAllString(description, -1) {
		words[word] = true
	}
	var links []promptSourceLink
	seen := map[string]bool{}
	for _, path := range files {
		for _, link := range index.symbols[path] {
			key := fmt.Sprintf("%s:%d", link.Path, link.Line)
			if words[link.Symbol] && !seen[key] {
				seen[key] = true
				links = append(links, link)
			}
		}
	}
	return links
}

func (index promptSourceIndex) codeLinks(c llm.RequestComposition) promptCodeLinks {
	parserFiles := append(append([]string(nil), c.SourceFiles...), promptParserSources(c)...)
	parserDescription := c.Parser
	if len(c.NativeFields) > 0 {
		parserDescription += " Validate ValidateSpeechAlignment"
	}
	links := promptCodeLinks{Composer: index.symbolLinks(c.Composer, c.SourceFiles), Parser: index.symbolLinks(parserDescription, parserFiles)}
	for path, link := range index.files {
		if strings.HasSuffix(path, ".json") && link.SHA256 == promptHash([]byte(c.Output.Schema)) {
			links.Schema = append(links.Schema, link)
		}
	}
	sort.Slice(links.Schema, func(i, j int) bool { return links.Schema[i].Path < links.Schema[j].Path })
	for _, path := range c.SourceFiles {
		// Schemas can also be authored in the executable run/candidate/speech
		// source. Keep a file-level link when the descriptor has no named schema.
		if len(links.Schema) == 0 && (strings.Contains(filepath.Base(path), "schema") || path == "internal/authoring/run.go" || len(c.NativeFields) > 0) {
			links.Schema = append(links.Schema, index.files[path])
		}
	}
	if len(links.Schema) == 0 {
		for _, path := range c.SourceFiles {
			links.Schema = append(links.Schema, index.files[path])
		}
	}
	for _, path := range promptConsumerSources(c) {
		links.Consumer = append(links.Consumer, index.files[path])
	}
	if len(links.Parser) == 0 {
		// Native output validation and plain-text contracts need no JSON parser.
		// File links retain their owning descriptor rather than inventing symbols.
		for _, path := range promptConsumerSources(c) {
			links.Parser = append(links.Parser, index.files[path])
		}
	}
	return links
}

func promptUnavailableCodeLinks(c llm.RequestComposition) promptCodeLinks {
	var out promptCodeLinks
	for _, path := range c.SourceFiles {
		link := promptSourceLink{Path: "backend/" + path}
		out.Composer = append(out.Composer, link)
		out.Schema = append(out.Schema, link)
		out.Parser = append(out.Parser, link)
	}
	for _, path := range promptConsumerSources(c) {
		out.Consumer = append(out.Consumer, promptSourceLink{Path: "backend/" + path})
	}
	return out
}

func promptCompositionKey(c llm.RequestComposition) string {
	return c.Stage + "/" + c.Mode + "@" + c.PromptVersion + "/" + c.SchemaVersion
}

func promptComposedText(c llm.RequestComposition) promptApplicationText {
	out := promptApplicationText{Scope: "ordered safe text projection from authoritative composition; media represented by prepared fragment references; schema and transport framing excluded", Native: c.NativeFields}
	for _, fragment := range c.Fragments {
		if fragment.Role == llm.InspectionRoleSystem {
			out.System += fragment.Text
			continue
		}
		if len(out.Messages) == 0 || out.Messages[len(out.Messages)-1].Role != fragment.Role {
			out.Messages = append(out.Messages, promptTextMessage{Role: fragment.Role})
		}
		out.Messages[len(out.Messages)-1].Text += fragment.Text
	}
	return out
}

func buildPromptInventory(compositions []llm.RequestComposition) (promptInventoryReport, error) {
	out := promptInventoryReport{Version: 2, Kind: "code", Material: "synthetic", SourceRoot: "backend", Compositions: compositions, Execution: promptOfflineExecution{Mode: "offline"}}
	root, rootErr := promptBackendRoot()
	var sources promptSourceIndex
	if rootErr == nil {
		provenance, index, err := loadPromptSources(root, promptSourcePaths(compositions))
		if err != nil {
			return out, err
		}
		out.Provenance, sources = provenance, index
	} else {
		out.Provenance = promptProvenance{SourceStatus: "unavailable", UnavailableReason: rootErr.Error(), GitRevision: "unavailable", SourceSHA256: "unavailable", IdentityNote: "Composition and schema hashes identify executing compiler output; source hashes and declaration lines require the matching source checkout."}
	}
	seen := map[string]bool{}
	fixtures := map[string]generation.PromptInventoryFixture{}
	for _, fixture := range generation.PromptInventoryFixtures() {
		hash, err := promptJSONHash(fixture.Request.Composition)
		if err != nil {
			return out, err
		}
		fixtures[hash] = fixture
	}
	for _, composition := range compositions {
		key := promptCompositionKey(composition)
		compositionHash, err := promptJSONHash(composition)
		if err != nil {
			return out, err
		}
		fixture, hasFixture := fixtures[compositionHash]
		if hasFixture {
			key += "#" + fixture.ID
		}
		if seen[key] {
			return out, fmt.Errorf("duplicate prompt inventory key %s", key)
		}
		seen[key] = true
		prepared, err := llm.PreparedCompositionInspection(&composition)
		if err != nil {
			return out, fmt.Errorf("inspect prompt %s: %w", key, err)
		}
		text := promptComposedText(composition)
		if hasFixture {
			prepared, err = llm.PreparedRequestInspection(fixture.Request)
			if err != nil {
				return out, err
			}
			text = promptRequestApplicationText(fixture.Request)
		}
		textHash, err := promptJSONHash(text)
		if err != nil {
			return out, err
		}
		operation := "chat-text-and-media"
		if len(composition.NativeFields) > 0 {
			operation = "native-speech"
		}
		links := promptUnavailableCodeLinks(composition)
		if rootErr == nil {
			links = sources.codeLinks(composition)
		}
		entry := promptInventoryEntry{Key: key, Operation: operation, CompositionSHA256: compositionHash, SchemaSHA256: promptHash([]byte(composition.Output.Schema)), TextSHA256: textHash, Links: links, Prepared: prepared, ApplicationText: text}
		if hasFixture {
			protocol := fixture.OriginProtocolVersion
			entry.FixtureID, entry.OriginProtocolVersion = fixture.ID, &protocol
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}
