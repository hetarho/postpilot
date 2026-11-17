package experiment

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

// memoryPhotoLedger is the photo rows in a map: the handshake and the sweep read and write the
// same rows, so one fake serves both ports.
type memoryPhotoLedger struct {
	rows       map[string]TestPhoto
	listErr    error
	deleteErr  map[string]error
	confirmHit int
}

func newMemoryPhotoLedger(photos ...TestPhoto) *memoryPhotoLedger {
	l := &memoryPhotoLedger{rows: map[string]TestPhoto{}, deleteErr: map[string]error{}}
	for _, photo := range photos {
		l.rows[photo.ID] = photo
	}
	return l
}

func (l *memoryPhotoLedger) CreateTestPhoto(_ context.Context, photo TestPhoto) error {
	l.rows[photo.ID] = photo
	return nil
}
func (l *memoryPhotoLedger) GetTestPhoto(_ context.Context, user, id string) (TestPhoto, error) {
	photo, ok := l.rows[id]
	if !ok || photo.UserID != user {
		return TestPhoto{}, ErrTestPhotoNotFound
	}
	return photo, nil
}
func (l *memoryPhotoLedger) ConfirmTestPhoto(_ context.Context, user, id string, bytes, width, height int64) (TestPhoto, error) {
	l.confirmHit++
	photo, err := l.GetTestPhoto(context.Background(), user, id)
	if err != nil {
		return TestPhoto{}, err
	}
	photo.Status, photo.Bytes, photo.Width, photo.Height = TestPhotoReady, bytes, width, height
	l.rows[id] = photo
	return photo, nil
}
func (l *memoryPhotoLedger) CountUnattachedTestPhotos(_ context.Context, user string) (int, error) {
	count := 0
	for _, photo := range l.rows {
		if photo.UserID == user && photo.TestID == "" {
			count++
		}
	}
	return count, nil
}
func (l *memoryPhotoLedger) DeleteUnattachedTestPhoto(_ context.Context, user, id string) (bool, error) {
	if err := l.deleteErr[id]; err != nil {
		return false, err
	}
	photo, ok := l.rows[id]
	if !ok || photo.UserID != user || photo.TestID != "" {
		return false, nil
	}
	delete(l.rows, id)
	return true, nil
}
func (l *memoryPhotoLedger) ListExpiredUnattachedTestPhotos(_ context.Context, before time.Time, limit int) ([]TestPhoto, error) {
	if l.listErr != nil {
		return nil, l.listErr
	}
	var out []TestPhoto
	for _, photo := range l.rows {
		if photo.TestID == "" && !photo.ExpiresAt.After(before) {
			out = append(out, photo)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (l *memoryPhotoLedger) ListTestPhotoKeys(context.Context) ([]string, error) {
	var keys []string
	for _, photo := range l.rows {
		keys = append(keys, photo.Key)
	}
	return keys, nil
}

// memoryPhotoObjects is a bucket in a map. Presigned URLs carry what was asked of them so a
// test can read the key, type and lifetime back.
type memoryPhotoObjects struct {
	objects   map[string]storedPhoto
	deleteErr map[string]error
	listErr   error
	heads     int
	presignOK bool
	lastPut   struct {
		key, contentType string
		ttl              time.Duration
	}
}

// storedPhoto is a listed object plus the size a HEAD reports for it.
type storedPhoto struct {
	TestPhotoObject
	size int64
}

func strayObject(key string, modified time.Time) storedPhoto {
	return storedPhoto{TestPhotoObject: TestPhotoObject{Key: key, LastModified: modified}}
}

func newMemoryPhotoObjects(objects ...storedPhoto) *memoryPhotoObjects {
	o := &memoryPhotoObjects{objects: map[string]storedPhoto{}, deleteErr: map[string]error{}, presignOK: true}
	for _, object := range objects {
		o.objects[object.Key] = object
	}
	return o
}

func (o *memoryPhotoObjects) PresignPut(_ context.Context, key, contentType string, ttl time.Duration) (string, error) {
	if !o.presignOK {
		return "", errors.New("presign unavailable")
	}
	o.lastPut.key, o.lastPut.contentType, o.lastPut.ttl = key, contentType, ttl
	return "https://storage.test/put/" + key, nil
}
func (o *memoryPhotoObjects) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/get/" + key, nil
}
func (o *memoryPhotoObjects) Head(_ context.Context, key string) (int64, error) {
	o.heads++
	object, ok := o.objects[key]
	if !ok {
		return 0, ErrTestObjectNotFound
	}
	return object.size, nil
}
func (o *memoryPhotoObjects) Delete(_ context.Context, key string) error {
	if err := o.deleteErr[key]; err != nil {
		return err
	}
	delete(o.objects, key)
	return nil
}
func (o *memoryPhotoObjects) List(_ context.Context, prefix string) ([]TestPhotoObject, error) {
	if o.listErr != nil {
		return nil, o.listErr
	}
	var out []TestPhotoObject
	for key, object := range o.objects {
		if strings.HasPrefix(key, prefix) {
			out = append(out, object.TestPhotoObject)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

var photoNow = time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)

const (
	photoPutTTL   = 10 * time.Minute
	photoGetTTL   = 10 * time.Minute
	photoTTL      = 24 * time.Hour
	photoMaxBytes = 10 << 20
)

func newPhotoService(ledger *memoryPhotoLedger, objects *memoryPhotoObjects, maxUnattached int) *TestPhotos {
	service := NewTestPhotos(ledger, objects, TestPhotoLimits{PutTTL: photoPutTTL, GetTTL: photoGetTTL, PhotoTTL: photoTTL, MaxBytes: photoMaxBytes, MaxUnattached: maxUnattached})
	service.now = func() time.Time { return photoNow }
	ids := 0
	service.newID = func() string { ids++; return string(rune('a'+ids-1)) + "id" }
	return service
}

func TestCreateTestPhotoUploadPresignsAPrivateJPEGKeyAndRecordsAPendingRow(t *testing.T) {
	ledger, objects := newMemoryPhotoLedger(), newMemoryPhotoObjects()
	service := newPhotoService(ledger, objects, 30)
	upload, err := service.Create(context.Background(), "alice", "  lunch.jpg ")
	if err != nil {
		t.Fatal(err)
	}
	wantKey := "tests/alice/aid.jpg"
	if objects.lastPut.key != wantKey || objects.lastPut.contentType != "image/jpeg" || objects.lastPut.ttl != photoPutTTL {
		t.Fatalf("presign = %+v, want %s as image/jpeg for %s", objects.lastPut, wantKey, photoPutTTL)
	}
	if upload.PutURL != "https://storage.test/put/"+wantKey || upload.ContentType != "image/jpeg" || !upload.PutExpires.Equal(photoNow.Add(photoPutTTL)) {
		t.Fatalf("upload = %+v", upload)
	}
	row := ledger.rows["aid"]
	if row.UserID != "alice" || row.Filename != "lunch.jpg" || row.Key != wantKey || row.Status != TestPhotoPending || row.TestID != "" || !row.ExpiresAt.Equal(photoNow.Add(photoTTL)) {
		t.Fatalf("recorded row = %+v", row)
	}
}

func TestCreateTestPhotoUploadLeavesNoRowWhenThePresignFailsAndRefusesAnEmptyName(t *testing.T) {
	ledger, objects := newMemoryPhotoLedger(), newMemoryPhotoObjects()
	service := newPhotoService(ledger, objects, 30)
	objects.presignOK = false
	if _, err := service.Create(context.Background(), "alice", "a.jpg"); err == nil || len(ledger.rows) != 0 {
		t.Fatalf("a failed presign must leave no reservation: err=%v rows=%d", err, len(ledger.rows))
	}
	objects.presignOK = true
	if _, err := service.Create(context.Background(), "alice", "  "); !errors.Is(err, ErrTestMaterialInvalid) {
		t.Fatalf("empty filename err = %v, want the material refusal", err)
	}
}

func TestCreateTestPhotoUploadRefusesAnOwnerAtTheUnattachedCapWithThePostPhotoLimit(t *testing.T) {
	ledger := newMemoryPhotoLedger(
		TestPhoto{ID: "p1", UserID: "alice", Status: TestPhotoReady},
		TestPhoto{ID: "p2", UserID: "alice", Status: TestPhotoPending},
		TestPhoto{ID: "p3", UserID: "alice", TestID: "attached", Status: TestPhotoReady},
	)
	service := newPhotoService(ledger, newMemoryPhotoObjects(), 2)
	_, err := service.Create(context.Background(), "alice", "third.jpg")
	var refusal *TestRefusal
	if !errors.As(err, &refusal) || refusal.Reason() != "POST_PHOTO_LIMIT" {
		t.Fatalf("at the cap err = %v, want the post photo-limit reason", err)
	}
	// A photo a test holds does not count, and another owner has their own cap.
	if _, err := service.Create(context.Background(), "bob", "ok.jpg"); err != nil {
		t.Fatalf("another owner was refused: %v", err)
	}
}

func readyObject(user, id string, size int64) storedPhoto {
	return storedPhoto{TestPhotoObject: TestPhotoObject{Key: testPhotoKey(user, id), LastModified: photoNow}, size: size}
}

func pendingPhoto(user, id string) TestPhoto {
	return TestPhoto{ID: id, UserID: user, Filename: id + ".jpg", Key: testPhotoKey(user, id), Status: TestPhotoPending, ExpiresAt: photoNow.Add(photoTTL), CreatedAt: photoNow}
}

func TestConfirmTestPhotoUploadRecordsTheMeasuredObjectAndIsIdempotent(t *testing.T) {
	ledger, objects := newMemoryPhotoLedger(pendingPhoto("alice", "p1")), newMemoryPhotoObjects(readyObject("alice", "p1", 4096))
	service := newPhotoService(ledger, objects, 30)
	photo, err := service.Confirm(context.Background(), "alice", "p1", 800, 600)
	if err != nil {
		t.Fatal(err)
	}
	if photo.Status != TestPhotoReady || photo.Bytes != 4096 || photo.Width != 800 || photo.Height != 600 {
		t.Fatalf("confirmed photo = %+v", photo)
	}
	again, err := service.Confirm(context.Background(), "alice", "p1", 800, 600)
	if err != nil || again != photo {
		t.Fatalf("repeat = %+v, %v; want the same photo", again, err)
	}
	if objects.heads != 1 || ledger.confirmHit != 1 {
		t.Fatalf("a repeat re-measured the object or rewrote the row: heads=%d confirms=%d", objects.heads, ledger.confirmHit)
	}
}

func TestConfirmTestPhotoUploadTreatsAForeignIdLikeAnUnknownOne(t *testing.T) {
	ledger, objects := newMemoryPhotoLedger(pendingPhoto("alice", "p1")), newMemoryPhotoObjects(readyObject("alice", "p1", 4096))
	service := newPhotoService(ledger, objects, 30)
	_, foreign := service.Confirm(context.Background(), "mallory", "p1", 800, 600)
	_, unknown := service.Confirm(context.Background(), "mallory", "nope", 800, 600)
	if !errors.Is(foreign, ErrTestNotFound) || !errors.Is(unknown, ErrTestNotFound) || foreign.Error() != unknown.Error() {
		t.Fatalf("foreign = %v, unknown = %v; both must be the not-found refusal", foreign, unknown)
	}
	// Even with bad dimensions: the owner check comes first, so nothing about the row leaks.
	if _, err := service.Confirm(context.Background(), "mallory", "p1", 0, 0); !errors.Is(err, ErrTestNotFound) {
		t.Fatalf("foreign id with bad dimensions err = %v", err)
	}
	if ledger.rows["p1"].Status != TestPhotoPending || objects.heads != 0 {
		t.Fatal("a foreign confirm touched the owner's photo")
	}
}

func TestConfirmTestPhotoUploadRefusesAMissingObjectAndKeepsTheRowForARetry(t *testing.T) {
	ledger, objects := newMemoryPhotoLedger(pendingPhoto("alice", "p1")), newMemoryPhotoObjects()
	service := newPhotoService(ledger, objects, 30)
	if _, err := service.Confirm(context.Background(), "alice", "p1", 800, 600); !errors.Is(err, ErrTestMaterialInvalid) {
		t.Fatalf("missing object err = %v", err)
	}
	if _, ok := ledger.rows["p1"]; !ok || ledger.rows["p1"].Status != TestPhotoPending {
		t.Fatal("the row must stay so the PUT can be retried with the same id")
	}
}

func TestConfirmTestPhotoUploadRefusesInvalidDimensions(t *testing.T) {
	for _, size := range [][2]int64{{0, 600}, {800, 0}, {-1, 600}, {800, 20001}, {20001, 600}} {
		ledger, objects := newMemoryPhotoLedger(pendingPhoto("alice", "p1")), newMemoryPhotoObjects(readyObject("alice", "p1", 4096))
		service := newPhotoService(ledger, objects, 30)
		if _, err := service.Confirm(context.Background(), "alice", "p1", size[0], size[1]); !errors.Is(err, ErrTestMaterialInvalid) {
			t.Fatalf("%dx%d err = %v", size[0], size[1], err)
		}
		if ledger.rows["p1"].Status != TestPhotoPending {
			t.Fatalf("%dx%d confirmed anyway", size[0], size[1])
		}
	}
	// The ceiling itself is allowed.
	ledger, objects := newMemoryPhotoLedger(pendingPhoto("alice", "p1")), newMemoryPhotoObjects(readyObject("alice", "p1", 4096))
	if _, err := newPhotoService(ledger, objects, 30).Confirm(context.Background(), "alice", "p1", 20000, 20000); err != nil {
		t.Fatalf("20000x20000 was refused: %v", err)
	}
}

func TestConfirmTestPhotoUploadDropsAnObjectOverTheSizeLimitAtOnce(t *testing.T) {
	for name, size := range map[string]int64{"over the limit": photoMaxBytes + 1, "empty": 0} {
		t.Run(name, func(t *testing.T) {
			ledger, objects := newMemoryPhotoLedger(pendingPhoto("alice", "p1")), newMemoryPhotoObjects(readyObject("alice", "p1", size))
			service := newPhotoService(ledger, objects, 30)
			if _, err := service.Confirm(context.Background(), "alice", "p1", 800, 600); !errors.Is(err, ErrTestMaterialInvalid) {
				t.Fatalf("err = %v", err)
			}
			if _, ok := ledger.rows["p1"]; ok {
				t.Fatal("the rejected row was left behind")
			}
			if _, ok := objects.objects[testPhotoKey("alice", "p1")]; ok {
				t.Fatal("the rejected object was left for the sweep")
			}
		})
	}
	// Exactly the limit is accepted.
	ledger, objects := newMemoryPhotoLedger(pendingPhoto("alice", "p1")), newMemoryPhotoObjects(readyObject("alice", "p1", photoMaxBytes))
	if _, err := newPhotoService(ledger, objects, 30).Confirm(context.Background(), "alice", "p1", 800, 600); err != nil {
		t.Fatalf("an object at the limit was refused: %v", err)
	}
}

func TestTestPhotoViewURLCarriesTheConfiguredLifetime(t *testing.T) {
	service := newPhotoService(newMemoryPhotoLedger(), newMemoryPhotoObjects(), 30)
	url, expires, err := service.ViewURL(context.Background(), "tests/alice/p1.jpg")
	if err != nil || url != "https://storage.test/get/tests/alice/p1.jpg" || !expires.Equal(photoNow.Add(photoGetTTL)) {
		t.Fatalf("view = %q %v %v", url, expires, err)
	}
}

func TestNewTestPhotosDefaultsAnUnsetLifetime(t *testing.T) {
	service := NewTestPhotos(newMemoryPhotoLedger(), newMemoryPhotoObjects(), TestPhotoLimits{})
	if service.limits.PhotoTTL != DefaultTestPhotoTTL || DefaultTestPhotoTTL != 24*time.Hour {
		t.Fatalf("photo ttl = %v", service.limits.PhotoTTL)
	}
}

func newPhotoSweeper(ledger *memoryPhotoLedger, objects *memoryPhotoObjects, minAge time.Duration) *TestPhotoSweeper {
	sweeper := NewTestPhotoSweeper(ledger, objects, minAge)
	sweeper.now = func() time.Time { return photoNow }
	return sweeper
}

func expiredPhoto(user, id string, age time.Duration) TestPhoto {
	photo := pendingPhoto(user, id)
	photo.ExpiresAt = photoNow.Add(-age)
	return photo
}

func TestTestPhotoSweepDeletesExpiredUnattachedRowsWithTheirObjectsAndKeepsTheRest(t *testing.T) {
	attached := expiredPhoto("alice", "held", 48*time.Hour)
	attached.TestID, attached.Status = "test-1", TestPhotoReady
	ledger := newMemoryPhotoLedger(
		expiredPhoto("alice", "gone", 2*time.Hour),
		expiredPhoto("bob", "gone2", 3*time.Hour),
		pendingPhoto("alice", "fresh"),
		attached,
	)
	objects := newMemoryPhotoObjects(
		readyObject("alice", "gone", 10), readyObject("bob", "gone2", 10),
		readyObject("alice", "fresh", 10), readyObject("alice", "held", 10),
	)
	newPhotoSweeper(ledger, objects, time.Hour).SweepOnce(context.Background())
	for _, id := range []string{"gone", "gone2"} {
		if _, ok := ledger.rows[id]; ok {
			t.Fatalf("expired row %s survived", id)
		}
	}
	for _, key := range []string{testPhotoKey("alice", "gone"), testPhotoKey("bob", "gone2")} {
		if _, ok := objects.objects[key]; ok {
			t.Fatalf("expired object %s survived", key)
		}
	}
	for _, id := range []string{"fresh", "held"} {
		if _, ok := ledger.rows[id]; !ok {
			t.Fatalf("row %s was swept", id)
		}
		if _, ok := objects.objects[testPhotoKey("alice", id)]; !ok {
			t.Fatalf("object of %s was swept", id)
		}
	}
}

func TestTestPhotoSweepWaitsOutTheMinimumAgePastExpiry(t *testing.T) {
	// Expired only a few minutes ago: a client that PUT just before its URL died may still answer.
	ledger := newMemoryPhotoLedger(expiredPhoto("alice", "recent", 10*time.Minute))
	objects := newMemoryPhotoObjects(readyObject("alice", "recent", 10))
	newPhotoSweeper(ledger, objects, time.Hour).SweepOnce(context.Background())
	if _, ok := ledger.rows["recent"]; !ok {
		t.Fatal("a row inside the grace period was swept")
	}
}

func TestTestPhotoSweepDeletesOnlyOldObjectsNoRowNames(t *testing.T) {
	old := photoNow.Add(-2 * time.Hour)
	ledger := newMemoryPhotoLedger(pendingPhoto("alice", "named"))
	objects := newMemoryPhotoObjects(
		strayObject(testPhotoKey("alice", "named"), old),
		strayObject("tests/alice/stray-old.jpg", old),
		strayObject("tests/alice/stray-young.jpg", photoNow.Add(-10*time.Minute)),
		strayObject("posts/slug/other.jpg", old),
		strayObject("voices/v/other.jpg", old),
	)
	newPhotoSweeper(ledger, objects, time.Hour).SweepOnce(context.Background())
	if _, ok := objects.objects["tests/alice/stray-old.jpg"]; ok {
		t.Fatal("an old object no row names survived")
	}
	for _, key := range []string{testPhotoKey("alice", "named"), "tests/alice/stray-young.jpg", "posts/slug/other.jpg", "voices/v/other.jpg"} {
		if _, ok := objects.objects[key]; !ok {
			t.Fatalf("%s was deleted", key)
		}
	}
}

func TestTestPhotoSweepOneFailingDeleteDoesNotStopThePass(t *testing.T) {
	old := photoNow.Add(-2 * time.Hour)
	ledger := newMemoryPhotoLedger(expiredPhoto("alice", "a", 3*time.Hour), expiredPhoto("alice", "b", 3*time.Hour), expiredPhoto("alice", "c", 3*time.Hour))
	objects := newMemoryPhotoObjects(
		readyObject("alice", "a", 1), readyObject("alice", "b", 1), readyObject("alice", "c", 1),
		strayObject("tests/alice/stray-1.jpg", old),
		strayObject("tests/alice/stray-2.jpg", old),
	)
	ledger.deleteErr["a"] = errors.New("row delete failed")
	objects.deleteErr[testPhotoKey("alice", "b")] = errors.New("object delete failed")
	objects.deleteErr["tests/alice/stray-1.jpg"] = errors.New("stray delete failed")
	newPhotoSweeper(ledger, objects, time.Hour).SweepOnce(context.Background())
	if _, ok := ledger.rows["a"]; !ok {
		t.Fatal("a row whose delete failed must stay for the next pass")
	}
	if _, ok := ledger.rows["b"]; ok {
		t.Fatal("the pass stopped before row b")
	}
	if _, ok := ledger.rows["c"]; ok {
		t.Fatal("the pass stopped before row c")
	}
	if _, ok := objects.objects[testPhotoKey("alice", "c")]; ok {
		t.Fatal("object c was not deleted after earlier failures")
	}
	if _, ok := objects.objects["tests/alice/stray-2.jpg"]; ok {
		t.Fatal("the stray after a failing stray was not deleted")
	}
}

func TestTestPhotoSweepAFailedListingDeletesNothing(t *testing.T) {
	old := photoNow.Add(-2 * time.Hour)
	ledger := newMemoryPhotoLedger(expiredPhoto("alice", "a", 3*time.Hour))
	objects := newMemoryPhotoObjects(readyObject("alice", "a", 1), strayObject("tests/alice/stray.jpg", old))
	objects.listErr = errors.New("listing failed")
	ledger.listErr = errors.New("rows unreadable")
	newPhotoSweeper(ledger, objects, time.Hour).SweepOnce(context.Background())
	if _, ok := objects.objects["tests/alice/stray.jpg"]; !ok {
		t.Fatal("a stray was deleted although the listing failed")
	}
	if _, ok := objects.objects[testPhotoKey("alice", "a")]; !ok {
		t.Fatal("an object was deleted although the expired rows could not be read")
	}
}

func TestTestPhotoSweepWorksThroughABacklogLargerThanOneBatch(t *testing.T) {
	var photos []TestPhoto
	for i := 0; i < testPhotoSweepBatch+25; i++ {
		photos = append(photos, expiredPhoto("alice", "p"+string(rune('A'+i/26))+string(rune('a'+i%26)), 3*time.Hour))
	}
	ledger, objects := newMemoryPhotoLedger(photos...), newMemoryPhotoObjects()
	newPhotoSweeper(ledger, objects, time.Hour).SweepOnce(context.Background())
	if len(ledger.rows) != 0 {
		t.Fatalf("%d expired rows survived the pass", len(ledger.rows))
	}
}
