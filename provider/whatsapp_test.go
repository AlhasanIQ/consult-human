package provider

import "testing"

func TestParseWhatsAppJIDFromPhone(t *testing.T) {
	jid, err := parseWhatsAppJID("+1 (555) 123-4567")
	if err != nil {
		t.Fatalf("parseWhatsAppJID returned error: %v", err)
	}
	if got, want := jid.String(), "15551234567@s.whatsapp.net"; got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestParseWhatsAppJIDFromGroupJID(t *testing.T) {
	jid, err := parseWhatsAppJID("1234567890-123456@g.us")
	if err != nil {
		t.Fatalf("parseWhatsAppJID returned error: %v", err)
	}
	if got, want := jid.String(), "1234567890-123456@g.us"; got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestParseWhatsAppJIDRejectsInvalid(t *testing.T) {
	if _, err := parseWhatsAppJID("not-a-phone"); err == nil {
		t.Fatalf("expected error for invalid jid")
	}
}
