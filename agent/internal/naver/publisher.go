package naver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
	"github.com/postpilot/agent/internal/publishing"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type PreparationStatus string

const (
	PreparationReady  PreparationStatus = "ready_to_arm"
	PreparationFailed PreparationStatus = "failed"
)

type FailureKind string

const (
	FailureSafe            FailureKind = "safe"
	FailureLoginExpired    FailureKind = "login_expired"
	FailureCaptcha         FailureKind = "captcha"
	FailureTwoFactor       FailureKind = "two_factor"
	FailureAccountMismatch FailureKind = "account_mismatch"
	FailureBrowserLost     FailureKind = "browser_lost"
	FailureEditorChanged   FailureKind = "editor_changed"
	FailureAssetMissing    FailureKind = "asset_missing"
)

type PreparationResult struct {
	Status   PreparationStatus
	Prepared *Prepared
	Failure  *Failure
}

type Failure struct {
	Kind FailureKind
}

// Prepared is the immutable hand-off to T006's commit fence. SnapshotToken identifies
// the full pre-fence observation; any later interaction must invalidate it.
type Prepared struct {
	SnapshotToken string
	Snapshot      Snapshot
}

type Input struct {
	Manifest   *postpilotv1.PublishManifest
	AssetPaths []string
}

type AuthState string

const (
	AuthReady   AuthState = "ready"
	AuthLogin   AuthState = "login"
	AuthCaptcha AuthState = "captcha"
	Auth2FA     AuthState = "two_factor"
)

type SemanticKind string

const (
	SemanticText    SemanticKind = "text"
	SemanticHeading SemanticKind = "heading"
	SemanticQuote   SemanticKind = "quote"
	SemanticList    SemanticKind = "list"
	SemanticImage   SemanticKind = "image"
)

type SemanticBlock struct {
	Kind     SemanticKind
	Text     string
	Level    int32
	Items    []string
	Ordinal  int
	Caption  string
	Uploaded bool
}

type SelectedSetting struct {
	ID       string
	Name     string
	Selected bool
}

type Snapshot struct {
	Token       string
	TargetID    string
	URL         string
	AccountID   string
	SignatureID string
	Auth        AuthState
	Title       string
	Body        []SemanticBlock
	ImageCount  int
	Tags        []string
	Category    SelectedSetting
	Visibility  SelectedSetting
	// Rebuilt from the current DOM and full accessibility tree. The publisher
	// requires one exact reviewed semantic match for its next typed mutation.
	LocatorMatches                 map[MutationKind]int
	UnversionedPublishLikeControls int
	NativeDialogOpen               bool
}

type MutationKind string

const (
	MutationTitle            MutationKind = "title"
	MutationText             MutationKind = "text"
	MutationHeading          MutationKind = "heading"
	MutationQuote            MutationKind = "quote"
	MutationList             MutationKind = "list"
	MutationImagePlaceholder MutationKind = "image_placeholder"
	MutationUploadImage      MutationKind = "upload_image"
	MutationImageCaption     MutationKind = "image_caption"
	MutationTags             MutationKind = "tags"
	MutationCategory         MutationKind = "category"
	MutationVisibility       MutationKind = "visibility"
)

// Mutation is a closed data-only command. Text, list items and setting ids remain inert;
// there is no selector, script, coordinate, key sequence or native-dialog operation.
type Mutation struct {
	Kind      MutationKind
	Text      string
	Level     int32
	Items     []string
	Ordinal   int
	AssetPath string
	Values    []string
	ID        string
	Name      string
}

// Port is deliberately high-level and closed. Its implementation owns the reviewed
// SmartEditor locator set and must bind every Observe/Apply to one poisoned-on-change CDP
// target. Apply may never accept caller-supplied selectors or executable instructions.
type Port interface {
	Observe(context.Context) (Snapshot, error)
	Apply(context.Context, Mutation) error
}

type FinalControl struct {
	OpaqueID       string
	AccessibleName string
	Matches        int
}

type Readback struct {
	PublishedURL string
	Snapshot     Snapshot
}

