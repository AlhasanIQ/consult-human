package provider

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureLocalQRViewerServerServesHTMLAndPNG(t *testing.T) {
	p := &WhatsAppProvider{}
	t.Cleanup(func() {
		_ = p.Close()
	})

	pngPath := filepath.Join(t.TempDir(), "qr", "code.png")
	t.Setenv(envWhatsAppQRPngPath, pngPath)
	generatedPath, err := writeWhatsAppQRCodePNG("example-whatsapp-qr")
	if err != nil {
		t.Fatalf("writeWhatsAppQRCodePNG: %v", err)
	}

	viewerURL, started, err := p.ensureLocalQRViewerServer(generatedPath)
	if err != nil {
		t.Fatalf("ensureLocalQRViewerServer: %v", err)
	}
	if !started {
		t.Fatalf("expected viewer server to start")
	}
	if !strings.HasPrefix(viewerURL, "http://127.0.0.1:") {
		t.Fatalf("unexpected viewer URL: %q", viewerURL)
	}

	client := &http.Client{Timeout: 2 * time.Second}

	var htmlResp *http.Response
	for i := 0; i < 10; i++ {
		htmlResp, err = client.Get(viewerURL + "/")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET viewer html: %v", err)
	}
	defer htmlResp.Body.Close()
	if htmlResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected html status: %d", htmlResp.StatusCode)
	}
	htmlBody, err := io.ReadAll(htmlResp.Body)
	if err != nil {
		t.Fatalf("read html body: %v", err)
	}
	if !strings.Contains(string(htmlBody), "consult-human WhatsApp Pairing") {
		t.Fatalf("viewer html missing expected title")
	}

	pngResp, err := client.Get(viewerURL + "/qr.png")
	if err != nil {
		t.Fatalf("GET viewer png: %v", err)
	}
	defer pngResp.Body.Close()
	if pngResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected png status: %d", pngResp.StatusCode)
	}
	pngBytes, err := io.ReadAll(pngResp.Body)
	if err != nil {
		t.Fatalf("read png body: %v", err)
	}
	if !bytes.HasPrefix(pngBytes, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("viewer png response is not PNG")
	}

	viewerURL2, started2, err := p.ensureLocalQRViewerServer(generatedPath)
	if err != nil {
		t.Fatalf("ensureLocalQRViewerServer second call: %v", err)
	}
	if started2 {
		t.Fatalf("expected existing viewer server to be reused")
	}
	if viewerURL2 != viewerURL {
		t.Fatalf("expected same viewer URL, got %q and %q", viewerURL, viewerURL2)
	}
}

func TestLocalQRViewerPNGNotFoundWithoutPath(t *testing.T) {
	p := &WhatsAppProvider{}
	req, err := http.NewRequest(http.MethodGet, "/qr.png", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	rr := httptest.NewRecorder()
	p.handleLocalQRViewerPNG(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
