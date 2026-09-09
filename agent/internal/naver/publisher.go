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
	// SettingsLayerOpen is true only when exactly one reviewed shown settings layer is
	// present. It is part of the full fence snapshot, not an inferred driver state.
	SettingsLayerOpen bool
	// Rebuilt from the current DOM and full accessibility tree. The publisher
	// requires one exact reviewed semantic match for its next typed mutation.
	LocatorMatches                 map[MutationKind]int
	UnversionedPublishLikeControls int
	NativeDialogOpen               bool
}

type MutationKind string

const (
	MutationTitle        MutationKind = "title"
	MutationText         MutationKind = "text"
	MutationHeading      MutationKind = "heading"
	MutationQuote        MutationKind = "quote"
	MutationList         MutationKind = "list"
	MutationUploadImage  MutationKind = "upload_image"
	MutationImageCaption MutationKind = "image_caption"
	// MutationOpenSettings opens the layer the three settings live behind. It replaced
	// image_placeholder in r4: an image needs no pre-allocated slot (PUBLISH-36) while the
	// settings are unreachable, and the editor is occluded, until the layer is open
	// (PUBLISH-37).
	MutationOpenSettings MutationKind = "open_settings"
	MutationTags         MutationKind = "tags"
	MutationCategory     MutationKind = "category"
	MutationVisibility   MutationKind = "visibility"
)

// reviewedMutationKinds is the closed vocabulary this signed driver release implements, in
// the only order Prepare may plan them. It is the one source of truth: the manifest
// locators, the observed locator counts and the probe all iterate it.
var reviewedMutationKinds = []MutationKind{
	MutationTitle, MutationText, MutationHeading, MutationQuote, MutationList,
	MutationUploadImage, MutationImageCaption,
	MutationOpenSettings, MutationTags, MutationCategory, MutationVisibility,
}

// bodyMutationKinds are the kinds that write into the editor document itself. None of them
// may be planned or applied once the settings layer is open, because the layer covers the
// editor and a click resolved from a body element's own geometry would land on the layer
// (PUBLISH-37).
var bodyMutationKinds = []MutationKind{
	MutationTitle, MutationText, MutationHeading, MutationQuote, MutationList,
	MutationUploadImage, MutationImageCaption,
}

func isBodyMutation(kind MutationKind) bool { return slices.Contains(bodyMutationKinds, kind) }

// plannedMutation pairs one typed command with the projection change it is expected to
// produce, so Prepare can assert the whole plan's order before touching the page.
type plannedMutation struct {
	mutation Mutation
	update   func(*Snapshot)
}

// assertPlanOrder refuses a plan that writes to the document after the settings layer
// opened, that writes a paragraph after a quote or list conversion, that converts forwards
// instead of backwards, that opens the layer twice, or that names a kind outside the
// reviewed vocabulary.
func assertPlanOrder(plan []plannedMutation) error {
	settingsOpen := false
	opened := 0
	converted := false
	lastConversion := 0
	for _, step := range plan {
		if !slices.Contains(reviewedMutationKinds, step.mutation.Kind) {
			return fmt.Errorf("unreviewed mutation kind %q", step.mutation.Kind)
		}
		if step.mutation.Kind == MutationOpenSettings {
			opened++
			settingsOpen = true
			continue
		}
		if settingsOpen && isBodyMutation(step.mutation.Kind) {
			return fmt.Errorf("%s planned after the settings layer opened", step.mutation.Kind)
		}
		// A converted quotation traps the caret in its 출처 module and a converted list turns
		// the next Enter into another list item, so no paragraph may be written once a
		// conversion has landed, and the conversions themselves must run backwards: 인용구
		// adds a citation paragraph and a list adds its remaining items, each shifting the
		// index of every paragraph after it. Verified live 2026-09-10.
		if step.mutation.Kind == MutationQuote || step.mutation.Kind == MutationList {
			if converted && step.mutation.Ordinal >= lastConversion {
				return fmt.Errorf("%s converts paragraph %d after paragraph %d: the conversion pass runs in reverse document order", step.mutation.Kind, step.mutation.Ordinal, lastConversion)
			}
			converted = true
			lastConversion = step.mutation.Ordinal
			continue
		}
		if converted && isBodyMutation(step.mutation.Kind) {
			// Every remaining body kind places itself from the caret or from a paragraph
			// index, and a landed conversion has moved both.
			return fmt.Errorf("%s planned after a quote or list conversion", step.mutation.Kind)
		}
	}
	if opened != 1 {
		return fmt.Errorf("the settings layer must be opened exactly once, planned %d", opened)
	}
	return nil
}