// CommitPort is the only extension allowed after preparation. ArmFinal is an observation
// bound to Prepared.SnapshotToken; ActivateFinal is one non-retrying low-level activation;
// Readback observes the same target after navigation. There is no other activation method.
type CommitPort interface {
	Port
	ArmFinal(context.Context, string, []string) (FinalControl, error)
	ActivateFinal(context.Context, FinalControl) error
	Readback(context.Context, string) (Readback, error)
}

type Publisher struct {
	Port Port
}

var _ publishing.Publisher = Publisher{}

var numericPostID = regexp.MustCompile(`^[1-9][0-9]*$`)

// Run connects the deterministic preparation to publishing.Executor. It reads only the
// owner-only local manifest and its enumerated files, crosses the synchronous durable fence,
// consumes the one activation authorization before calling the port, and requires exact
// same-target readback before returning the sole published terminal result.
func (p Publisher) Run(ctx context.Context, dir string, reporter publishing.Reporter) (publishing.Result, error) {
	input, failure := loadInput(dir)
	if failure != "" {
		return publishing.Result{Status: "failed", FailureKind: string(failure)}, nil
	}
	prepared := p.Prepare(ctx, input, reporter)
	if prepared.Status != PreparationReady || prepared.Prepared == nil {
		kind := FailureSafe
		if prepared.Failure != nil {
			kind = prepared.Failure.Kind
		}
		return publishing.Result{Status: "failed", FailureKind: string(kind)}, nil
	}
	port, ok := p.Port.(CommitPort)
	if !ok {
		return publishing.Result{Status: "failed", FailureKind: string(FailureSafe)}, nil
	}
	compatibility, err := Manifest()
	if err != nil {
		return publishing.Result{Status: "failed", FailureKind: string(FailureEditorChanged)}, nil
	}
	control, err := port.ArmFinal(ctx, prepared.Prepared.SnapshotToken, slices.Clone(compatibility.FinalControlAccessibleNames))
	if err != nil {
		return publishing.Result{Status: "failed", FailureKind: string(classifyPortError(err))}, nil
	}
	if control.OpaqueID == "" || control.Matches != 1 || !slices.Contains(compatibility.FinalControlAccessibleNames, control.AccessibleName) {
		return publishing.Result{Status: "failed", FailureKind: string(FailureEditorChanged)}, nil
	}
	// The synchronous return is the authorization boundary. A timeout or refusal returns
	// before any activation and leaves the durable stage pre-commit.
	if err := reporter.Advance(ctx, publishing.StageCommitting); err != nil {
		return publishing.Result{}, err
	}
	// Authorization is consumed before the call. No loop, retry, fallback or second
	// activation exists on this path, including when the result is ambiguous.
	if err := port.ActivateFinal(ctx, control); err != nil {
		return publishing.Result{}, err
	}
	if err := reporter.Advance(ctx, publishing.StageVerifying); err != nil {
		return publishing.Result{}, err
	}
	readback, err := port.Readback(ctx, prepared.Prepared.Snapshot.TargetID)
	if err != nil {
		return publishing.Result{}, err
	}
	if err := validateReadback(input.Manifest, prepared.Prepared.Snapshot, readback); err != nil {
		return publishing.Result{}, err
	}
	return publishing.Result{Status: "published", PublishedURL: readback.PublishedURL}, nil
}

