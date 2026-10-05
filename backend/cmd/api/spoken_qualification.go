package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/fxrate"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/modelcatalog"
	catalogapp "github.com/postpilot/backend/internal/modelcatalog/app"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice/spoken"
	spokenapp "github.com/postpilot/backend/internal/voice/spoken/app"
	spokenstore "github.com/postpilot/backend/internal/voice/spoken/store"
	"io"
	"net/http"
	"os"
	"time"
)

type spokenQualificationCatalog struct{ service *modelcatalog.SpeechService }

func (s spokenQualificationCatalog) Session(ctx context.Context, owner, id string) (spokenapp.QualificationSession, error) {
	q, e := s.service.SpeechQualification(ctx, owner, id)
	return spokenapp.QualificationSession{ID: q.ID, OwnerID: q.OwnerID, ProfileID: q.ProfileID, Revision: q.Revision, MaximumUSD: q.MaximumUSD, ExpiresAt: q.ExpiresAt}, e
}
func (s spokenQualificationCatalog) Publish(ctx context.Context, e spokenapp.QualificationEvidence) error {
	return s.service.RecordVoiceQualification(ctx, modelcatalog.SpeechQualificationEvidence{OwnerID: e.OwnerID, SessionID: e.SessionID, ReportID: e.ReportID, DesignRequestID: e.DesignRequestID, ConfirmRequestID: e.ConfirmRequestID, ConfirmedVoiceID: e.ConfirmedVoiceID, SpeechRequestIDs: e.SpeechRequestIDs, AuditionAccepted: e.Review.AuditionAccepted, KoreanAccepted: e.Review.KoreanAccepted, ContinuityAccepted: e.Review.ContinuityAccepted, UsageVerified: true})
}

type spokenQualificationAccounting struct{ store *usagestore.Store }

func (s spokenQualificationAccounting) Job(ctx context.Context, owner, id string) (spokenapp.QualificationJob, error) {
	var out spokenapp.QualificationJob
	a, e := s.store.AccountingForJob(ctx, owner, id, []string{spoken.JobKindDesign, spoken.JobKindConfirm, spoken.JobKindProbe})
	if e != nil {
		return out, e
	}
	if a == nil || a.Approved == nil {
		return out, spokenapp.ErrQualificationEvidence
	}
	quoteID, bs, e := s.store.UnitAdmission(ctx, id)
	if e != nil {
		return out, e
	}
	q, e := s.store.GetUnitQuote(ctx, owner, quoteID)
	if e != nil || q.ConsumedJobID != id || q.MaxCredits != *a.Approved || len(bs) != len(q.Calls) {
		return out, spokenapp.ErrQualificationEvidence
	}
	for i := range bs {
		if bs[i].Fingerprint() != q.Calls[i].Fingerprint() {
			return out, spokenapp.ErrQualificationEvidence
		}
	}
	if a.Settled && (a.FinalCharge == nil || *a.FinalCharge > *a.Approved || (a.ShadowCharge != nil && *a.ShadowCharge > *a.Approved)) {
		return out, spokenapp.ErrQualificationBudget
	}
	usd, e := s.store.UnitCostForJob(ctx, id)
	if e != nil {
		return out, e
	}
	out = spokenapp.QualificationJob{Budgets: bs, ActualUSD: usd, Settled: a.Settled, Settlement: usage.Settlement{Reason: usage.TerminalOutcome(a.SettlementReason)}}
	if a.FinalCharge != nil {
		out.Settlement.Credits = *a.FinalCharge
	}
	return out, nil
}

// This is a producer beside the deployed API, not another server boot. It never
// sweeps running jobs, starts workers, recovers other contexts or changes tiers.
func newSpokenQualificationProducer(p *platform) *contexts {
	c := &contexts{platform: p}
	c.auth = auth.NewService(authstore.New(p.db.Writer, p.db.Reader), p.cfg.SessionTTL, auth.Deps{Mailer: p.mailer})
	us := usagestore.New(p.db.Writer, p.db.Reader)
	var rates usage.RateSource = unavailableRateSource{}
	if p.cfg.EximAPIKey != "" {
		rates = fxrate.NewEximbank(p.cfg.EximAPIKey, &http.Client{Timeout: 5 * time.Second})
	}
	c.ledger = usage.NewService(us, p.registry, int64(p.cfg.LLMMaxTokensDefault), usageAnchors{auth: c.auth, late: c}, usage.NewRateSelector(rates, us), approvedCeilingKinds()...).WithModelGrades().WithOwnerCancellation(ownerCancellableKinds()...).WithUnitAccounting(catalogapp.SpeechBudgets{Profiles: p.speechCatalog})
	js := jobstore.New(p.db.Writer, p.db.Reader, jobKinds())
	c.jobs = job.New(js, config.WorkerPollInterval, jobReporting{})
	c.jobs.AllowCancellation(jobCancellation{})
	c.jobs.Admit(jobAdmission{ledger: c.ledger, registry: p.registry, plans: c.auth, jobs: js})
	c.metered = meteredRegistry{Registry: p.registry, ledger: c.ledger}
	c.spoken = spoken.NewService(spokenstore.New(p.db.Writer, p.db.Reader), spokenProfiles{p.speechCatalog}, p.bucket)
	c.spokenGeneration = newSpokenGeneration(c)
	return c
}

