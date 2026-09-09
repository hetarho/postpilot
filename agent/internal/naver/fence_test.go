package naver

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
)

const (
	fakePermalink = "https://blog.naver.com/alice/223456789"
	fakePostView  = "https://blog.naver.com/PostView.naver?blogId=alice&logNo=223456789"
)

// fencePort builds a port whose editor is filled and whose settings layer is open, which is
// the only state the fence may be reached from (PUB-37).
func fencePort(t *testing.T, editor *fakeEditor) (*CDPPort, Snapshot) {
	t.Helper()
	if editor.observation == nil {
		editor.observation = healthyObservation()
	}
	if editor.permalink == "" {
		editor.permalink = fakePermalink
	}
	if editor.postView == nil {
		editor.postView = publishedPost()
	}
	port := applyPort(t, editor)
	snapshot, err := port.Observe(context.Background())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	return port, snapshot
}

// fenceManifest is the manifest the healthy editor's content belongs to: validateReadback
// checks the published URL against its expected account.
func fenceManifest() *postpilotv1.PublishManifest {
	return &postpilotv1.PublishManifest{ExpectedPlatformAccountId: "alice"}
}

// publishedPost is what the readback script yields for the healthy editor's own content, so
// an unmodified pair verifies and every tampering below is one field away from it.
func publishedPost() map[string]any {
	return map[string]any{
		"href":  fakePostView,
		"title": "제목입니다",
		"blocks": []map[string]any{
			{"kind": "text", "text": "본문 문단", "ordinal": 0, "caption": "", "uploaded": false, "source": ""},
			{"kind": "text", "text": "“인용”", "ordinal": 0, "caption": "", "uploaded": false, "source": ""},
			{"kind": "text", "text": "- 하나\n- 둘", "ordinal": 0, "caption": "", "uploaded": false, "source": ""},
			{"kind": "image", "text": "", "ordinal": 0, "caption": "캡션", "uploaded": true, "source": "a.jpg"},
		},
		"image_count":    1,
		"tags":           []string{"태그"},
		"category_name":  "식당",
		"editor_root":    false,
		"settings_layer": 0,
	}
}

func armed(t *testing.T, port *CDPPort, snapshot Snapshot) FinalControl {
	t.Helper()
	control, err := port.ArmFinal(context.Background(), snapshot.Token, []string{"발행"})
	if err != nil {
		t.Fatalf("arm: %v", err)
	}
	if control.Matches != 1 || control.OpaqueID == "" || control.AccessibleName != "발행" {
		t.Fatalf("armed control = %+v", control)
	}
	return control
}

// Arming is bound to the observation that armed it: PUB-22 says any interaction after the
// verification invalidates it, so a token that is no longer current cannot arm.
func TestArmFinalRefusesATokenThatIsNoLongerCurrent(t *testing.T) {
	port, snapshot := fencePort(t, &fakeEditor{})
	for name, token := range map[string]string{
		"a stale token":  snapshot.Token + "-stale",
		"an empty token": "",
		"another job's":  "snapshot-from-another-run",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := port.ArmFinal(context.Background(), token, []string{"발행"})
			var portErr PortError
			if !errors.As(err, &portErr) {
				t.Fatalf("err = %v, want a port error", err)
			}
			if got := port.armed.Token; got != "" {
				t.Fatalf("the port armed anyway: %q", got)
			}
		})
	}
}