func loadInput(dir string) (Input, FailureKind) {
	if !filepath.IsAbs(dir) {
		return Input{}, FailureSafe
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	info, err := os.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 1<<20 {
		return Input{}, FailureSafe
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return Input{}, FailureSafe
	}
	manifest := &postpilotv1.PublishManifest{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, manifest); err != nil {
		return Input{}, FailureSafe
	}
	paths := make([]string, 0, len(manifest.GetAssets()))
	for _, asset := range manifest.GetAssets() {
		if asset == nil || filepath.Base(asset.GetFilename()) != asset.GetFilename() {
			return Input{}, FailureAssetMissing
		}
		path := filepath.Join(dir, asset.GetFilename())
		relative, err := filepath.Rel(dir, path)
		if err != nil || relative == ".." || filepath.IsAbs(relative) {
			return Input{}, FailureAssetMissing
		}
		paths = append(paths, path)
	}
	return Input{Manifest: manifest, AssetPaths: paths}, ""
}

func validateReadback(manifest *postpilotv1.PublishManifest, prepared Snapshot, readback Readback) error {
	parsed, err := url.Parse(readback.PublishedURL)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "blog.naver.com") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("published URL is not an exact Naver post URL")
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(segments) != 2 {
		return errors.New("published URL does not belong to the expected Naver account")
	}
	account, err := url.PathUnescape(segments[0])
	if err != nil || account != manifest.GetExpectedPlatformAccountId() || !numericPostID.MatchString(segments[1]) {
		return errors.New("published URL does not belong to the expected Naver account")
	}
	snapshot := readback.Snapshot
	if snapshot.TargetID != prepared.TargetID || snapshot.AccountID != prepared.AccountID || snapshot.Token == "" || snapshot.Token == prepared.Token || snapshot.Title != prepared.Title || snapshot.ImageCount != prepared.ImageCount || !equalBlocks(snapshot.Body, prepared.Body) || !slices.Equal(snapshot.Tags, prepared.Tags) || snapshot.Category != prepared.Category || snapshot.Visibility != prepared.Visibility {
		return errors.New("published Naver readback does not match the frozen editor snapshot")
	}
	return nil
}