// plannedBody replays a plan's body appends to learn the index the NEXT block will occupy.
// It exists because a conversion and a caption both address a block by its position in the
// projection, and with photos interleaved that position is no longer the block's position
// among the text blocks.
func plannedBody(plan []plannedMutation) []SemanticBlock {
	var snapshot Snapshot
	for _, step := range plan {
		step.update(&snapshot)
	}
	return snapshot.Body
}

// assertEnumeratedAssets refuses a plan whose upload names a path outside this job's
// enumerated asset set, or whose upload count disagrees with it, and refuses an asset path
// on any other kind. PUB-19 allows only enumerated current-job JPEG paths to reach the
// restricted upload operation, and Prepare builds the plan from that very set — so this is a
// structural guard on the plan rather than a reachable branch, which is exactly why it is
// asserted instead of assumed.
func assertEnumeratedAssets(plan []plannedMutation, paths []string) error {
	// Each enumerated asset is consumed exactly once, so an upload naming a path outside
	// the set and an upload naming the same file twice are both refused.
	remaining := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		remaining[path] = struct{}{}
	}
	uploads := 0
	for _, step := range plan {
		if step.mutation.Kind != MutationUploadImage {
			if step.mutation.AssetPath != "" {
				return fmt.Errorf("%s carries an asset path", step.mutation.Kind)
			}
			continue
		}
		// PUB-21: one at a time in exact manifest-ordinal order. A plan that jumps, repeats
		// or reverses an ordinal is refused before anything is uploaded.
		if step.mutation.Ordinal != uploads {
			return fmt.Errorf("upload %d carries ordinal %d", uploads, step.mutation.Ordinal)
		}
		uploads++
		if _, ok := remaining[step.mutation.AssetPath]; !ok {
			return fmt.Errorf("upload_image names a path that is not one of this job's %d unconsumed enumerated assets", len(paths))
		}
		delete(remaining, step.mutation.AssetPath)
	}
	if uploads != len(paths) || len(remaining) != 0 {
		return fmt.Errorf("planned %d uploads for %d enumerated assets", uploads, len(paths))
	}
	return nil
}

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
	if current.Title != "" || len(current.Body) != 0 || current.ImageCount != 0 || len(current.Tags) != 0 ||
		current.SettingsLayerOpen || current.Category.Selected || current.Visibility.Selected ||
		current.LocatorMatches[MutationTags] != 0 || current.LocatorMatches[MutationCategory] != 0 || current.LocatorMatches[MutationVisibility] != 0 {
		return fail(FailureEditorChanged)
	}
	if err := reporter.Advance(ctx, publishing.StageFillingContent); err != nil {
		return fail(FailureSafe)
	}

	// The whole plan is built and order-checked before anything is typed, so
	// PUBLISH-37's rule that no document write may follow the settings layer is a
	// property of the plan rather than something each step has to remember.
	title := manifestCopy.GetContent().GetTitle()
	plan := []plannedMutation{{
		mutation: Mutation{Kind: MutationTitle, Text: title},
		update:   func(snapshot *Snapshot) { snapshot.Title = title },
	}}

	// The body runs in two passes. The first walks the manifest IN ORDER, writing each text
	// block as a plain paragraph and uploading each photo where it belongs — a photo needs
	// no index at all, because SmartEditor splits the caret's text component at the caret's
	// paragraph and puts the image between the halves, and the caret is already at the end
	// of the block just written (verified live 2026-09-10). A heading converts inside its
	// own write, since Naver exports a section title as plain text and a separate step could
	// never move the snapshot token.
	//
	// The second pass converts the quotes and lists. Those cannot be interleaved: a
	// converted quotation traps the caret in its 출처 module and a converted list turns the
	// next Enter into another list item, so anything written after either would land in the
	// wrong place. It runs BACKWARDS so the paragraphs a conversion inserts never shift an
	// index still to be converted.
	imageOrdinal := 0
	conversions := make([]plannedMutation, 0, len(manifestCopy.GetContent().GetBlocks()))
	paragraphIndex := 0
	for _, block := range manifestCopy.GetContent().GetBlocks() {
		planned, planErr := planBlock(block, imageOrdinal, paragraphIndex)
		if planErr != nil {
			return fail(FailureSafe)
		}
		bodyIndex := len(plannedBody(plan))
		if planned.written.Kind == SemanticImage {
			if imageOrdinal >= len(paths) {
				return fail(FailureAssetMissing)
			}
			ordinal, path, written := imageOrdinal, paths[imageOrdinal], planned.written
			written.Uploaded = true
			plan = append(plan, plannedMutation{
				mutation: Mutation{Kind: MutationUploadImage, Ordinal: ordinal, AssetPath: path},
				update: func(snapshot *Snapshot) {
					snapshot.ImageCount++
					snapshot.Body = append(snapshot.Body, written)
				},
			})
			if caption := block.GetCaption(); caption != "" {
				index := bodyIndex
				plan = append(plan, plannedMutation{
					mutation: Mutation{Kind: MutationImageCaption, Ordinal: ordinal, Text: caption},
					update:   func(snapshot *Snapshot) { snapshot.Body[index].Caption = caption },
				})
			}
			imageOrdinal++
			continue
		}
		written := planned.written
		plan = append(plan, plannedMutation{
			mutation: planned.write,
			update:   func(snapshot *Snapshot) { snapshot.Body = append(snapshot.Body, written) },
		})
		if planned.conversion.Kind != "" {
			index, converted := bodyIndex, planned.converted
			conversions = append(conversions, plannedMutation{
				mutation: planned.conversion,
				update:   func(snapshot *Snapshot) { snapshot.Body[index].Text = converted },
			})
		}
		// A photo produces no paragraph of its own — its caption module lives inside the
		// image component and never enters the body-paragraph order — so only the text
		// blocks advance the index a conversion addresses. Each of them contributes exactly
		// one paragraph at write time, a list included: its remaining items arrive with its
		// own conversion, after every earlier index has already been used.
		paragraphIndex++
	}
	slices.Reverse(conversions)
	plan = append(plan, conversions...)
	if imageOrdinal != len(paths) {
		return fail(FailureSafe)
	}

	tags := normalizeTags(manifestCopy.GetTags())
	categoryID, categoryName := manifestCopy.GetCategoryId(), manifestCopy.GetCategoryName()
	visibility := visibilityID(manifestCopy.GetVisibility())
	plan = append(plan, plannedMutation{
		mutation: Mutation{Kind: MutationOpenSettings},
		update:   func(snapshot *Snapshot) { snapshot.SettingsLayerOpen = true },
	})
	if len(tags) > 0 {
		plan = append(plan, plannedMutation{
			mutation: Mutation{Kind: MutationTags, Values: tags},
			update:   func(snapshot *Snapshot) { snapshot.Tags = slices.Clone(tags) },
		})
	}
	plan = append(plan,
		plannedMutation{
			mutation: Mutation{Kind: MutationCategory, ID: categoryID, Name: categoryName},
			update: func(snapshot *Snapshot) {
				snapshot.Category = SelectedSetting{ID: categoryID, Name: categoryName, Selected: true}
			},
		},
		plannedMutation{
			mutation: Mutation{Kind: MutationVisibility, ID: visibility},
			update: func(snapshot *Snapshot) {
				snapshot.Visibility = SelectedSetting{ID: visibility, Name: visibilityName(visibility), Selected: true}
			},
		},
	)

	if err := assertPlanOrder(plan); err != nil {
		return fail(FailureSafe)
	}
	if err := assertEnumeratedAssets(plan, paths); err != nil {
		return fail(FailureAssetMissing)
	}

	photosAnnounced := false
	for _, step := range plan {
		if !photosAnnounced && step.mutation.Kind == MutationUploadImage {
			if err := reporter.Advance(ctx, publishing.StageUploadingPhotos); err != nil {
				return fail(FailureSafe)
			}
			photosAnnounced = true
		}
		// PUBLISH-13 r4 adds a `filling_settings` stage here. It is a proto, backend and
		// frontend contract change and belongs to T045 with the rest of the fence
		// reporting; until then the settings run under `uploading_photos`.
		if failure = p.mutate(ctx, expected, step.mutation, step.update); failure != "" {
			return fail(failure)
		}
		step.update(&expected)
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
	if before.LocatorMatches[mutation.Kind] != expectedLocatorMatches(mutation.Kind) {
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
		if _, err := planBlock(block, imageOrdinal, 0); err != nil {
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

// plannedBlock is one manifest block split into the write the first body pass performs and,
// for the two kinds whose conversion traps the caret, the conversion the second pass applies
// to the paragraph that write produced.
//
// A heading carries no conversion because its conversion is invisible to the projection —
// Naver exports a section title as plain text — so it has to land inside its own write step
// for Publisher.mutate to see the token move. A quote and a list both change the block's
// text, so their conversions are observable on their own.
type plannedBlock struct {
	// written is the projection right after the write, before any conversion.
	written SemanticBlock
	write   Mutation
	// conversion has a zero Kind when the block needs none.
	conversion Mutation
	// converted is the block's text once its conversion landed.
	converted string
}

func planBlock(block *postpilotv1.Block, imageOrdinal, paragraphIndex int) (plannedBlock, error) {
	switch block.GetType() {
	case postpilotv1.BlockType_TEXT:
		if strings.TrimSpace(block.GetContent()) == "" {
			return plannedBlock{}, errors.New("empty text")
		}
		return plannedBlock{
			written: SemanticBlock{Kind: SemanticText, Text: block.GetContent()},
			write:   Mutation{Kind: MutationText, Text: block.GetContent()},
		}, nil
	case postpilotv1.BlockType_HEADING:
		if strings.TrimSpace(block.GetContent()) == "" || block.GetLevel() < 1 || block.GetLevel() > 3 {
			return plannedBlock{}, errors.New("invalid heading")
		}
		return plannedBlock{
			written: SemanticBlock{Kind: SemanticText, Text: block.GetContent()},
			write:   Mutation{Kind: MutationHeading, Text: block.GetContent(), Level: block.GetLevel()},
		}, nil
	case postpilotv1.BlockType_QUOTE:
		if strings.TrimSpace(block.GetContent()) == "" {
			return plannedBlock{}, errors.New("empty quote")
		}
		return plannedBlock{
			written:    SemanticBlock{Kind: SemanticText, Text: block.GetContent()},
			write:      Mutation{Kind: MutationText, Text: block.GetContent()},
			conversion: Mutation{Kind: MutationQuote, Text: block.GetContent(), Ordinal: paragraphIndex},
			converted:  "“" + block.GetContent() + "”",
		}, nil
	case postpilotv1.BlockType_LIST:
		if len(block.GetItems()) == 0 || slices.ContainsFunc(block.GetItems(), func(value string) bool { return strings.TrimSpace(value) == "" }) {
			return plannedBlock{}, errors.New("invalid list")
		}
		lines := make([]string, len(block.GetItems()))
		for index, item := range block.GetItems() {
			lines[index] = "- " + item
		}
		// The first item is written as a plain paragraph; the conversion turns it into the
		// list's first LI and appends the rest.
		return plannedBlock{
			written:    SemanticBlock{Kind: SemanticText, Text: block.GetItems()[0]},
			write:      Mutation{Kind: MutationText, Text: block.GetItems()[0]},
			conversion: Mutation{Kind: MutationList, Items: slices.Clone(block.GetItems()), Ordinal: paragraphIndex},
			converted:  strings.Join(lines, "\n"),
		}, nil
	case postpilotv1.BlockType_IMAGE:
		if strings.TrimSpace(block.GetFile()) == "" {
			return plannedBlock{}, errors.New("empty image")
		}
		// r4: an image is created by its own upload at the position the caret gives it, so
		// the body pass plans nothing for it (PUBLISH-36).
		return plannedBlock{written: SemanticBlock{Kind: SemanticImage, Ordinal: imageOrdinal}}, nil
	default:
		return plannedBlock{}, fmt.Errorf("unsupported block type %s", block.GetType())
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

func visibilityName(id string) string {
	switch id {
	case "public":
		return "전체공개"
	case "neighbor":
		return "이웃공개"
	case "both_neighbor":
		return "서로이웃공개"
	case "private":
		return "비공개"
	default:
		return ""
	}
}

func expectedLocatorMatches(kind MutationKind) int {
	if kind == MutationVisibility {
		return 4
	}
	return 1
}

func equalSnapshot(got, want Snapshot) bool {
	return got.TargetID == want.TargetID && got.URL == want.URL && got.AccountID == want.AccountID && got.SignatureID == want.SignatureID && got.Auth == want.Auth && got.Title == want.Title &&
		got.ImageCount == want.ImageCount && slices.Equal(got.Tags, want.Tags) && got.Category == want.Category && got.Visibility == want.Visibility &&
		got.SettingsLayerOpen == want.SettingsLayerOpen && equalBlocks(got.Body, want.Body)
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