// The fence accepts exactly one control, matched on its stable data-testid. A duplicate, a
// renamed one, an unversioned publish-like button, or a layer that only offers the scheduled
// choice all return a count that refuses to arm rather than an armed handle.
func TestArmFinalRefusesEveryControlButTheOneVersionedImmediateOne(t *testing.T) {
	cases := map[string]driverFinalControl{
		"no control":            {Matches: 0, LayerMatches: 1},
		"two controls":          {Matches: 2, LayerMatches: 1},
		"renamed":               {Matches: 1, LayerMatches: 1, Name: "게시", Versioned: 1, Actionable: true},
		"unversioned duplicate": {Matches: 1, LayerMatches: 1, Name: "발행", Versioned: 2, Actionable: true},
		"scheduled only":        {Matches: 0, LayerMatches: 1, Scheduled: 2},
		"not actionable":        {Matches: 1, LayerMatches: 1, Name: "발행", Versioned: 1, Actionable: false},
		"layer gone":            {Matches: 0, LayerMatches: 0},
		"two layers":            {Matches: 0, LayerMatches: 2},
	}
	for name, resolved := range cases {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{final: func([]string) driverFinalControl { return resolved }}
			port, snapshot := fencePort(t, editor)
			control, err := port.ArmFinal(context.Background(), snapshot.Token, []string{"발행"})
			if err == nil && control.OpaqueID != "" {
				t.Fatalf("armed %+v", control)
			}
			// Whatever it returned, the publisher's own check must also reject it.
			if control.Matches == 1 && control.AccessibleName == "발행" && control.OpaqueID != "" {
				t.Fatal("an unreviewed control produced an armed handle")
			}
			if port.armed.Token != "" {
				t.Fatal("the port kept an armed snapshot")
			}
		})
	}
}

// The scheduled choice sits in the same layer as the immediate control (verified live), so
// the driver has to count it. This pins that the reviewed function looks for it at all.
func TestTheFinalControlFunctionCountsTheScheduledChoice(t *testing.T) {
	for _, testID := range []string{"nowTimeRadioBtn", "preTimeRadioBtn", "seOnePublishBtn"} {
		if !strings.Contains(driverFinalControlFn, testID) {
			t.Fatalf("the reviewed final-control function no longer mentions %s", testID)
		}
	}
	if strings.Contains(driverFinalControlFn, ".click()") {
		t.Fatal("arming must observe, never activate")
	}
}

// Exactly one activation, and the latch is set BEFORE the call is issued: a crash inside it
// is the ambiguity PUB-15 turns into outcome_unknown, so a second attempt must be impossible
// even when the first one's outcome is unknown.
func TestActivateFinalSendsOneActivationAndLatchesBeforeIssuingIt(t *testing.T) {
	editor := &fakeEditor{}
	port, snapshot := fencePort(t, editor)
	control := armed(t, port, snapshot)
	if err := port.ActivateFinal(context.Background(), control); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !port.activated {
		t.Fatal("the port did not latch the activation")
	}
	if editor.activations != 1 {
		t.Fatalf("the fence sent %d activations, want exactly 1", editor.activations)
	}
	// A second activation, a re-arm and any body write are all refused by the PORT.
	var portErr PortError
	if err := port.ActivateFinal(context.Background(), control); !errors.As(err, &portErr) {
		t.Fatalf("a second activation returned %v", err)
	}
	if _, err := port.ArmFinal(context.Background(), snapshot.Token, []string{"발행"}); !errors.As(err, &portErr) {
		t.Fatalf("re-arming after activation returned %v", err)
	}
	for _, kind := range reviewedMutationKinds {
		if err := port.Apply(context.Background(), Mutation{Kind: kind, Text: "본문", Items: []string{"하나"}}); !errors.As(err, &portErr) {
			t.Fatalf("Apply(%s) after activation returned %v", kind, err)
		}
	}
	if editor.activations != 1 {
		t.Fatalf("activations grew to %d", editor.activations)
	}
}

// An activation handle that was not produced by this port's own arming cannot be used: the
// opaque id ties the control to the exact observation that armed it.
func TestActivateFinalRefusesAHandleItDidNotArm(t *testing.T) {
	for name, control := range map[string]FinalControl{
		"forged id":       {OpaqueID: "deadbeef", AccessibleName: "발행", Matches: 1},
		"renamed control": {OpaqueID: finalControlID("snapshot-1", "게시"), AccessibleName: "게시", Matches: 1},
		"no match":        {OpaqueID: finalControlID("snapshot-1", "발행"), AccessibleName: "발행", Matches: 0},
		"empty":           {},
	} {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{}
			port, snapshot := fencePort(t, editor)
			_ = armed(t, port, snapshot)
			var portErr PortError
			if err := port.ActivateFinal(context.Background(), control); !errors.As(err, &portErr) {
				t.Fatalf("err = %v, want a port error", err)
			}
			if editor.activations != 0 {
				t.Fatalf("%d activations were sent", editor.activations)
			}
		})
	}
}