// Prepare deterministically fills and fully verifies the editor but cannot activate any
// publish-like control. It always returns one typed terminal preparation result.
func (p Publisher) Prepare(ctx context.Context, input Input, reporter publishing.Reporter) PreparationResult {
	fail := func(kind FailureKind) PreparationResult {
		return PreparationResult{Status: PreparationFailed, Failure: &Failure{Kind: kind}}
	}
	manifest, paths, kind := validateInput(input)
	if kind != "" {
		return fail(kind)
	}
	if p.Port == nil || reporter == nil {
		return fail(FailureSafe)
	}
	manifestCopy, _ := proto.Clone(manifest).(*postpilotv1.PublishManifest)
	if manifestCopy == nil {
		return fail(FailureSafe)
	}
	if err := reporter.Advance(ctx, publishing.StagePreparing); err != nil {
		return fail(FailureSafe)
	}
	compatibility, err := Manifest()
	if err != nil {
		return fail(FailureEditorChanged)
	}
	if err := reporter.Advance(ctx, publishing.StageOpeningEditor); err != nil {
		return fail(FailureSafe)
	}
	expected := Snapshot{AccountID: manifestCopy.GetExpectedPlatformAccountId(), SignatureID: compatibility.SignatureID, Auth: AuthReady}
	current, failure := p.observe(ctx, expected, true)
	if failure != "" {
		return fail(failure)
	}
	expected.TargetID, expected.URL = current.TargetID, current.URL
	if current.Title != "" || len(current.Body) != 0 || current.ImageCount != 0 || len(current.Tags) != 0 {
		return fail(FailureEditorChanged)
	}
	if err := reporter.Advance(ctx, publishing.StageFillingContent); err != nil {
		return fail(FailureSafe)
	}
	if failure = p.mutate(ctx, expected, Mutation{Kind: MutationTitle, Text: manifestCopy.GetContent().GetTitle()}, func(snapshot *Snapshot) { snapshot.Title = manifestCopy.GetContent().GetTitle() }); failure != "" {
		return fail(failure)
	}
	expected.Title = manifestCopy.GetContent().GetTitle()
	imageOrdinal := 0
	imageCaptions := make([]string, 0, len(paths))
	for _, block := range manifestCopy.GetContent().GetBlocks() {
		semantic, mutation, mapErr := mapBlock(block, imageOrdinal)
		if mapErr != nil {
			return fail(FailureSafe)
		}
		if semantic.Kind == SemanticImage {
			imageCaptions = append(imageCaptions, block.GetCaption())
			imageOrdinal++
		}
		if failure = p.mutate(ctx, expected, mutation, func(snapshot *Snapshot) { snapshot.Body = append(snapshot.Body, semantic) }); failure != "" {
			return fail(failure)
		}
		expected.Body = append(expected.Body, semantic)
	}
	tags := normalizeTags(manifestCopy.GetTags())
	for _, setting := range []struct {
		mutation Mutation
		update   func(*Snapshot)
	}{
		{Mutation{Kind: MutationTags, Values: tags}, func(snapshot *Snapshot) { snapshot.Tags = slices.Clone(tags) }},
		{Mutation{Kind: MutationCategory, ID: manifestCopy.GetCategoryId(), Name: manifestCopy.GetCategoryName()}, func(snapshot *Snapshot) {
			snapshot.Category = SelectedSetting{ID: manifestCopy.GetCategoryId(), Name: manifestCopy.GetCategoryName(), Selected: true}
		}},
		{Mutation{Kind: MutationVisibility, ID: visibilityID(manifestCopy.GetVisibility())}, func(snapshot *Snapshot) {
			snapshot.Visibility = SelectedSetting{ID: visibilityID(manifestCopy.GetVisibility()), Selected: true}
		}},
	} {
		if failure = p.mutate(ctx, expected, setting.mutation, setting.update); failure != "" {
			return fail(failure)
		}
		setting.update(&expected)
	}
	if err := reporter.Advance(ctx, publishing.StageUploadingPhotos); err != nil {
		return fail(FailureSafe)
	}
	for ordinal, path := range paths {
		beforeImages := expected.ImageCount
		mutation := Mutation{Kind: MutationUploadImage, Ordinal: ordinal, AssetPath: path}
		failure = p.mutate(ctx, expected, mutation, func(snapshot *Snapshot) {
			snapshot.ImageCount++
			for index := range snapshot.Body {
				if snapshot.Body[index].Kind == SemanticImage && snapshot.Body[index].Ordinal == ordinal {
					snapshot.Body[index].Uploaded = true
				}
			}
		})
		if failure != "" {
			return fail(failure)
		}
		expected.ImageCount++
		for index := range expected.Body {
			if expected.Body[index].Kind == SemanticImage && expected.Body[index].Ordinal == ordinal {
				expected.Body[index].Uploaded = true
				caption := imageCaptions[ordinal]
				if caption != "" {
					if failure = p.mutate(ctx, expected, Mutation{Kind: MutationImageCaption, Ordinal: ordinal, Text: caption}, func(snapshot *Snapshot) {
						for bodyIndex := range snapshot.Body {
							if snapshot.Body[bodyIndex].Kind == SemanticImage && snapshot.Body[bodyIndex].Ordinal == ordinal {
								snapshot.Body[bodyIndex].Caption = caption
							}
						}
					}); failure != "" {
						return fail(failure)
					}
					expected.Body[index].Caption = caption
				}
			}
		}
		if expected.ImageCount != beforeImages+1 {
			return fail(FailureEditorChanged)
		}
	}
	final, failure := p.observe(ctx, expected, false)
	if failure != "" || final.Token == "" || !equalSnapshot(final, expected) {
		if failure == "" {
			failure = FailureEditorChanged
		}
		return fail(failure)
	}
	return PreparationResult{Status: PreparationReady, Prepared: &Prepared{SnapshotToken: final.Token, Snapshot: final}}
}

func (p Publisher) mutate(ctx context.Context, expected Snapshot, mutation Mutation, update func(*Snapshot)) FailureKind {
	before, failure := p.observe(ctx, expected, false)
	if failure != "" || !equalSnapshot(before, expected) {
		if failure != "" {
			return failure
		}
		return FailureEditorChanged
	}
	if before.LocatorMatches[mutation.Kind] != 1 {
		return FailureEditorChanged
	}
	if err := p.Port.Apply(ctx, mutation); err != nil {
		return classifyPortError(err)
	}
	update(&expected)
	after, failure := p.observe(ctx, expected, false)
	if failure != "" {
		return failure
	}
	if before.Token == after.Token || !equalSnapshot(after, expected) {
		return FailureEditorChanged
	}
	return ""
}

