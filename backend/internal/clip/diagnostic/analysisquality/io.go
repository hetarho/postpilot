package analysisquality

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type verifierFactory func(string, string, string) (Verifier, error)

func execute(ctx context.Context, args []string, out io.Writer, now time.Time, factory verifierFactory) error {
	flags := flag.NewFlagSet("analysis-quality", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "inspect", "inspect, replay, audit")
	input := flags.String("input", "", "private corpus manifest")
	output := flags.String("output", "", "new private report directory")
	live := flags.Bool("live", false, "live work is unavailable until independently admitted")
	ffmpeg := flags.String("ffmpeg", "ffmpeg", "local ffmpeg executable")
	ffprobe := flags.String("ffprobe", "ffprobe", "local ffprobe executable")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *input == "" || *output == "" || !slices.Contains([]string{"inspect", "replay", "audit"}, *mode) || *ffmpeg == "" || *ffprobe == "" || factory == nil {
		return ErrInput
	}
	// No config boot, credential load, price GET, factory or reservation occurs
	// for --live. Supplying a JSON cap or claimed review cannot bypass this gate.
	if *live {
		return ErrLive
	}
	c, root, e := Load(*input, now)
	if e != nil {
		return ErrInput
	}
	if *mode == "audit" && c.HumanReview == nil {
		return ErrInput
	}
	dir, e := filepath.Abs(*output)
	if e != nil {
		return ErrOutput
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(dir))
	if e != nil || parent != filepath.Dir(dir) || os.Mkdir(dir, 0700) != nil {
		return ErrOutput
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	// Lazily construct the local media adapter after all immutable input and
	// replay files passed Run's preflight; invalid files construct nothing.
	var verify Verifier
	lazy := func(callCtx context.Context, in Input, data []byte) (verification clip.AnalysisCopyVerification, err error) {
		if verify == nil {
			verify, err = factory(filepath.Join(dir, "work"), *ffmpeg, *ffprobe)
			if err != nil {
				return verification, ErrInput
			}
		}
		return verify(callCtx, in, data)
	}
	r, runErr := Run(ctx, c, root, *mode, now, lazy)
	if e = writePrivate(filepath.Join(dir, "report.json"), r); e != nil {
		summary := r.Summary()
		summary.Status = "report_failed"
		_ = writePrivate(filepath.Join(dir, "summary.json"), summary)
		_ = json.NewEncoder(out).Encode(summary)
		return ErrOutput
	}
	if e = writePrivate(filepath.Join(dir, "summary.json"), r.Summary()); e != nil {
		return ErrOutput
	}
	if json.NewEncoder(out).Encode(r.Summary()) != nil {
		return ErrOutput
	}
	if runErr != nil {
		return safeError(runErr)
	}
	return nil
}

func writePrivate(path string, v any) error {
	data, e := json.MarshalIndent(v, "", "  ")
	if e != nil || len(data) > MaxDocumentBytes {
		return ErrOutput
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return ErrOutput
	}
	if _, e = f.Write(append(data, '\n')); e == nil {
		e = f.Sync()
	}
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return ErrOutput
	}
	return nil
}
