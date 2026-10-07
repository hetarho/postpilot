package post

import "testing"

func TestOriginIdentitiesTolerateDurableEmptyArrayOmission(t *testing.T) {
	before := PostContent{Title: "Ready", Tags: []string{}, Blocks: []Block{{Type: BlockText, Content: "Valid prose", Items: []string{}, Files: []string{}}}}
	after := PostContent{Title: "Ready", Blocks: []Block{{Type: BlockText, Content: "Valid prose"}}}
	if ContentOriginIdentity(before, 3) != ContentOriginIdentity(after, 3) || ContentOriginIdentity(before, 3) == ContentOriginIdentity(after, 4) {
		t.Fatal("storage omission staled matching canonical identity or erased revision fence")
	}
	if ContentOriginIdentity(PostContent{}, 0) != ContentOriginIdentity(PostContent{Blocks: []Block{}}, 0) {
		t.Fatal("empty legacy content hash depended on omitted array spelling")
	}
	if ValidateContent(PostContent{}, nil, nil) == nil || ValidateContent(PostContent{Blocks: []Block{}}, nil, nil) == nil {
		t.Fatal("identity normalization admitted empty canonical machine content")
	}
	observation := Observation{File: "a.jpg", Scene: "Actual scene", Objects: []string{}, Events: []string{}}
	stored := Observation{File: "a.jpg", Scene: "Actual scene", Model: "new registry label", Rotation: 90, Origins: &ObservationOriginReview{Version: OriginVersion}}
	if ObservationOriginIdentity(observation) != ObservationOriginIdentity(stored) {
		t.Fatal("observation reuse identity changed with storage omission/provenance/presentation")
	}
	stored.Scene = "Different actual scene"
	if ObservationOriginIdentity(observation) == ObservationOriginIdentity(stored) {
		t.Fatal("observation material changes did not stale origin identity")
	}
}