func (p Publisher) observe(ctx context.Context, expected Snapshot, allowBlankBinding bool) (Snapshot, FailureKind) {
	snapshot, err := p.Port.Observe(ctx)
	if err != nil {
		return Snapshot{}, classifyPortError(err)
	}
	if snapshot.Auth != AuthReady {
		switch snapshot.Auth {
		case AuthLogin:
			return Snapshot{}, FailureLoginExpired
		case AuthCaptcha:
			return Snapshot{}, FailureCaptcha
		case Auth2FA:
			return Snapshot{}, FailureTwoFactor
		default:
			return Snapshot{}, FailureEditorChanged
		}
	}
	if snapshot.AccountID != expected.AccountID {
		return Snapshot{}, FailureAccountMismatch
	}
	if snapshot.SignatureID != expected.SignatureID || snapshot.TargetID == "" || snapshot.Token == "" || !strings.HasPrefix(snapshot.URL, "https://blog.naver.com/PostWriteForm.naver") || snapshot.UnversionedPublishLikeControls != 0 || snapshot.NativeDialogOpen {
		return Snapshot{}, FailureEditorChanged
	}
	if !allowBlankBinding && (snapshot.TargetID != expected.TargetID || snapshot.URL != expected.URL) {
		return Snapshot{}, FailureEditorChanged
	}
	return snapshot, ""
}

func validateInput(input Input) (*postpilotv1.PublishManifest, []string, FailureKind) {
	manifest := input.Manifest
	if manifest == nil || manifest.GetContent() == nil || strings.TrimSpace(manifest.GetJobId()) == "" || strings.TrimSpace(manifest.GetContent().GetTitle()) == "" ||
		strings.TrimSpace(manifest.GetExpectedPlatformAccountId()) == "" || strings.TrimSpace(manifest.GetCategoryId()) == "" || strings.TrimSpace(manifest.GetCategoryName()) == "" || visibilityID(manifest.GetVisibility()) == "" {
		return nil, nil, FailureSafe
	}
	if _, ok := uniqueTags(manifest.GetTags()); !ok {
		return nil, nil, FailureSafe
	}
	assets := manifest.GetAssets()
	if len(assets) != len(input.AssetPaths) {
		return nil, nil, FailureAssetMissing
	}
	imageFiles := make([]string, 0, len(assets))
	imageOrdinal := 0
	for _, block := range manifest.GetContent().GetBlocks() {
		if block == nil || block.GetSlot() != nil {
			return nil, nil, FailureSafe
		}
		if _, _, err := mapBlock(block, imageOrdinal); err != nil {
			return nil, nil, FailureSafe
		}
		if block.GetType() == postpilotv1.BlockType_IMAGE {
			imageFiles = append(imageFiles, block.GetFile())
			imageOrdinal++
		}
	}
	if len(imageFiles) != len(assets) {
		return nil, nil, FailureAssetMissing
	}
	paths := slices.Clone(input.AssetPaths)
	seenPaths := make(map[string]struct{}, len(paths))
	for ordinal, asset := range assets {
		if asset == nil || int(asset.GetOrdinal()) != ordinal || asset.GetDownloadUrl() != "" || asset.GetSourceFilename() != imageFiles[ordinal] || filepath.Base(asset.GetFilename()) != asset.GetFilename() || !strings.HasSuffix(strings.ToLower(asset.GetFilename()), ".jpg") {
			return nil, nil, FailureAssetMissing
		}
		path := paths[ordinal]
		info, err := os.Lstat(path)
		cleanPath := filepath.Clean(path)
		if err != nil || !filepath.IsAbs(path) || cleanPath != path || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() != asset.GetBytes() || filepath.Base(path) != asset.GetFilename() {
			return nil, nil, FailureAssetMissing
		}
		if _, exists := seenPaths[path]; exists {
			return nil, nil, FailureAssetMissing
		}
		seenPaths[path] = struct{}{}
	}
	return manifest, paths, ""
}