// Nothing may be activated before arming, and nothing read back before activating.
func TestTheFenceRefusesEveryOutOfOrderStep(t *testing.T) {
	editor := &fakeEditor{}
	port, snapshot := fencePort(t, editor)
	var portErr PortError
	if err := port.ActivateFinal(context.Background(), FinalControl{OpaqueID: "x", AccessibleName: "발행", Matches: 1}); !errors.As(err, &portErr) {
		t.Fatalf("activation before arming returned %v", err)
	}
	if _, err := port.Readback(context.Background(), snapshot.TargetID); !errors.As(err, &portErr) {
		t.Fatalf("readback before activation returned %v", err)
	}
	if editor.activations != 0 {
		t.Fatalf("%d activations were sent", editor.activations)
	}
}

// The readback observes the post through the account's own post-view URL and reports the
// CANONICAL permalink — the two differ on purpose (PUB-20 r4).
func TestReadbackObservesThePostViewAndReportsThePermalink(t *testing.T) {
	editor := &fakeEditor{}
	port, snapshot := fencePort(t, editor)
	control := armed(t, port, snapshot)
	if err := port.ActivateFinal(context.Background(), control); err != nil {
		t.Fatalf("activate: %v", err)
	}
	readback, err := port.Readback(context.Background(), snapshot.TargetID)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if readback.PublishedURL != fakePermalink {
		t.Fatalf("published URL = %q, want the canonical permalink %q", readback.PublishedURL, fakePermalink)
	}
	if editor.navigatedTo != fakePostView {
		t.Fatalf("the readback navigated to %q, want the post view %q", editor.navigatedTo, fakePostView)
	}
	if readback.Snapshot.URL != fakePostView {
		t.Fatalf("the readback snapshot was taken at %q", readback.Snapshot.URL)
	}
	// Visibility and the category id are carried from the pre-fence snapshot, because a
	// published post renders neither; the category NAME is what the readback proves.
	if readback.Snapshot.Visibility != snapshot.Visibility {
		t.Fatalf("visibility = %+v, want the pre-fence %+v", readback.Snapshot.Visibility, snapshot.Visibility)
	}
	if readback.Snapshot.Category != snapshot.Category {
		t.Fatalf("category = %+v, want %+v", readback.Snapshot.Category, snapshot.Category)
	}
	// And the whole pair passes the publisher's own validation.
	if err := validateReadback(fenceManifest(), snapshot, readback); err != nil {
		t.Fatalf("the untampered pair was rejected: %v", err)
	}
}

// A readback whose post drifted from the frozen snapshot fails, one field at a time.
func TestReadbackRefusesAPostThatDriftedFromTheFrozenSnapshot(t *testing.T) {
	drifts := map[string]func(post map[string]any){
		"title": func(post map[string]any) { post["title"] = "다른 제목" },
		"block order": func(post map[string]any) {
			blocks := post["blocks"].([]map[string]any)
			blocks[0], blocks[1] = blocks[1], blocks[0]
		},
		"a block's text": func(post map[string]any) {
			post["blocks"].([]map[string]any)[0]["text"] = "다른 본문"
		},
		"an inserted block": func(post map[string]any) {
			post["blocks"] = append(post["blocks"].([]map[string]any), map[string]any{"kind": "text", "text": "끼어든 문단", "ordinal": 0, "caption": "", "uploaded": false, "source": ""})
		},
		"image count":  func(post map[string]any) { post["image_count"] = 2 },
		"a caption":    func(post map[string]any) { post["blocks"].([]map[string]any)[3]["caption"] = "다른 캡션" },
		"tag order":    func(post map[string]any) { post["tags"] = []string{"다른태그", "태그"} },
		"a lost tag":   func(post map[string]any) { post["tags"] = []string{} },
		"the category": func(post map[string]any) { post["category_name"] = "여행" },
	}
	for name, drift := range drifts {
		t.Run(name, func(t *testing.T) {
			post := publishedPost()
			drift(post)
			editor := &fakeEditor{postView: post}
			port, snapshot := fencePort(t, editor)
			control := armed(t, port, snapshot)
			if err := port.ActivateFinal(context.Background(), control); err != nil {
				t.Fatalf("activate: %v", err)
			}
			readback, err := port.Readback(context.Background(), snapshot.TargetID)
			if err != nil {
				return // refused at the port, which is also fail-closed
			}
			if err := validateReadback(fenceManifest(), snapshot, readback); err == nil {
				t.Fatalf("a post that drifted in %s was accepted: %+v", name, readback.Snapshot)
			}
		})
	}
}

