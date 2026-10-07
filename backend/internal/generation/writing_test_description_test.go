package generation

import (
	"context"
	"reflect"
	"testing"
)

func TestWritingTestDescriptionRestoresAllAxesWithoutSourceOrProviderReads(t *testing.T) {
	for _, axis := range []struct{ factor, stage string }{{"model", "observe"}, {"model", "write"}, {"voice", ""}, {"template", ""}, {"guideline", ""}} {
		t.Run(axis.factor+axis.stage, func(t *testing.T) {
			factory, ports, _, _, _, _ := plannerFixture(t)
			plannerAttachments(ports, 5, false)
			ports.source.Post.Content = &PostContent{Title: "old source", Tags: []string{"old"}, Blocks: []Block{{Type: BlockList, Items: []string{"source"}}}}
			request, _ := plannerRequest(t, ports, axis.factor, axis.stage, 16)
			original, err := factory.FreezeWritingTest(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			reads, checks := ports.sourceReads, len(ports.modelChecks)
			restored, err := RestoreWritingTestSnapshot(original.Common, original.Variants, original.Hash, original.PromptVersion)
			if err != nil || !reflect.DeepEqual(restored, original) {
				t.Fatalf("restore changed snapshot: %v", err)
			}
			view, err := DescribeWritingTestSnapshot(restored)
			if err != nil {
				t.Fatal(err)
			}
			if view.UserID != request.UserID || view.Factor != axis.factor || view.ModelStage != axis.stage || view.SourcePostSlug != request.SourcePostSlug || view.InputRevision != 7 || view.ContentRevision != 9 || view.AssignmentsHash != original.AssignmentsHash || view.TargetLanguage != LanguageKorean || len(view.Variants) != 16 {
				t.Fatalf("wrong private publication identity: %#v", view)
			}
			for index, variant := range view.Variants {
				var ref WritingTestReference
				if err := decodeWritingTestJSON(request.Variants[index], &ref); err != nil {
					t.Fatal(err)
				}
				if variant.Reference != ref || variant.Revision == "" || variant.SemanticKey == "" || variant.WriteModel.ModelID == "" {
					t.Fatalf("variant metadata lost: %#v", variant)
				}
			}
			payload := append([]byte(nil), view.Variants[0].Payload...)
			if len(view.Variants[0].Payload) > 0 {
				view.Variants[0].Payload[0] = 'x'
			}
			again, err := DescribeWritingTestSnapshot(restored)
			if err != nil || !reflect.DeepEqual(again.Variants[0].Payload, payload) {
				t.Fatalf("description aliases private bytes: %v", err)
			}
			if ports.sourceReads != reads || len(ports.modelChecks) != checks {
				t.Fatal("private description reread source or model")
			}
			// Ownership is established by the app before using these private bytes;
			// any corrupt/replaced byte or mismatched metadata invalidates the snapshot.
			corrupt := append([]byte(nil), original.Variants[0]...)
			corrupt[len(corrupt)-1] = ' '
			variants := append([][]byte(nil), original.Variants...)
			variants[0] = corrupt
			if _, err := RestoreWritingTestSnapshot(original.Common, variants, original.Hash, original.PromptVersion); err == nil {
				t.Fatal("corrupt variant accepted")
			}
			if _, err := RestoreWritingTestSnapshot(original.Common, original.Variants, original.Hash, "different"); err == nil {
				t.Fatal("changed prompt version accepted")
			}
			if _, err := RestoreWritingTestSnapshot(nil, nil, original.Hash, original.PromptVersion); err == nil {
				t.Fatal("purged snapshot restored")
			}
		})
	}
}