func mapBlock(block *postpilotv1.Block, imageOrdinal int) (SemanticBlock, Mutation, error) {
	switch block.GetType() {
	case postpilotv1.BlockType_TEXT:
		if strings.TrimSpace(block.GetContent()) == "" {
			return SemanticBlock{}, Mutation{}, errors.New("empty text")
		}
		return SemanticBlock{Kind: SemanticText, Text: block.GetContent()}, Mutation{Kind: MutationText, Text: block.GetContent()}, nil
	case postpilotv1.BlockType_HEADING:
		if strings.TrimSpace(block.GetContent()) == "" || block.GetLevel() < 1 || block.GetLevel() > 3 {
			return SemanticBlock{}, Mutation{}, errors.New("invalid heading")
		}
		return SemanticBlock{Kind: SemanticText, Text: block.GetContent()}, Mutation{Kind: MutationHeading, Text: block.GetContent(), Level: block.GetLevel()}, nil
	case postpilotv1.BlockType_QUOTE:
		if strings.TrimSpace(block.GetContent()) == "" {
			return SemanticBlock{}, Mutation{}, errors.New("empty quote")
		}
		return SemanticBlock{Kind: SemanticText, Text: "“" + block.GetContent() + "”"}, Mutation{Kind: MutationQuote, Text: block.GetContent()}, nil
	case postpilotv1.BlockType_LIST:
		if len(block.GetItems()) == 0 || slices.ContainsFunc(block.GetItems(), func(value string) bool { return strings.TrimSpace(value) == "" }) {
			return SemanticBlock{}, Mutation{}, errors.New("invalid list")
		}
		lines := make([]string, len(block.GetItems()))
		for index, item := range block.GetItems() {
			lines[index] = "- " + item
		}
		return SemanticBlock{Kind: SemanticText, Text: strings.Join(lines, "\n")}, Mutation{Kind: MutationList, Items: slices.Clone(block.GetItems())}, nil
	case postpilotv1.BlockType_IMAGE:
		if strings.TrimSpace(block.GetFile()) == "" {
			return SemanticBlock{}, Mutation{}, errors.New("empty image")
		}
		return SemanticBlock{Kind: SemanticImage, Ordinal: imageOrdinal}, Mutation{Kind: MutationImagePlaceholder, Ordinal: imageOrdinal}, nil
	default:
		return SemanticBlock{}, Mutation{}, fmt.Errorf("unsupported block type %s", block.GetType())
	}
}

func normalizeTags(values []string) []string {
	result, _ := uniqueTags(values)
	return result
}

func uniqueTags(values []string) ([]string, bool) {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.Join(strings.Fields(strings.TrimLeft(strings.TrimSpace(value), "#")), " ")
		if value == "" {
			return nil, false
		}
		if _, exists := seen[value]; exists {
			return nil, false
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, true
}

func visibilityID(value postpilotv1.PublishVisibility) string {
	switch value {
	case postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PUBLIC:
		return "public"
	case postpilotv1.PublishVisibility_PUBLISH_VISIBILITY_PRIVATE:
		return "private"
	default:
		return ""
	}
}

func equalSnapshot(got, want Snapshot) bool {
	return got.TargetID == want.TargetID && got.URL == want.URL && got.AccountID == want.AccountID && got.SignatureID == want.SignatureID && got.Auth == want.Auth && got.Title == want.Title &&
		got.ImageCount == want.ImageCount && slices.Equal(got.Tags, want.Tags) && got.Category == want.Category && got.Visibility == want.Visibility && equalBlocks(got.Body, want.Body)
}

func equalBlocks(left, right []SemanticBlock) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Kind != right[index].Kind || left[index].Text != right[index].Text || left[index].Level != right[index].Level || left[index].Ordinal != right[index].Ordinal || left[index].Caption != right[index].Caption || left[index].Uploaded != right[index].Uploaded || !slices.Equal(left[index].Items, right[index].Items) {
			return false
		}
	}
	return true
}

type PortError struct{ Kind FailureKind }

func (e PortError) Error() string { return string(e.Kind) }

func classifyPortError(err error) FailureKind {
	var typed PortError
	if errors.As(err, &typed) {
		switch typed.Kind {
		case FailureLoginExpired, FailureCaptcha, FailureTwoFactor, FailureAccountMismatch, FailureBrowserLost, FailureEditorChanged, FailureAssetMissing:
			return typed.Kind
		}
	}
	return FailureSafe
}
