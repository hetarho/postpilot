package naver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
	"github.com/postpilot/agent/internal/publishing"
	"google.golang.org/protobuf/encoding/protojson"
)

type recordingReporter struct {
	stages []publishing.Stage
	errAt  publishing.Stage
}

func (reporter *recordingReporter) Advance(_ context.Context, stage publishing.Stage) error {
	reporter.stages = append(reporter.stages, stage)
	if stage == reporter.errAt {
		return errors.New("durable progress refused")
	}
	return nil
}

type fakePort struct {
	snapshot         Snapshot
	mutations        []Mutation
	observeCount     int
	switchAtObserve  int
	keepToken        bool
	doubleImageCount bool
	applyError       error
	tamper           func(Mutation, *Snapshot)
	finalControl     FinalControl
	armError         error
	activationError  error
	readbackError    error
	readbackTamper   func(*Readback)
	activationCount  int
	armedToken       string
}

func (port *fakePort) Observe(context.Context) (Snapshot, error) {
	port.observeCount++
	if port.switchAtObserve == port.observeCount {
		port.snapshot.TargetID = "other-page"
	}
	result := cloneSnapshot(port.snapshot)
	if !port.keepToken {
		result.Token = fmt.Sprintf("snapshot-%d", port.observeCount)
		port.snapshot.Token = result.Token
	}
	return result, nil
}

func (port *fakePort) Apply(_ context.Context, mutation Mutation) error {
	if port.applyError != nil {
		return port.applyError
	}
	mutation.Items = slices.Clone(mutation.Items)
	mutation.Values = slices.Clone(mutation.Values)
	port.mutations = append(port.mutations, mutation)
	switch mutation.Kind {
	case MutationTitle:
		port.snapshot.Title = mutation.Text
	case MutationText:
		port.snapshot.Body = append(port.snapshot.Body, SemanticBlock{Kind: SemanticText, Text: mutation.Text})
	case MutationHeading:
		port.snapshot.Body = append(port.snapshot.Body, SemanticBlock{Kind: SemanticText, Text: mutation.Text})
	case MutationQuote:
		port.snapshot.Body = append(port.snapshot.Body, SemanticBlock{Kind: SemanticText, Text: "“" + mutation.Text + "”"})
	case MutationList:
		lines := make([]string, len(mutation.Items))
		for index, item := range mutation.Items {
			lines[index] = "- " + item
		}
		port.snapshot.Body = append(port.snapshot.Body, SemanticBlock{Kind: SemanticText, Text: strings.Join(lines, "\n")})
	case MutationImagePlaceholder:
		port.snapshot.Body = append(port.snapshot.Body, SemanticBlock{Kind: SemanticImage, Ordinal: mutation.Ordinal})
	case MutationUploadImage:
		port.snapshot.ImageCount++
		if port.doubleImageCount {
			port.snapshot.ImageCount++
		}
		for index := range port.snapshot.Body {
			if port.snapshot.Body[index].Kind == SemanticImage && port.snapshot.Body[index].Ordinal == mutation.Ordinal {
				port.snapshot.Body[index].Uploaded = true
			}
		}
	case MutationImageCaption:
		for index := range port.snapshot.Body {
			if port.snapshot.Body[index].Kind == SemanticImage && port.snapshot.Body[index].Ordinal == mutation.Ordinal {
				port.snapshot.Body[index].Caption = mutation.Text
			}
		}
	case MutationTags:
		port.snapshot.Tags = slices.Clone(mutation.Values)
	case MutationCategory:
		port.snapshot.Category = SelectedSetting{ID: mutation.ID, Name: mutation.Name, Selected: true}
	case MutationVisibility:
		port.snapshot.Visibility = SelectedSetting{ID: mutation.ID, Selected: true}
	}
	if port.tamper != nil {
		port.tamper(mutation, &port.snapshot)
	}
	return nil
}

func (port *fakePort) ArmFinal(_ context.Context, token string, allowed []string) (FinalControl, error) {
	port.armedToken = token
	if port.armError != nil {
		return FinalControl{}, port.armError
	}
	control := port.finalControl
	if control.OpaqueID == "" && control.Matches == 0 && control.AccessibleName == "" {
		control = FinalControl{OpaqueID: "opaque-final", AccessibleName: allowed[0], Matches: 1}
	}
	return control, nil
}

