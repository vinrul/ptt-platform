package firebase

import "testing"

func TestPttWakeupDataIncludesDirectSpeaker(t *testing.T) {
	data := pttWakeupData(PttWakeup{
		GroupID:         "group-1",
		SessionID:       "session-1",
		Mode:            "direct",
		SpeakerUserID:   "speaker-1",
		SpeakerUsername: "field1",
	})

	if data["mode"] != "direct" {
		t.Fatalf("expected direct mode, got %q", data["mode"])
	}
	if data["speakerUserId"] != "speaker-1" {
		t.Fatalf("expected speaker user id, got %q", data["speakerUserId"])
	}
	if data["speakerUsername"] != "field1" {
		t.Fatalf("expected speaker username, got %q", data["speakerUsername"])
	}
}
