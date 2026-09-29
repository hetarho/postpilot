package rpc

import (
	"testing"

	"google.golang.org/protobuf/proto"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// VOICE-10: a voice is created by name alone. A client that still sends the retired language
// or description (fields 2-3) is read as a plain name.
func TestCreateVoiceTakesTheNameAlone(t *testing.T) {
	wire, err := proto.Marshal(&postpilotv1.CreateVoiceRequest{Name: "리뷰"})
	if err != nil {
		t.Fatal(err)
	}
	// field 2 (varint 1) and field 3 (a length-delimited "x"), as an older client wrote them.
	wire = append(wire, 0x10, 0x01, 0x1a, 0x01, 'x')
	var decoded postpilotv1.CreateVoiceRequest
	if err := proto.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.GetName() != "리뷰" {
		t.Fatalf("name = %q", decoded.GetName())
	}
}