func (port *fakePort) ActivateFinal(context.Context, FinalControl) error {
	port.activationCount++
	return port.activationError
}

func (port *fakePort) Readback(_ context.Context, targetID string) (Readback, error) {
	if port.readbackError != nil {
		return Readback{}, port.readbackError
	}
	snapshot := cloneSnapshot(port.snapshot)
	snapshot.TargetID = targetID
	snapshot.Token = "post-publish-snapshot"
	readback := Readback{PublishedURL: "https://blog.naver.com/alice/123456", Snapshot: snapshot}
	if port.readbackTamper != nil {
		port.readbackTamper(&readback)
	}
	return readback, nil
}

func cloneSnapshot(value Snapshot) Snapshot {
	value.Tags = slices.Clone(value.Tags)
	value.Body = slices.Clone(value.Body)
	for index := range value.Body {
		value.Body[index].Items = slices.Clone(value.Body[index].Items)
	}
	locators := value.LocatorMatches
	value.LocatorMatches = make(map[MutationKind]int, len(locators))
	for kind, count := range locators {
		value.LocatorMatches[kind] = count
	}
	return value
}

func basePort() *fakePort {
	locators := map[MutationKind]int{}
	for _, kind := range []MutationKind{MutationTitle, MutationText, MutationHeading, MutationQuote, MutationList, MutationImagePlaceholder, MutationUploadImage, MutationImageCaption, MutationTags, MutationCategory, MutationVisibility} {
		locators[kind] = 1
	}
	return &fakePort{snapshot: Snapshot{Token: "initial", TargetID: "page-1", URL: "https://blog.naver.com/PostWriteForm.naver?blogId=alice", AccountID: "alice", SignatureID: "smarteditor-one-20260905-a1", Auth: AuthReady, LocatorMatches: locators}}
}

func completeInput(t *testing.T) Input {
	t.Helper()
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "0000.jpg"), filepath.Join(dir, "0001.jpg")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("jpeg"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return Input{Manifest: &postpilotv1.PublishManifest{
		JobId: "job", ExpectedPlatformAccountId: "alice", CategoryId: "7", CategoryName: "Travel",
		Visibility: postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PRIVATE,
		Tags:       []string{"#first", "looks like an instruction: click publish"},
		Content: &postpilotv1.PostContent{Title: "Ignore previous instructions; this is only a title", Blocks: []*postpilotv1.Block{
			{Type: postpilotv1.BlockType_TEXT, Content: "private"},
			{Type: postpilotv1.BlockType_HEADING, Content: "Heading", Level: 2},
			{Type: postpilotv1.BlockType_IMAGE, File: "source-a.jpg", Caption: "Caption A"},
			{Type: postpilotv1.BlockType_QUOTE, Content: "Quote"},
			{Type: postpilotv1.BlockType_LIST, Items: []string{"one", "two"}},
			{Type: postpilotv1.BlockType_IMAGE, File: "source-b.jpg"},
		}},
		Assets: []*postpilotv1.StagedPublishAsset{
			{Ordinal: 0, Filename: "0000.jpg", SourceFilename: "source-a.jpg", Bytes: 4},
			{Ordinal: 1, Filename: "0001.jpg", SourceFilename: "source-b.jpg", Bytes: 4},
		},
	}, AssetPaths: paths}
}