type spokenQualificationFile struct {
	Version int                             `json:"version"`
	Input   spokenapp.QualificationInput    `json:"input"`
	Plan    *spokenapp.QualificationPlan    `json:"plan,omitempty"`
	Report  *spokenapp.QualificationReport  `json:"report,omitempty"`
	Session *spokenapp.QualificationSession `json:"session,omitempty"`
}

const spokenQualificationFileMax = 4 << 20

func readSpokenQualificationFile(path string) (spokenQualificationFile, []byte, error) {
	var result spokenQualificationFile
	f, e := os.Open(path)
	if e != nil {
		return result, nil, errors.New("private qualification input cannot be opened")
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || stat.Size() > spokenQualificationFileMax {
		return result, nil, errors.New("qualification input must be a bounded private regular file (0600)")
	}
	data, e := io.ReadAll(io.LimitReader(f, spokenQualificationFileMax+1))
	if e != nil || len(data) > spokenQualificationFileMax {
		return result, nil, errors.New("invalid qualification file")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&result); e != nil || result.Version != 1 {
		return result, nil, errors.New("invalid qualification file schema")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return result, nil, errors.New("trailing qualification file input")
	}
	return result, data, nil
}
func encodeSpokenQualificationFile(writer io.Writer, file spokenQualificationFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > spokenQualificationFileMax {
		return errors.New("qualification report exceeds private file limit")
	}
	_, err = writer.Write(append(data, '\n'))
	return err
}
func writeSpokenQualificationFile(path string, file spokenQualificationFile) error {
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("private qualification output must be a new file")
	}
	defer output.Close()
	return encodeSpokenQualificationFile(output, file)
}
func runSpokenQualification(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("spoken-qualification", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "preflight", "preflight, plan, run, audit or publish")
	inputPath := flags.String("input", "", "private input file")
	outputPath := flags.String("output", "", "new private output file")
	live := flags.Bool("live", false, "explicit live work or readiness publication")
	approval := flags.String("approve", "", "exact approved private plan digest")
	if e := flags.Parse(args); e != nil || flags.NArg() != 0 || *inputPath == "" {
		return errors.New("usage: api spoken-qualification --mode <preflight|plan|run|audit|publish> --input <0600 file> [--output <new file>] [--live --approve <exact digest>]")
	}
	if *mode != "preflight" && *mode != "plan" && *mode != "run" && *mode != "audit" && *mode != "publish" {
		return errors.New("unknown qualification mode")
	}
	if (*mode == "run" || *mode == "publish") && !*live {
		return spokenapp.ErrQualificationEvidence
	}
	file, data, e := readSpokenQualificationFile(*inputPath)
	if e != nil {
		return e
	}
	if *mode != "publish" && *outputPath == "" {
		return errors.New("a new private output file is required")
	}
	var output *os.File
	committed := false
	if *mode != "publish" {
		output, e = os.OpenFile(*outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return errors.New("private qualification output must be a new file")
		}
		defer func() {
			output.Close()
			if !committed {
				os.Remove(*outputPath)
			}
		}()
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	p, e := loadPlatform(ctx)
	if e != nil {
		return e
	}
	defer p.db.Close()
	c := newSpokenQualificationProducer(p)
	tier, e := c.auth.PlanOf(ctx, file.Input.OwnerID)
	if e != nil || tier != plan.Master {
		return spokenapp.ErrQualificationOnly
	}
	harness := spokenapp.NewQualificationHarness(c.spokenGeneration, spokenQualificationCatalog{p.speechCatalog}, spokenQualificationAccounting{usagestore.New(p.db.Writer, p.db.Reader)})
	switch *mode {
	case "preflight":
		q, e := harness.Preflight(ctx, file.Input.OwnerID, file.Input.SessionID)
		if e != nil {
			return e
		}
		file.Session = &q
	case "plan":
		plan, e := harness.Plan(ctx, file.Input)
		if e != nil {
			return e
		}
		file.Plan = &plan
	case "run":
		if file.Plan == nil || file.Plan.Input != file.Input {
			return spokenapp.ErrQualificationEvidence
		}
		report, runErr := harness.Run(ctx, *file.Plan, *live, *approval)
		file.Report = &report
		if runErr != nil {
			if encodeSpokenQualificationFile(output, file) == nil {
				committed = true
			}
			return runErr
		}
	case "audit":
		if file.Report == nil || file.Report.Plan.Input != file.Input {
			return spokenapp.ErrQualificationEvidence
		}
		report, e := harness.Audit(ctx, *file.Report)
		if e != nil {
			return e
		}
		file.Report = &report
	case "publish":
		if file.Report == nil || file.Report.Plan.Input != file.Input {
			return spokenapp.ErrQualificationEvidence
		}
		sum := sha256.Sum256(data)
		if e := harness.Publish(ctx, *file.Report, hex.EncodeToString(sum[:])); e != nil {
			return e
		}
		_, e = fmt.Fprintln(out, "Voice creation qualification published; narrated export readiness remains unchanged.")
		return e
	}
	if e := encodeSpokenQualificationFile(output, file); e != nil {
		return e
	}
	committed = true
	_, e = fmt.Fprintln(out, "Private qualification output written. No live qualification is implied by a preflight, plan or audit.")
	return e
}