// The readback refuses a page that is not the paired account's post, and one that is still
// the writer.
func TestReadbackRefusesAForeignOrNonPostPage(t *testing.T) {
	cases := map[string]func(editor *fakeEditor){
		"another account's permalink": func(editor *fakeEditor) { editor.permalink = "https://blog.naver.com/mallory/223456789" },
		"a non-numeric post id":       func(editor *fakeEditor) { editor.permalink = "https://blog.naver.com/alice/not-a-number" },
		"a foreign host":              func(editor *fakeEditor) { editor.permalink = "https://blog.example.com/alice/223456789" },
		"the editor, not a post": func(editor *fakeEditor) {
			post := publishedPost()
			post["editor_root"] = true
			editor.postView = post
		},
		"a settings layer on the post": func(editor *fakeEditor) {
			post := publishedPost()
			post["settings_layer"] = 1
			editor.postView = post
		},
		"a post view for another blog": func(editor *fakeEditor) {
			post := publishedPost()
			post["href"] = "https://blog.naver.com/PostView.naver?blogId=mallory&logNo=223456789"
			editor.postView = post
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			editor := &fakeEditor{}
			port, snapshot := fencePort(t, editor)
			arrange(editor)
			control := armed(t, port, snapshot)
			if err := port.ActivateFinal(context.Background(), control); err != nil {
				t.Fatalf("activate: %v", err)
			}
			readback, err := port.Readback(context.Background(), snapshot.TargetID)
			if err != nil {
				return
			}
			if err := validateReadback(fenceManifest(), snapshot, readback); err == nil {
				t.Fatalf("%s was accepted: %+v", name, readback)
			}
		})
	}
}

// A readback for a target other than the bound one is refused before anything is read: the
// evidence would belong to a different page (PUB-20).
func TestReadbackRefusesADifferentBoundTarget(t *testing.T) {
	editor := &fakeEditor{}
	port, snapshot := fencePort(t, editor)
	control := armed(t, port, snapshot)
	if err := port.ActivateFinal(context.Background(), control); err != nil {
		t.Fatalf("activate: %v", err)
	}
	var portErr PortError
	if _, err := port.Readback(context.Background(), "page-2"); !errors.As(err, &portErr) {
		t.Fatalf("err = %v, want a port error", err)
	}
	if editor.navigatedTo != "" {
		t.Fatalf("it navigated anyway: %q", editor.navigatedTo)
	}
}

// The permalink the editor never reaches is an ambiguous outcome, not a retryable failure:
// the wait ends and the port refuses rather than activating anything again.
func TestReadbackRefusesWhenThePermalinkNeverArrives(t *testing.T) {
	editor := &fakeEditor{permalink: fakeWriterURL}
	port, snapshot := fencePort(t, editor)
	port.settle = 300 * time.Millisecond
	control := armed(t, port, snapshot)
	if err := port.ActivateFinal(context.Background(), control); err != nil {
		t.Fatalf("activate: %v", err)
	}
	var portErr PortError
	if _, err := port.Readback(context.Background(), snapshot.TargetID); !errors.As(err, &portErr) {
		t.Fatalf("err = %v, want a port error", err)
	}
	if editor.activations != 1 {
		t.Fatalf("%d activations were sent", editor.activations)
	}
}