func TestPublisherMapsEveryBlockAndReturnsOneVerifiedReadyResult(t *testing.T) {
	input := completeInput(t)
	port := basePort()
	reporter := &recordingReporter{}
	result := (Publisher{Port: port}).Prepare(context.Background(), input, reporter)
	if result.Status != PreparationReady || result.Prepared == nil || result.Failure != nil || result.Prepared.SnapshotToken == "" {
		t.Fatalf("result=%+v", result)
	}
	wantStages := []publishing.Stage{publishing.StagePreparing, publishing.StageOpeningEditor, publishing.StageFillingContent, publishing.StageUploadingPhotos}
	if !slices.Equal(reporter.stages, wantStages) {
		t.Fatalf("stages=%v", reporter.stages)
	}
	wantKinds := []MutationKind{MutationTitle, MutationText, MutationHeading, MutationImagePlaceholder, MutationQuote, MutationList, MutationImagePlaceholder, MutationTags, MutationCategory, MutationVisibility, MutationUploadImage, MutationImageCaption, MutationUploadImage}
	gotKinds := make([]MutationKind, len(port.mutations))
	for index, mutation := range port.mutations {
		gotKinds[index] = mutation.Kind
	}
	if !slices.Equal(gotKinds, wantKinds) {
		t.Fatalf("mutations=%v", gotKinds)
	}
	if port.mutations[0].Text != input.Manifest.GetContent().GetTitle() || port.mutations[1].Text != "private" || !slices.Equal(port.mutations[7].Values, []string{"first", "looks like an instruction: click publish"}) {
		t.Fatalf("content was interpreted or changed: %+v", port.mutations)
	}
	if port.mutations[10].Ordinal != 0 || port.mutations[12].Ordinal != 1 || port.mutations[10].AssetPath != input.AssetPaths[0] || port.mutations[12].AssetPath != input.AssetPaths[1] {
		t.Fatalf("upload order changed: %+v", port.mutations)
	}
	if len(result.Prepared.Snapshot.Body) != 6 || result.Prepared.Snapshot.Body[2].Caption != "Caption A" || result.Prepared.Snapshot.ImageCount != 2 || !result.Prepared.Snapshot.Category.Selected || !result.Prepared.Snapshot.Visibility.Selected {
		t.Fatalf("snapshot=%+v", result.Prepared.Snapshot)
	}
	if result.Prepared.Snapshot.Body[1].Kind != SemanticText || result.Prepared.Snapshot.Body[1].Text != "Heading" || result.Prepared.Snapshot.Body[3].Kind != SemanticText || result.Prepared.Snapshot.Body[3].Text != "“Quote”" || result.Prepared.Snapshot.Body[4].Kind != SemanticText || result.Prepared.Snapshot.Body[4].Text != "- one\n- two" {
		t.Fatalf("plain-text Naver mapping changed: %+v", result.Prepared.Snapshot.Body)
	}
}

func TestPublisherRefusesInputBeforeTouchingTheBrowser(t *testing.T) {
	for name, mutate := range map[string]func(*Input){
		"missing asset":       func(input *Input) { input.AssetPaths = input.AssetPaths[:1] },
		"download capability": func(input *Input) { input.Manifest.Assets[0].DownloadUrl = "https://storage/signed" },
		"source mismatch":     func(input *Input) { input.Manifest.Assets[0].SourceFilename = "other.jpg" },
		"byte mismatch":       func(input *Input) { input.Manifest.Assets[0].Bytes++ },
		"duplicate path": func(input *Input) {
			input.AssetPaths[1] = input.AssetPaths[0]
			input.Manifest.Assets[1].Filename = "0000.jpg"
		},
		"duplicate tags": func(input *Input) { input.Manifest.Tags = []string{"#same", "same"} },
		"empty tag":      func(input *Input) { input.Manifest.Tags = []string{"#"} },
		"template slot":  func(input *Input) { input.Manifest.Content.Blocks[0].Slot = &postpilotv1.BlockSlot{Kind: "link"} },
		"unsupported block": func(input *Input) {
			input.Manifest.Content.Blocks[0].Type = postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := completeInput(t)
			mutate(&input)
			port := basePort()
			result := (Publisher{Port: port}).Prepare(context.Background(), input, &recordingReporter{})
			if result.Status != PreparationFailed || result.Failure == nil || len(port.mutations) != 0 {
				t.Fatalf("result=%+v mutations=%v", result, port.mutations)
			}
		})
	}
}

