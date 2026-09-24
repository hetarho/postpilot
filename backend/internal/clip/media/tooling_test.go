package media

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// One real listing, kept verbatim, so the parser is tested against the shape ffmpeg
// actually prints: a legend, a dashes-only rule, then rows whose second field is the name.
const sampleListing = `Filters:
  T.. = Timeline support
  .S. = Slice threading
  A = Audio input/output
  | = Source or sink filter
  ------
 .. acrossfade        N->A       Cross fade two input audio streams.
 T. afade             A->A       Fade in/out input audio.
 ..C scale            V->V       Scale the input video size.
`

// splitOutsideQuotes splits on sep, ignoring separators inside the single quotes ffmpeg
// wraps expression arguments in — crop's x='max(0,min(iw-ow,...))' carries its own commas.
func splitOutsideQuotes(s string, sep byte) []string {
	var parts []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'':
			quoted = !quoted
			cur.WriteByte(c)
		case c == sep && !quoted:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(parts, cur.String())
}

// filterNames pulls the filter names out of a filtergraph, dropping the [in]/[out] labels
// that bracket each chain and the arguments after the first `=`. A fixture may carry more
// than one graph, one per line, each headed by a label line like `delivered inputs=4`;
// splitting on newlines first keeps a header from swallowing the graph beneath it, and
// the name shape below discards the header itself.
func filterNames(graph string) []string {
	var names []string
	for _, line := range strings.Split(graph, "\n") {
		names = append(names, filterNamesInLine(line)...)
	}
	return names
}

// isFilterName reports whether s has the shape of an ffmpeg filter name. Anything else is
// a fixture header or a stray label, not something to look for in the binary.
func isFilterName(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func filterNamesInLine(graph string) []string {
	var names []string
	for _, chain := range splitOutsideQuotes(graph, ';') {
		for _, entry := range splitOutsideQuotes(chain, ',') {
			entry = strings.TrimSpace(entry)
			for strings.HasPrefix(entry, "[") {
				end := strings.Index(entry, "]")
				if end < 0 {
					break
				}
				entry = entry[end+1:]
			}
			if cut := strings.IndexAny(entry, "[="); cut >= 0 {
				entry = entry[:cut]
			}
			if isFilterName(entry) {
				names = append(names, entry)
			}
		}
	}
	return names
}

// reportedNames runs one of ffmpeg's listings and returns what it carries.
func reportedNames(t *testing.T, binary, flag string) map[string]bool {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), binary, "-hide_banner", flag).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", binary, flag, err, out)
	}
	return parseListing(string(out))
}

func missingFrom(reported map[string]bool, required []string) []string {
	var missing []string
	for _, name := range required {
		if !reported[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// The half that needs no Docker, so forgetting to declare a new filter fails ARCH-26
// rather than waiting for an image build (→ARCH-39).
func TestGoldenFiltersAreDeclared(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range requiredFilters {
		declared[name] = true
	}
	fixtures, err := filepath.Glob(filepath.Join("testdata", "*.filter"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no filtergraph fixtures found: this check would pass vacuously")
	}
	undeclared := map[string]string{}
	for _, path := range fixtures {
		graph, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range filterNames(string(graph)) {
			if !declared[name] {
				undeclared[name] = filepath.Base(path)
			}
		}
	}
	if len(undeclared) > 0 {
		names := make([]string, 0, len(undeclared))
		for name, fixture := range undeclared {
			names = append(names, name+" ("+fixture+")")
		}
		sort.Strings(names)
		t.Fatalf("filtergraph fixtures use filters requiredFilters does not declare: %s", strings.Join(names, ", "))
	}
}

// The half that needs the real binaries, so it runs in its own image stage (→ARCH-36).
func TestBundledToolingCoverage(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("the bundled binaries live in the image")
	}
	cfg := mediaConfig(t)
	for _, group := range []struct {
		kind     string
		flag     string
		required []string
	}{
		{"filter", "-filters", requiredFilters},
		{"decoder", "-decoders", requiredDecoders},
		{"encoder", "-encoders", requiredEncoders},
		{"muxer", "-muxers", requiredMuxers},
	} {
		t.Run(group.kind, func(t *testing.T) {
			missing := missingFrom(reportedNames(t, cfg.FFmpegPath, group.flag), group.required)
			if len(missing) > 0 {
				t.Fatalf("the bundled ffmpeg carries no %s %s — enable it in backend/build/media-tools.sh and remember the configure flag may spell it differently",
					group.kind, strings.Join(missing, ", "))
			}
		})
	}
}

func TestListingParserReadsNamesAndReportsAbsentOnes(t *testing.T) {
	reported := parseListing(sampleListing)
	for _, name := range []string{"acrossfade", "afade", "scale"} {
		if !reported[name] {
			t.Errorf("parser lost %q, which the listing carries", name)
		}
	}
	for _, name := range []string{"Filters", "T..", "A->A", "atempo"} {
		if reported[name] {
			t.Errorf("parser invented %q, which the listing does not carry as a name", name)
		}
	}
	missing := missingFrom(reported, []string{"afade", "atempo"})
	if len(missing) != 1 || missing[0] != "atempo" {
		t.Fatalf("a name absent from the binaries must be reported: got %v", missing)
	}
}

func TestFilterNamesReadsAQuotedGraph(t *testing.T) {
	// A real shape: bracketed labels, an expression argument carrying commas, and a
	// filter with no arguments at all.
	graph := "[0:V:0]trim=duration=7.600,crop=1920:1080:x='max(0,min(iw-ow,iw*0.25))',setsar=1[base];[base]vfrdet[v]"
	got := filterNames(graph)
	want := []string{"trim", "crop", "setsar", "vfrdet"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("filterNames(%q) = %v, want %v", graph, got, want)
	}
}
