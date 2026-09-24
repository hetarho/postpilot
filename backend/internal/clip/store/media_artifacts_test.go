package store_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/clip/workerclient"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/db"
)

type artifactObjects struct {
	objects  map[string]clip.SourceObjectInfo
	failSign bool
	onHead   func()
}

func (o *artifactObjects) PresignMediaRead(_ context.Context, key string, ttl time.Duration) (clip.MediaArtifactAccess, error) {
	return clip.MediaArtifactAccess{URL: "https://private.test/" + key, ExpiresAfter: ttl}, nil
}
func (o *artifactObjects) PresignMediaWrite(_ context.Context, key, contentType string, bytes int64, ttl time.Duration) (clip.MediaArtifactAccess, error) {
	if o.failSign {
		return clip.MediaArtifactAccess{}, errors.New("storage signing unavailable")
	}
	return clip.MediaArtifactAccess{URL: "https://private.test/" + key, Bytes: bytes, ContentType: contentType, ExpiresAfter: ttl, Headers: map[string]string{"If-None-Match": "*"}}, nil
}
func (o *artifactObjects) HeadMediaArtifact(_ context.Context, key string) (clip.SourceObjectInfo, error) {
	if o.onHead != nil {
		f := o.onHead
		o.onHead = nil
		f()
	}
	v, ok := o.objects[key]
	if !ok {
		return v, clip.ErrNotFound
	}
	return v, nil
}

type artifactFixture struct {
	a       *clipapp.MediaArtifacts
	st      *store.Store
	d       *db.DB
	bind    clipapp.Binder
	now     time.Time
	lease   clip.MediaLeaseCredentials
	task    clip.MediaTask
	result  clip.MediaResult
	objects *artifactObjects
}

