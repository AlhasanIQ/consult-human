package provider

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteWhatsAppQRCodePNG(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "qr", "code.png")
	t.Setenv(envWhatsAppQRPngPath, outPath)

	gotPath, err := writeWhatsAppQRCodePNG("example-whatsapp-qr")
	if err != nil {
		t.Fatalf("writeWhatsAppQRCodePNG returned error: %v", err)
	}
	if gotPath != outPath {
		t.Fatalf("want %q got %q", outPath, gotPath)
	}

	data, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read png: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("png is empty")
	}
	if !bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("file is not a PNG")
	}
}

func TestWriteWhatsAppQRCodePNGRejectsEmptyCode(t *testing.T) {
	if _, err := writeWhatsAppQRCodePNG(" "); err == nil {
		t.Fatalf("expected error for empty code")
	}
}
