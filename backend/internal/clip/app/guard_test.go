package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

type fakeAuthorizer struct{ calls int }

func (a *fakeAuthorizer) AuthorizeDispatch(context.Context, string, string) error {
	a.calls++
	return nil
}

// passAccess is an access checker that always admits.
type passAccess struct{}

func (passAccess) CheckAccess(context.Context, Hold) error { return nil }

// txRecorder is a writer whose only behaviour is transactions: it records how many
// were begun and whether one is open right now, so a test can tell what ran inside
// the writer from what ran before it.
type txRecorder struct {
	mu    sync.Mutex
	begun int
	open  bool
}

func (r *txRecorder) Connect(context.Context) (driver.Conn, error) { return recorderConn{r}, nil }
func (r *txRecorder) Driver() driver.Driver                        { return recorderDriver{r} }
func (r *txRecorder) state() (begun int, open bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.begun, r.open
}
func (r *txRecorder) set(open bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if open {
		r.begun++
	}
	r.open = open
}

type recorderDriver struct{ r *txRecorder }

func (d recorderDriver) Open(string) (driver.Conn, error) { return recorderConn(d), nil }

type recorderConn struct{ r *txRecorder }

func (recorderConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("recorder: no statements")
}
func (recorderConn) Close() error { return nil }
func (c recorderConn) Begin() (driver.Tx, error) {
	c.r.set(true)
	return recorderTx(c), nil
}

type recorderTx struct{ r *txRecorder }

func (t recorderTx) Commit() error   { t.r.set(false); return nil }
func (t recorderTx) Rollback() error { t.r.set(false); return nil }

// accessProbe records whether a transaction was open when the check ran.
type accessProbe struct {
	writer *txRecorder
	err    error
	calls  int
	inTx   bool
}

func (p *accessProbe) CheckAccess(context.Context, Hold) error {
	p.calls++
	if _, open := p.writer.state(); open {
		p.inTx = true
	}
	return p.err
}

// txAdmission records whether its hold ran inside a transaction.
type txAdmission struct {
	writer *txRecorder
	held   []Hold
	inTx   []bool
}

func (a *txAdmission) Hold(_ context.Context, h Hold) error {
	_, open := a.writer.state()
	a.held, a.inTx = append(a.held, h), append(a.inTx, open)
	return nil
}

func TestGuardChecksAccessBeforeTheWriterAndTellsTheHoldItRan(t *testing.T) {
	j := prepareJob()
	jobs := &fakeJobs{jobs: map[string]job.Job{"job": j}}
	recorder := &txRecorder{}
	writer := sql.OpenDB(recorder)
	t.Cleanup(func() { writer.Close() })
	admission := &txAdmission{writer: recorder}
	access := &accessProbe{writer: recorder}
	g := NewGuard(writer, bindFakes(jobs, newFakeClips(oldProject()), admission), &fakeAuthorizer{}, access)
	if err := g.Reserve(context.Background(), holdFor(j)); err != nil {
		t.Fatal(err)
	}
	if access.calls != 1 || access.inTx {
		t.Fatalf("access check calls=%d inside a transaction=%v, want one outside", access.calls, access.inTx)
	}
	if len(admission.held) != 1 || !admission.inTx[0] || !admission.held[0].AccessChecked {
		t.Fatalf("hold = %+v inTx=%v, want one in-transaction hold told the check ran", admission.held, admission.inTx)
	}
	if begun, open := recorder.state(); begun != 1 || open {
		t.Fatalf("transactions begun=%d open=%v, want one, closed", begun, open)
	}

	refused := errors.New("free path no longer qualifies")
	access.err = refused
	if err := g.Reserve(context.Background(), holdFor(j)); !errors.Is(err, refused) {
		t.Fatalf("err = %v, want the checker's refusal", err)
	}
	if begun, _ := recorder.state(); begun != 1 || len(admission.held) != 1 {
		t.Fatalf("a refused check opened a transaction (begun=%d) or reached the ledger (%d holds)", begun, len(admission.held))
	}
}

func TestGuardNeedsAnAccessChecker(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a guard without an access checker must not be constructible")
		}
	}()
	NewGuard(memoryWriter(t), bindFakes(&fakeJobs{}, newFakeClips(oldProject()), &fakeAdmission{}), &fakeAuthorizer{}, nil)
}

func prepareJob() job.Job {
	j := runningJob(clip.JobKindGenerate)
	j.Stage, j.CancellationPolicyVersion = "prepare", 1
	return j
}
func holdFor(j job.Job) Hold {
	return Hold{UserID: j.UserID, Kind: j.Kind, JobID: j.ID, Reservation: Reservation{CancellationPolicyVersion: 1}}
}

func TestReservableNamesEveryConditionAHoldNeeds(t *testing.T) {
	ok := prepareJob()
	if !Reservable(ok, holdFor(ok)) {
		t.Fatal("the reference job must be reservable")
	}
	at := time.Now()
	mutations := map[string]func(*job.Job, *Hold){
		"foreign user":     func(j *job.Job, _ *Hold) { j.UserID = "bob" },
		"foreign hold":     func(_ *job.Job, h *Hold) { h.UserID = "bob" },
		"render kind":      func(j *job.Job, _ *Hold) { j.Kind = clip.JobKindRender },
		"not running":      func(j *job.Job, _ *Hold) { j.Status = job.StatusQueued },
		"past prepare":     func(j *job.Job, _ *Hold) { j.Stage = "render" },
		"cancel requested": func(j *job.Job, _ *Hold) { j.CancelRequestedAt = &at },
		"other policy":     func(j *job.Job, _ *Hold) { j.CancellationPolicyVersion = 2 },
	}
	for name, mutate := range mutations {
		j, h := prepareJob(), holdFor(prepareJob())
		mutate(&j, &h)
		if Reservable(j, h) {
			t.Fatal(name, "must refuse a hold")
		}
	}
}

func TestGuardHoldsInsideTheTransactionOnlyForAReservableJob(t *testing.T) {
	j := prepareJob()
	jobs := &fakeJobs{jobs: map[string]job.Job{"job": j}}
	admission := &fakeAdmission{}
	authorizer := &fakeAuthorizer{}
	g := NewGuard(memoryWriter(t), bindFakes(jobs, newFakeClips(oldProject()), admission), authorizer, passAccess{})
	if err := g.Reserve(context.Background(), holdFor(j)); err != nil || len(admission.held) != 1 {
		t.Fatal(admission.held, err)
	}
	late := holdFor(j)
	late.Reservation.CancellationPolicyVersion = 2
	if err := g.Reserve(context.Background(), late); !errors.Is(err, clip.ErrCreditAllowance) || len(admission.held) != 1 {
		t.Fatal("a refused hold never reaches the ledger", err, admission.held)
	}
	if err := g.Authorize(context.Background(), "alice", "job"); err != nil || authorizer.calls != 1 {
		t.Fatal(err, authorizer.calls)
	}
	noLedger := NewGuard(memoryWriter(t), bindFakes(jobs, newFakeClips(oldProject()), nil), authorizer, passAccess{})
	if err := noLedger.Reserve(context.Background(), holdFor(j)); err == nil {
		t.Fatal("a missing ledger must fail closed")
	}
}