// An eight-photo post whose images Naver grouped into a strip must still read back eight, or
// the fence would reject a correct publication.
func TestReadbackCountsImagesInsideAStrip(t *testing.T) {
	if !strings.Contains(readbackObservationScript, "se-imageStrip") {
		t.Fatal("the readback no longer looks inside an image strip")
	}
	if !strings.Contains(readbackObservationScript, "se-module-image") {
		t.Fatal("the readback does not walk a strip's member images")
	}
	// The observation is what counts them, so the count crossing into the snapshot is what
	// this pins: a post reporting eight images yields eight.
	post := publishedPost()
	blocks := post["blocks"].([]map[string]any)
	for ordinal := 1; ordinal < 8; ordinal++ {
		blocks = append(blocks, map[string]any{"kind": "image", "text": "", "ordinal": ordinal, "caption": "", "uploaded": true, "source": "s.jpg"})
	}
	post["blocks"], post["image_count"] = blocks, 8
	editor := &fakeEditor{postView: post}
	port, snapshot := fencePort(t, editor)
	control := armed(t, port, snapshot)
	if err := port.ActivateFinal(context.Background(), control); err != nil {
		t.Fatalf("activate: %v", err)
	}
	readback, err := port.Readback(context.Background(), snapshot.TargetID)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if readback.Snapshot.ImageCount != 8 {
		t.Fatalf("image count = %d, want 8", readback.Snapshot.ImageCount)
	}
	images := 0
	for _, block := range readback.Snapshot.Body {
		if block.Kind == SemanticImage {
			images++
		}
	}
	if images != 8 {
		t.Fatalf("%d image blocks reached the projection, want 8", images)
	}
}

// The readback's projection must be the editor's projection, or the fence compares two
// different vocabularies and no correct post ever verifies.
func TestTheReadbackProjectsTheBodyLikeTheEditorDoes(t *testing.T) {
	for _, rule := range []string{"se-sectionTitle", "se-quotation", "se-text-list-item", "se-module-text.se-caption", "__se-hash-tag", "category_title"} {
		if !strings.Contains(readbackObservationScript, rule) {
			t.Fatalf("the readback does not read %s", rule)
		}
	}
	if !strings.Contains(readbackObservationScript, `'“' + textOf(quoted) + '”'`) {
		t.Fatal("the readback no longer wraps a quotation the way the editor projection does")
	}
	if !strings.Contains(readbackObservationScript, "'- '") {
		t.Fatal("the readback no longer projects a list as the editor does")
	}
}

// The readback is a separate observation path on purpose: Observe requires the writer's URL,
// and relaxing it would let a post be mistaken for an editor.
func TestObserveStillRequiresTheWriterURL(t *testing.T) {
	if _, err := writerAccount(fakePostView); err == nil {
		t.Fatal("the writer predicate accepted a post-view URL")
	}
	if _, err := postViewAccount(fakeWriterURL); err == nil {
		t.Fatal("the post-view predicate accepted the writer URL")
	}
	account, err := postViewAccount(fakePostView)
	if err != nil || account != "alice" {
		t.Fatalf("post-view account = %q, %v", account, err)
	}
}

