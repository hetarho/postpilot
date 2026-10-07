package generation

import "github.com/postpilot/backend/internal/llm"

// WritingTestDescription is private owner-side publication metadata decoded by
// the snapshot owner. Application adapters need no knowledge of storage JSON.
// It is not a customer inspection or blind-comparison projection.
type WritingTestDescription struct {
	UserID, Factor, ModelStage, SourcePostSlug string
	SourceRevision, AssignmentsHash            string
	InputRevision, ContentRevision             int64
	TargetLanguage                             Language
	RequiredFields                             []WritingTestTemplateField
	Variants                                   []WritingTestVariantDescription
}

type WritingTestVariantDescription struct {
	Reference                WritingTestReference
	Revision, SemanticKey    string
	Synthetic                bool
	Payload                  []byte
	ObserveModel, WriteModel llm.ModelRef
}

// RestoreWritingTestSnapshot reconstructs metadata omitted by an aggregate's
// public DTO from its retained private bytes. All bytes and the complete hash
// are validated before anything is returned; it never reads live source data.
func RestoreWritingTestSnapshot(common []byte, variants [][]byte, hash, promptVersion string) (WritingTestSnapshot, error) {
	var wire writingTestCommonWire
	if err := decodeWritingTestJSON(common, &wire); err != nil {
		return WritingTestSnapshot{}, err
	}
	value := WritingTestSnapshot{
		Common: append([]byte(nil), common...), Hash: hash,
		PromptVersion: promptVersion, AssignmentsHash: wire.AssignmentsHash,
		Variants: make([][]byte, len(variants)),
	}
	for index, raw := range variants {
		value.Variants[index] = append([]byte(nil), raw...)
	}
	decoded, decodedVariants, err := decodeWritingTestSnapshot(value)
	if err != nil {
		return WritingTestSnapshot{}, err
	}
	if !observerWritingTest(decoded) && !decoded.Prepared {
		targets, _ := frozenObserveSelection(decoded.Post.Images, decoded.ObserveFiles, decoded.Observations)
		value.ObserveCalls = len(writingTestBatches(targets, decoded.BatchSize))
	}
	if observerWritingTest(decoded) {
		for _, variant := range decodedVariants {
			value.ObserveCalls += len(writingTestBatches(variant.Snapshot.Post.Images, decoded.BatchSize))
		}
	}
	return value, nil
}

func DescribeWritingTestSnapshot(snapshot WritingTestSnapshot) (WritingTestDescription, error) {
	common, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		return WritingTestDescription{}, err
	}
	value := WritingTestDescription{
		UserID: common.Post.UserID, Factor: common.Factor, ModelStage: common.ModelStage,
		SourcePostSlug: common.Post.Slug, SourceRevision: common.SourceRevision,
		AssignmentsHash: common.AssignmentsHash, InputRevision: common.InputRevision,
		ContentRevision: common.ContentRevision, TargetLanguage: common.Post.TargetLanguage,
		RequiredFields: append([]WritingTestTemplateField(nil), common.RequiredFields...),
		Variants:       make([]WritingTestVariantDescription, len(variants)),
	}
	for index, variant := range variants {
		value.Variants[index] = WritingTestVariantDescription{
			Reference: variant.Reference, Revision: variant.Revision,
			SemanticKey: variant.SemanticKey, Synthetic: variant.Synthetic,
			Payload:      append([]byte(nil), variant.Payload...),
			ObserveModel: variant.ObserveModel, WriteModel: variant.WriteModel,
		}
	}
	return value, nil
}
