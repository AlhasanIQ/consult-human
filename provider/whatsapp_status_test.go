package provider

import (
	"bytes"
	"strings"
	"testing"
)

func TestWhatsAppQRStatusWriterNonTTY(t *testing.T) {
	var out bytes.Buffer
	w := newWhatsAppQRStatusWriter(&out)

	w.Update("/tmp/consult-human-whatsapp-qr.png", "http://127.0.0.1:9999")
	w.Update("/tmp/consult-human-whatsapp-qr.png", "http://127.0.0.1:9999")

	got := out.String()
	if strings.Count(got, "WhatsApp QR PNG (refreshed") != 2 {
		t.Fatalf("expected 2 png update lines, got output: %q", got)
	}
	if strings.Count(got, "WhatsApp QR viewer (auto-refresh):") != 1 {
		t.Fatalf("expected 1 viewer line, got output: %q", got)
	}
}