// Every reviewed driver action on the fence path leaves no coordinate behind.
func TestTheFencePathRetainsNoGeometry(t *testing.T) {
	editor := &fakeEditor{}
	port, snapshot := fencePort(t, editor)
	control := armed(t, port, snapshot)
	if control.OpaqueID == "" {
		t.Fatal("no handle")
	}
	// The handle is derived from the control's identity and the observation that armed it,
	// never from where it sat: moving the control does not change the handle, and a
	// different observation does.
	moved := &fakeEditor{final: func(allowed []string) driverFinalControl {
		return driverFinalControl{Matches: 1, LayerMatches: 1, Name: allowed[0], Versioned: 1, Actionable: true, X: 777, Y: 888}
	}}
	movedPort, movedSnapshot := fencePort(t, moved)
	movedControl := armed(t, movedPort, movedSnapshot)
	if movedControl.OpaqueID != finalControlID(movedSnapshot.Token, "발행") {
		t.Fatal("the handle is not derived from the arming observation and the control's name")
	}
	if finalControlID(movedSnapshot.Token, "발행") == finalControlID(movedSnapshot.Token+"x", "발행") {
		t.Fatal("the handle does not change with the observation that armed it")
	}
	if err := port.ActivateFinal(context.Background(), control); err != nil {
		t.Fatalf("activate: %v", err)
	}
	readback, err := port.Readback(context.Background(), snapshot.TargetID)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if len(readback.Snapshot.LocatorMatches) != 0 {
		t.Fatalf("the readback snapshot carries locator counts: %+v", readback.Snapshot.LocatorMatches)
	}
	// The final control is resolved twice — once to arm, once to act — and never cached.
	resolves := 0
	for _, action := range editor.recorded() {
		if strings.HasPrefix(action, "final:") {
			resolves++
		}
	}
	if resolves != 2 {
		t.Fatalf("the final control was resolved %d times, want one per step", resolves)
	}
	if !slices.Contains(editor.recorded(), "navigate:"+fakePostView) {
		t.Fatalf("the readback did not navigate to the post view: %v", editor.recorded())
	}
}

// PUB-15's two sides, simulated against the real port rather than a fake publisher: a
// process loss BEFORE the fence leaves nothing activated, so the job requeues safely, and
// one AFTER the activation leaves exactly one activation behind and no retry anywhere.
func TestProcessLossOnEitherSideOfTheFenceLeavesTheRightNumberOfActivations(t *testing.T) {
	t.Run("before the fence", func(t *testing.T) {
		// The browser disappears while the settings are still being filled: the port is
		// dropped without arming, and nothing was ever activated.
		editor := &fakeEditor{}
		port, snapshot := fencePort(t, editor)
		port.evaluateFailsFrom(editor)
		if _, err := port.ArmFinal(context.Background(), snapshot.Token, []string{"발행"}); err == nil {
			t.Fatal("arming succeeded against a dead page")
		}
		if editor.activations != 0 {
			t.Fatalf("%d activations were sent before the fence", editor.activations)
		}
	})

	t.Run("after the activation", func(t *testing.T) {
		editor := &fakeEditor{}
		port, snapshot := fencePort(t, editor)
		control := armed(t, port, snapshot)
		if err := port.ActivateFinal(context.Background(), control); err != nil {
			t.Fatalf("activate: %v", err)
		}
		// The page dies before the permalink can be read. The outcome is unknown by
		// design: the readback refuses, and nothing tries again.
		port.evaluateFailsFrom(editor)
		if _, err := port.Readback(context.Background(), snapshot.TargetID); err == nil {
			t.Fatal("the readback succeeded against a dead page")
		}
		if editor.activations != 1 {
			t.Fatalf("%d activations survive the loss, want exactly 1", editor.activations)
		}
		// And the port still refuses to do anything more, ambiguity included.
		var portErr PortError
		if err := port.ActivateFinal(context.Background(), control); !errors.As(err, &portErr) {
			t.Fatalf("a retry after the ambiguous outcome returned %v", err)
		}
		if editor.activations != 1 {
			t.Fatalf("%d activations after the retry attempt", editor.activations)
		}
	})
}

// The publisher's own fence order is asserted elsewhere with a fake port; this pins that the
// REAL port is what satisfies CommitPort, so the two can never drift apart.
func TestTheCDPPortIsTheCommitPortThePublisherRequires(t *testing.T) {
	editor := &fakeEditor{}
	port, _ := fencePort(t, editor)
	if _, ok := any(port).(CommitPort); !ok {
		t.Fatal("CDPPort no longer satisfies CommitPort")
	}
	if _, ok := any(port).(Port); !ok {
		t.Fatal("CDPPort no longer satisfies Port")
	}
}