func mediaArtifactsFixture(t *testing.T) *artifactFixture {
	t.Helper()
	service, st, d := setup(t)
	_, project := create(t, service)
	originals := fakeSources()
	sources := clipapp.NewSourceService(st, originals, clip.DefaultSourceLimits(clip.Environment{SourceBatchTTL: time.Hour, PutTTL: 10 * time.Minute}))
	created, err := sources.Create(t.Context(), "alice", project.ID, manifest(1))
	if err != nil {
		t.Fatal(err)
	}
	originals.upload(created.Batch)
	batch, err := sources.Confirm(t.Context(), "alice", created.Batch.ID, created.Batch.Sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	f := &artifactFixture{st: st, d: d, now: time.Now().UTC(), objects: &artifactObjects{objects: map[string]clip.SourceObjectInfo{}}}
	_, err = d.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,dispatch_ready,cancellation_policy_version,created_at,updated_at) VALUES('artifact-job','alice',?,'generate_clip','running',1,1,?,?)`, project.ID, f.now.Format(time.RFC3339Nano), f.now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.LinkSourceJob(t.Context(), "alice", batch.ID, "artifact-job", f.now); err != nil {
		t.Fatal(err)
	}
	source := batch.Sources[0]
	f.task = clip.MediaTask{Version: clip.MediaContractVersion, Sources: []clip.MediaTaskSource{{ID: source.ID, SourceMetadata: source.SourceMetadata}}}
	payload, err := mediacodec.EncodeTask(f.task)
	if err != nil {
		t.Fatal(err)
	}
	q, err := clipapp.NewMediaStages(st, clip.DefaultMediaStageLimits(clip.Environment{}), func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	_, err = q.Create(t.Context(), clip.MediaStageInput{ID: "artifact-stage", ParentJobID: "artifact-job", UserID: "alice", ProjectID: project.ID, ExpectedRevision: project.EditPlanRevision, Operation: clip.MediaPrepare, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Payload: payload, InputDigest: clip.MediaPayloadDigest(payload)})
	if err != nil {
		t.Fatal(err)
	}
	p := mediaProfile()
	p.Operation = clip.MediaPrepare
	lease, err := q.Claim(t.Context(), p)
	if err != nil || lease == nil {
		t.Fatal(err)
	}
	f.lease = lease.Credentials
	f.bind = func(tx *sql.Tx) clipapp.Ports {
		c := store.NewTx(tx)
		j := jobstore.NewTx(tx, jobKindsForTest())
		return clipapp.Ports{Clips: c, Media: c, Jobs: j, Waits: j}
	}
	f.a = clipapp.NewMediaArtifacts(d.Writer, f.bind, f.objects, clip.DefaultMediaConfig(clip.Environment{}), func() time.Time { return f.now })
	f.result = clip.MediaResult{Version: clip.MediaContractVersion, Sources: []clip.MediaVerifiedSource{{ID: source.ID, Fingerprint: source.Fingerprint, Info: clip.MediaInfo{Width: source.Width, Height: source.Height, DurationMS: source.DurationMS}}}, Outputs: []clip.MediaOutput{{Slot: clip.MediaAnalysisSlot(source.ID, 0), SourceID: source.ID, Bytes: 50, ContentType: "video/mp4", Digest: strings.Repeat("a", 64), DurationMS: 1000, Info: clip.MediaInfo{Width: 720, Height: 406, DurationMS: 1000}}}}
	return f
}
func (f *artifactFixture) encoded(t *testing.T) string {
	t.Helper()
	raw, err := mediacodec.EncodeResult(f.result)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func (f *artifactFixture) reserveAndUpload(t *testing.T) []clip.MediaArtifact {
	t.Helper()
	if _, err := f.a.Reserve(t.Context(), f.lease, f.result.Outputs); err != nil {
		t.Fatal(err)
	}
	artifacts, err := f.st.MediaArtifacts(t.Context(), f.lease.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range artifacts {
		f.objects.objects[a.ObjectKey] = clip.SourceObjectInfo{Bytes: a.Bytes, ContentType: a.ContentType}
	}
	return artifacts
}

func TestMediaArtifactsReadOnlyCurrentOriginals(t *testing.T) {
	f := mediaArtifactsFixture(t)
	slot := "source/" + f.task.Sources[0].ID
	access, err := f.a.Read(t.Context(), f.lease, slot)
	if err != nil || access.Bytes != 100 || access.ExpiresAfter > 10*time.Minute || !strings.Contains(access.URL, clip.SourcePrefix) {
		t.Fatal("input access", err)
	}
	for _, slot := range []string{"clip-inputs/bob/foreign.mp4", "source/another-source", f.result.Outputs[0].Slot} {
		if _, err := f.a.Read(t.Context(), f.lease, slot); err == nil {
			t.Fatal("foreign/unrelated input signed")
		}
	}
	other := f.lease
	other.WorkerID = "other"
	if _, err := f.a.Read(t.Context(), other, slot); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("stolen input access", err)
	}
	if _, err := f.d.Writer.Exec(`UPDATE clip_source_leases SET retention_expires_at=?`, f.now.Add(-time.Second).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.a.Read(t.Context(), f.lease, slot); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal("expired original signed", err)
	}
}

func TestMediaArtifactsReservationReceiptAndRestart(t *testing.T) {
	f := mediaArtifactsFixture(t)
	f.objects.failSign = true
	if _, err := f.a.Reserve(t.Context(), f.lease, f.result.Outputs); err == nil {
		t.Fatal("expected lost signing response")
	}
	rows, err := f.st.MediaArtifacts(t.Context(), f.lease.AttemptID)
	if err != nil || len(rows) != 1 {
		t.Fatal("reservation was not durable", err)
	}
	key := rows[0].ObjectKey
	outputSlot := "output/" + rows[0].Slot
	read, err := f.a.Read(t.Context(), f.lease, outputSlot)
	if err != nil || read.Bytes != rows[0].Bytes || !strings.HasSuffix(read.URL, key) {
		t.Fatal("own candidate read", err)
	}
	stolen := f.lease
	stolen.WorkerID = "foreign"
	if _, err = f.a.Read(t.Context(), stolen, outputSlot); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("foreign candidate signed", err)
	}
	if _, err = f.a.Read(t.Context(), f.lease, "output/foreign"); err == nil {
		t.Fatal("unknown candidate signed")
	}

	f.objects.failSign = false
	access, err := f.a.Reserve(t.Context(), f.lease, f.result.Outputs)
	if err != nil || len(access) != 1 || !strings.HasSuffix(access[0].URL, key) || access[0].Headers["If-None-Match"] != "*" {
		t.Fatal("reservation response replay", err)
	}
	changed := append([]clip.MediaOutput(nil), f.result.Outputs...)
	changed[0].Bytes++
	if _, err := f.a.Reserve(t.Context(), f.lease, changed); !errors.Is(err, clip.ErrMediaConflict) {
		t.Fatal("immutable reservation changed", err)
	}
	if _, err := f.a.Accept(t.Context(), f.lease, f.encoded(t)); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("missing file accepted", err)
	}
	for _, info := range []clip.SourceObjectInfo{{Bytes: 51, ContentType: "video/mp4"}, {Bytes: 50, ContentType: "text/plain"}} {
		f.objects.objects[key] = info
		if _, err := f.a.Accept(t.Context(), f.lease, f.encoded(t)); !errors.Is(err, clip.ErrInvalidMedia) {
			t.Fatal("size/type mismatch accepted", err)
		}
	}
	f.objects.objects[key] = clip.SourceObjectInfo{Bytes: 50, ContentType: "video/mp4"}
	accepted, err := f.a.Accept(t.Context(), f.lease, f.encoded(t))
	if err != nil || accepted.State != clip.MediaSucceeded {
		t.Fatal(err)
	}
	f.now = f.now.Add(3 * time.Hour)
	restarted := clipapp.NewMediaArtifacts(f.d.Writer, f.bind, f.objects, clip.DefaultMediaConfig(clip.Environment{}), func() time.Time { return f.now })
	again, err := restarted.Accept(t.Context(), f.lease, f.encoded(t))
	if err != nil || again.AcceptedResult != accepted.AcceptedResult {
		t.Fatal("restart lost receipt", err)
	}
	rows, err = f.st.MediaArtifacts(t.Context(), f.lease.AttemptID)
	if err != nil || rows[0].State != "accepted" || rows[0].Digest != f.result.Outputs[0].Digest {
		t.Fatal("accepted artifact not retained", err)
	}
	if _, err := restarted.Reserve(t.Context(), f.lease, f.result.Outputs); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("terminal URL refresh allowed", err)
	}
	keys, err := f.st.ResultKeys(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range keys {
		if v == key {
			found = true
		}
	}
	if !found {
		t.Fatal("live artifact omitted from protected keys")
	}
}

func TestMediaArtifactsRecheckAuthorityAfterStorageIO(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		f := mediaArtifactsFixture(t)
		f.reserveAndUpload(t)
		f.objects.onHead = func() {
			if cancel {
				if _, err := f.d.Writer.Exec(`UPDATE generation_jobs SET cancel_requested_at=? WHERE id='artifact-job'`, f.now.Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
			} else {
				f.now = f.now.Add(2 * time.Minute)
			}
		}
		_, err := f.a.Accept(t.Context(), f.lease, f.encoded(t))
		want := clip.ErrMediaLeaseLost
		if cancel {
			want = clip.ErrMediaCancelled
		}
		if !errors.Is(err, want) {
			t.Fatal("storage race accepted", err)
		}
		rows, err := f.st.MediaArtifacts(t.Context(), f.lease.AttemptID)
		if err != nil || rows[0].State != "reserved" {
			t.Fatal("partial acceptance", err)
		}
	}
}

func TestMediaArtifactsDeletionIntentSurvivesCascade(t *testing.T) {
	f := mediaArtifactsFixture(t)
	artifacts := f.reserveAndUpload(t)
	if _, err := f.d.Writer.Exec(`DELETE FROM clip_media_stages WHERE id=?`, f.lease.StageID); err != nil {
		t.Fatal(err)
	}
	var key, notBefore string
	if err := f.d.Reader.QueryRow(`SELECT object_key,not_before FROM clip_media_deletions`).Scan(&key, &notBefore); err != nil {
		t.Fatal(err)
	}
	at, err := time.Parse(time.RFC3339Nano, notBefore)
	if err != nil || key != artifacts[0].ObjectKey || !at.After(f.now) {
		t.Fatal("lost orphan key or live PUT expiry")
	}
}

func TestMediaArtifactsHTTPMapsAccessAndVerifiedCompletion(t *testing.T) {
	f := mediaArtifactsFixture(t)
	service := clipapp.NewMediaWorker(f.st, f.a, func() time.Time { return f.now })
	httpAPI := httptest.NewServer(cliprpc.NewMediaWorkerServer("", map[string]string{"worker-1": "token"}, service).Handler)
	defer httpAPI.Close()
	c := workerclient.New(httpAPI.URL, "worker-1", "token")
	input, err := c.Read(t.Context(), f.lease, "source/"+f.task.Sources[0].ID)
	if err != nil || input.Bytes != 100 {
		t.Fatal("mapped input", err)
	}
	outputs, err := c.Reserve(t.Context(), f.lease, f.result.Outputs)
	if err != nil || len(outputs) != 1 || outputs[0].Headers["If-None-Match"] != "*" {
		t.Fatal("mapped reservation", err)
	}
	rows, err := f.st.MediaArtifacts(t.Context(), f.lease.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	f.objects.objects[rows[0].ObjectKey] = clip.SourceObjectInfo{Bytes: 50, ContentType: "video/mp4"}
	if err := c.Complete(t.Context(), f.lease, f.encoded(t)); err != nil {
		t.Fatal("mapped complete", err)
	}
}