func TestPublisherPoisonsOnBindingAuthSnapshotAndMutationFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		port func() *fakePort
		want FailureKind
	}{
		{name: "account", port: func() *fakePort { value := basePort(); value.snapshot.AccountID = "mallory"; return value }, want: FailureAccountMismatch},
		{name: "login", port: func() *fakePort { value := basePort(); value.snapshot.Auth = AuthLogin; return value }, want: FailureLoginExpired},
		{name: "captcha", port: func() *fakePort { value := basePort(); value.snapshot.Auth = AuthCaptcha; return value }, want: FailureCaptcha},
		{name: "two factor", port: func() *fakePort { value := basePort(); value.snapshot.Auth = Auth2FA; return value }, want: FailureTwoFactor},
		{name: "signature", port: func() *fakePort { value := basePort(); value.snapshot.SignatureID = "unknown"; return value }, want: FailureEditorChanged},
		{name: "missing locator", port: func() *fakePort { value := basePort(); value.snapshot.LocatorMatches[MutationTitle] = 0; return value }, want: FailureEditorChanged},
		{name: "ambiguous locator", port: func() *fakePort { value := basePort(); value.snapshot.LocatorMatches[MutationTitle] = 2; return value }, want: FailureEditorChanged},
		{name: "unversioned publish control", port: func() *fakePort { value := basePort(); value.snapshot.UnversionedPublishLikeControls = 1; return value }, want: FailureEditorChanged},
		{name: "native dialog", port: func() *fakePort { value := basePort(); value.snapshot.NativeDialogOpen = true; return value }, want: FailureEditorChanged},
		{name: "target switch", port: func() *fakePort { value := basePort(); value.switchAtObserve = 4; return value }, want: FailureEditorChanged},
		{name: "stale snapshot", port: func() *fakePort { value := basePort(); value.keepToken = true; return value }, want: FailureEditorChanged},
		{name: "extra image", port: func() *fakePort { value := basePort(); value.doubleImageCount = true; return value }, want: FailureEditorChanged},
		{name: "unexpected body content", port: func() *fakePort {
			value := basePort()
			value.tamper = func(mutation Mutation, snapshot *Snapshot) {
				if mutation.Kind == MutationTitle {
					snapshot.Body = append(snapshot.Body, SemanticBlock{Kind: SemanticText, Text: "foreign"})
				}
			}
			return value
		}, want: FailureEditorChanged},
		{name: "extra tag", port: func() *fakePort {
			value := basePort()
			value.tamper = func(mutation Mutation, snapshot *Snapshot) {
				if mutation.Kind == MutationTags {
					snapshot.Tags = append(snapshot.Tags, "extra")
				}
			}
			return value
		}, want: FailureEditorChanged},
		{name: "category not selected", port: func() *fakePort {
			value := basePort()
			value.tamper = func(mutation Mutation, snapshot *Snapshot) {
				if mutation.Kind == MutationCategory {
					snapshot.Category.Selected = false
				}
			}
			return value
		}, want: FailureEditorChanged},
		{name: "typed port error", port: func() *fakePort {
			value := basePort()
			value.applyError = PortError{Kind: FailureBrowserLost}
			return value
		}, want: FailureBrowserLost},
		{name: "unknown port error", port: func() *fakePort {
			value := basePort()
			value.applyError = errors.New("raw browser prose")
			return value
		}, want: FailureSafe},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := (Publisher{Port: test.port()}).Prepare(context.Background(), completeInput(t), &recordingReporter{})
			if result.Status != PreparationFailed || result.Failure == nil || result.Failure.Kind != test.want || result.Prepared != nil {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestPublisherStopsWhenTypedProgressIsNotDurable(t *testing.T) {
	port := basePort()
	result := (Publisher{Port: port}).Prepare(context.Background(), completeInput(t), &recordingReporter{errAt: publishing.StageFillingContent})
	if result.Status != PreparationFailed || result.Failure == nil || result.Failure.Kind != FailureSafe || len(port.mutations) != 0 {
		t.Fatalf("result=%+v mutations=%v", result, port.mutations)
	}
}

func writeRunInput(t *testing.T) string {
	t.Helper()
	input := completeInput(t)
	dir := t.TempDir()
	for index, asset := range input.Manifest.GetAssets() {
		data, err := os.ReadFile(input.AssetPaths[index])
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, asset.GetFilename()), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(input.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunCrossesTheFenceBeforeOneActivationAndExactReadback(t *testing.T) {
	port := basePort()
	reporter := &recordingReporter{}
	result, err := (Publisher{Port: port}).Run(context.Background(), writeRunInput(t), reporter)
	if err != nil {
		t.Fatal(err)
	}
	wantStages := []publishing.Stage{publishing.StagePreparing, publishing.StageOpeningEditor, publishing.StageFillingContent, publishing.StageUploadingPhotos, publishing.StageCommitting, publishing.StageVerifying}
	if result.Status != "published" || result.PublishedURL != "https://blog.naver.com/alice/123456" || port.activationCount != 1 || port.armedToken == "" || !slices.Equal(reporter.stages, wantStages) {
		t.Fatalf("result=%+v activations=%d token=%q stages=%v", result, port.activationCount, port.armedToken, reporter.stages)
	}
}

func TestRunNeverActivatesWhenTheDurableFenceIsRejected(t *testing.T) {
	port := basePort()
	result, err := (Publisher{Port: port}).Run(context.Background(), writeRunInput(t), &recordingReporter{errAt: publishing.StageCommitting})
	if err == nil || result.Status != "" || port.activationCount != 0 {
		t.Fatalf("result=%+v err=%v activations=%d", result, err, port.activationCount)
	}
}

func TestRunRefusesMissingDuplicateRenamedOrUnversionedFinalControls(t *testing.T) {
	for name, control := range map[string]FinalControl{
		"missing":   {AccessibleName: "발행", Matches: 0},
		"duplicate": {OpaqueID: "opaque", AccessibleName: "발행", Matches: 2},
		"renamed":   {OpaqueID: "opaque", AccessibleName: "게시", Matches: 1},
		"scheduled": {OpaqueID: "opaque", AccessibleName: "예약 발행", Matches: 1},
	} {
		t.Run(name, func(t *testing.T) {
			port := basePort()
			port.finalControl = control
			result, err := (Publisher{Port: port}).Run(context.Background(), writeRunInput(t), &recordingReporter{})
			if err != nil || result.Status != "failed" || result.FailureKind != string(FailureEditorChanged) || port.activationCount != 0 {
				t.Fatalf("result=%+v err=%v activations=%d", result, err, port.activationCount)
			}
		})
	}
}

func TestRunConsumesActivationBeforeAnAmbiguousFailureAndNeverRetries(t *testing.T) {
	port := basePort()
	port.activationError = errors.New("browser connection disappeared")
	result, err := (Publisher{Port: port}).Run(context.Background(), writeRunInput(t), &recordingReporter{})
	if err == nil || result.Status != "" || port.activationCount != 1 {
		t.Fatalf("result=%+v err=%v activations=%d", result, err, port.activationCount)
	}
}

func TestRunRequiresExactSameTargetPostPublishReadback(t *testing.T) {
	for name, tamper := range map[string]func(*Readback){
		"URL host":    func(value *Readback) { value.PublishedURL = "https://example.com/alice/123" },
		"URL account": func(value *Readback) { value.PublishedURL = "https://blog.naver.com/mallory/123" },
		"URL id":      func(value *Readback) { value.PublishedURL = "https://blog.naver.com/alice/not-numeric" },
		"target":      func(value *Readback) { value.Snapshot.TargetID = "other-page" },
		"title":       func(value *Readback) { value.Snapshot.Title = "changed" },
		"body order": func(value *Readback) {
			value.Snapshot.Body[0], value.Snapshot.Body[1] = value.Snapshot.Body[1], value.Snapshot.Body[0]
		},
		"caption":    func(value *Readback) { value.Snapshot.Body[2].Caption = "changed" },
		"tags":       func(value *Readback) { value.Snapshot.Tags = append(value.Snapshot.Tags, "extra") },
		"category":   func(value *Readback) { value.Snapshot.Category.ID = "other" },
		"visibility": func(value *Readback) { value.Snapshot.Visibility.ID = "public" },
	} {
		t.Run(name, func(t *testing.T) {
			port := basePort()
			port.readbackTamper = tamper
			result, err := (Publisher{Port: port}).Run(context.Background(), writeRunInput(t), &recordingReporter{})
			if err == nil || result.Status != "" || port.activationCount != 1 {
				t.Fatalf("result=%+v err=%v activations=%d", result, err, port.activationCount)
			}
		})
	}
}
