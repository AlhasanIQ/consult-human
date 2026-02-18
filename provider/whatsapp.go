package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/AlhasanIQ/consult-human/config"
	"github.com/AlhasanIQ/consult-human/contract"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
	"rsc.io/qr"
)

var whatsAppNonDigit = regexp.MustCompile(`\D`)

const envWhatsAppQRPngPath = "CONSULT_HUMAN_WHATSAPP_QR_PNG"

const whatsAppQRViewerHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>consult-human WhatsApp QR</title>
  <style>
    :root { color-scheme: light; }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      background: radial-gradient(circle at top, #f5f7ff 0%, #f6f8fb 45%, #eef2f6 100%);
      color: #18212f;
      font-family: "SF Pro Text", "Segoe UI", "Noto Sans", sans-serif;
    }
    .card {
      width: min(92vw, 560px);
      background: rgba(255, 255, 255, 0.92);
      border: 1px solid #d8e1ea;
      border-radius: 20px;
      box-shadow: 0 20px 50px rgba(20, 33, 50, 0.12);
      padding: 26px;
      backdrop-filter: blur(10px);
    }
    h1 { margin: 0 0 8px; font-size: 1.2rem; }
    p { margin: 0 0 18px; color: #3f5168; }
    .qr-wrap {
      background: #f3f7fb;
      border: 1px solid #d5dde8;
      border-radius: 16px;
      display: grid;
      place-items: center;
      padding: 18px;
    }
    #qr {
      width: min(70vw, 360px);
      max-width: 100%;
      image-rendering: pixelated;
      border-radius: 10px;
      background: #fff;
      padding: 12px;
      box-shadow: inset 0 0 0 1px #e8edf2;
    }
    .row {
      display: flex;
      gap: 10px;
      align-items: center;
      justify-content: space-between;
      margin-top: 14px;
      flex-wrap: wrap;
    }
    #status { color: #4d6078; font-size: 0.95rem; }
    button {
      border: 0;
      border-radius: 999px;
      background: #1166ff;
      color: #fff;
      padding: 8px 14px;
      font-size: 0.9rem;
      cursor: pointer;
    }
    button:hover { background: #0d57da; }
  </style>
</head>
<body>
  <main class="card">
    <h1>consult-human WhatsApp Pairing</h1>
    <p>Keep this tab open. The QR refreshes automatically as new codes are generated.</p>
    <div class="qr-wrap">
      <img id="qr" src="/qr.png" alt="WhatsApp QR">
    </div>
    <div class="row">
      <span id="status">Waiting for QR updates...</span>
      <button id="refresh" type="button">Refresh Now</button>
    </div>
  </main>
  <script>
    (function () {
      var qr = document.getElementById("qr");
      var status = document.getElementById("status");
      var refreshBtn = document.getElementById("refresh");
      var intervalMs = 1200;

      function refresh() {
        var now = new Date();
        qr.src = "/qr.png?v=" + now.getTime();
        status.textContent = "Last refresh: " + now.toLocaleTimeString();
      }

      qr.addEventListener("error", function () {
        status.textContent = "Waiting for the next QR code...";
      });

      refreshBtn.addEventListener("click", refresh);
      refresh();
      setInterval(refresh, intervalMs);
    })();
  </script>
</body>
</html>`

type whatsAppInbound struct {
	MessageID string
	ChatJID   string
	SenderJID string
	Text      string
	QuotedID  string
	At        time.Time
}

type WhatsAppProvider struct {
	recipient types.JID
	storePath string

	mu             sync.Mutex
	connected      bool
	client         *whatsmeow.Client
	pending        map[string]string
	inbox          chan whatsAppInbound
	qrViewerServer *http.Server
	qrViewerURL    string
	qrPNGPath      string
}

func NewWhatsApp(cfg config.Config) (*WhatsAppProvider, error) {
	recipientRaw := strings.TrimSpace(cfg.WhatsApp.Recipient)
	if recipientRaw == "" {
		return nil, fmt.Errorf("whatsapp.recipient is required")
	}
	recipient, err := parseWhatsAppJID(recipientRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid whatsapp.recipient: %w", err)
	}

	storePath, err := config.ExpandPath(strings.TrimSpace(cfg.WhatsApp.StorePath))
	if err != nil {
		return nil, err
	}
	if storePath == "" {
		storePath, err = config.DefaultWhatsAppStorePath()
		if err != nil {
			return nil, err
		}
	}

	return &WhatsAppProvider{
		recipient: recipient,
		storePath: storePath,
		pending:   make(map[string]string),
		inbox:     make(chan whatsAppInbound, 256),
	}, nil
}

func (p *WhatsAppProvider) Name() string { return "whatsapp" }

func (p *WhatsAppProvider) EnsureConnected(ctx context.Context) error {
	return p.ensureConnected(ctx)
}

func (p *WhatsAppProvider) Close() error {
	p.mu.Lock()
	client := p.client
	viewer := p.qrViewerServer
	p.client = nil
	p.qrViewerServer = nil
	p.qrViewerURL = ""
	p.qrPNGPath = ""
	p.connected = false
	p.mu.Unlock()

	if client != nil {
		client.Disconnect()
	}
	if viewer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = viewer.Shutdown(ctx)
	}
	return nil
}

func (p *WhatsAppProvider) Send(ctx context.Context, req contract.AskRequest) (string, error) {
	if err := p.ensureConnected(ctx); err != nil {
		return "", err
	}

	message := &waProto.Message{
		Conversation: proto.String(RenderPrompt(req)),
	}

	resp, err := p.client.SendMessage(ctx, p.recipient, message)
	if err != nil {
		return "", err
	}

	p.mu.Lock()
	p.pending[req.RequestID] = string(resp.ID)
	p.mu.Unlock()

	return req.RequestID, nil
}

func (p *WhatsAppProvider) Receive(ctx context.Context, requestID string) (contract.Reply, error) {
	if err := p.ensureConnected(ctx); err != nil {
		return contract.Reply{}, err
	}

	p.mu.Lock()
	targetID, ok := p.pending[requestID]
	p.mu.Unlock()
	if !ok || strings.TrimSpace(targetID) == "" {
		return contract.Reply{}, fmt.Errorf("unknown request id %q", requestID)
	}
	defer func() {
		p.mu.Lock()
		delete(p.pending, requestID)
		p.mu.Unlock()
	}()

	recipientChat := p.recipient.String()
	for {
		select {
		case <-ctx.Done():
			return contract.Reply{}, ctx.Err()
		case msg := <-p.inbox:
			if msg.ChatJID != recipientChat {
				continue
			}
			matchesByQuote := msg.QuotedID == targetID
			matchesByToken := strings.Contains(msg.Text, requestID)
			if !matchesByQuote && !matchesByToken {
				// Fallback: with exactly one pending request,
				// accept any text from the same chat as a reply.
				p.mu.Lock()
				pendingCount := len(p.pending)
				p.mu.Unlock()
				if pendingCount != 1 {
					continue
				}
			}

			from := msg.SenderJID
			if strings.TrimSpace(from) == "" {
				from = msg.ChatJID
			}

			return contract.Reply{
				RequestID:         requestID,
				Text:              strings.TrimSpace(msg.Text),
				Raw:               msg.Text,
				From:              from,
				ProviderMessageID: msg.MessageID,
				ReceivedAt:        msg.At.UTC(),
			}, nil
		}
	}
}

func (p *WhatsAppProvider) ensureConnected(ctx context.Context) error {
	p.mu.Lock()
	if p.connected && p.client != nil && p.client.IsConnected() {
		p.mu.Unlock()
		return nil
	}
	if p.client == nil {
		if err := p.initClient(); err != nil {
			p.mu.Unlock()
			return err
		}
	}
	client := p.client
	p.mu.Unlock()

	if client.IsConnected() {
		p.mu.Lock()
		p.connected = true
		p.mu.Unlock()
		return nil
	}

	if client.Store.ID == nil {
		qrChan, err := client.GetQRChannel(ctx)
		if err != nil {
			return err
		}
		if err := client.Connect(); err != nil {
			return err
		}

		qrStatus := newWhatsAppQRStatusWriter(os.Stderr)
		defer qrStatus.Finish()
		fmt.Fprintln(os.Stderr, "Scan the WhatsApp QR code to link this device.")

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case evt, ok := <-qrChan:
				if !ok {
					return fmt.Errorf("whatsapp QR channel closed")
				}
				switch evt.Event {
				case "code":
					if pngPath, err := writeWhatsAppQRCodePNG(evt.Code); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to write WhatsApp QR PNG: %v\n", err)
					} else {
						if viewerURL, _, err := p.ensureLocalQRViewerServer(pngPath); err != nil {
							fmt.Fprintf(os.Stderr, "Failed to start local QR viewer: %v\n", err)
							qrStatus.Update(pngPath, "")
						} else {
							qrStatus.Update(pngPath, viewerURL)
						}
					}
				case "success":
					return p.waitForConnected(ctx, client, 30*time.Second)
				case "timeout":
					return fmt.Errorf("whatsapp QR timed out")
				case "error":
					return fmt.Errorf("whatsapp login error")
				}
			}
		}
	}

	if err := client.Connect(); err != nil {
		return err
	}
	return p.waitForConnected(ctx, client, 45*time.Second)
}

func (p *WhatsAppProvider) waitForConnected(ctx context.Context, client *whatsmeow.Client, maxWait time.Duration) error {
	timer := time.NewTimer(maxWait)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()

	for {
		if client.IsConnected() {
			p.mu.Lock()
			p.connected = true
			p.mu.Unlock()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("whatsapp connection timeout")
		case <-ticker.C:
		}
	}
}

func (p *WhatsAppProvider) ensureLocalQRViewerServer(pngPath string) (string, bool, error) {
	p.mu.Lock()
	p.qrPNGPath = pngPath
	if p.qrViewerServer != nil {
		url := p.qrViewerURL
		p.mu.Unlock()
		return url, false, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleLocalQRViewerHTML)
	mux.HandleFunc("/qr.png", p.handleLocalQRViewerPNG)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		p.mu.Unlock()
		return "", false, err
	}
	server := &http.Server{Handler: mux}
	p.qrViewerServer = server
	p.qrViewerURL = "http://" + listener.Addr().String()
	viewerURL := p.qrViewerURL
	p.mu.Unlock()

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "WhatsApp QR viewer server error: %v\n", err)
		}
	}()

	return viewerURL, true, nil
}

func (p *WhatsAppProvider) handleLocalQRViewerHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	_, _ = io.WriteString(w, whatsAppQRViewerHTML)
}

func (p *WhatsAppProvider) handleLocalQRViewerPNG(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/qr.png" {
		http.NotFound(w, r)
		return
	}

	p.mu.Lock()
	pngPath := p.qrPNGPath
	p.mu.Unlock()
	if strings.TrimSpace(pngPath) == "" {
		http.Error(w, "QR PNG path is not set", http.StatusNotFound)
		return
	}

	data, err := os.ReadFile(pngPath)
	if err != nil {
		http.Error(w, "QR PNG is not available yet", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (p *WhatsAppProvider) initClient() error {
	if err := os.MkdirAll(filepath.Dir(p.storePath), 0o755); err != nil {
		return err
	}

	dsn := fmt.Sprintf("file:%s?_foreign_keys=on", p.storePath)
	container, err := sqlstore.New(context.Background(), "sqlite3", dsn, waLog.Stdout("DB", "ERROR", true))
	if err != nil {
		return err
	}

	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return err
	}

	client := whatsmeow.NewClient(device, waLog.Stdout("Client", "ERROR", true))
	client.AddEventHandler(p.handleEvent)

	p.client = client
	return nil
}

func (p *WhatsAppProvider) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Connected:
		p.mu.Lock()
		p.connected = true
		p.mu.Unlock()
	case *events.Disconnected:
		p.mu.Lock()
		p.connected = false
		p.mu.Unlock()
	case *events.Message:
		if v.Info.IsFromMe {
			return
		}
		text := extractWhatsAppText(v.Message)
		if strings.TrimSpace(text) == "" {
			return
		}
		quoted := extractQuotedID(v.Message)
		msg := whatsAppInbound{
			MessageID: string(v.Info.ID),
			ChatJID:   v.Info.Chat.String(),
			SenderJID: v.Info.Sender.String(),
			Text:      text,
			QuotedID:  quoted,
			At:        v.Info.Timestamp,
		}
		select {
		case p.inbox <- msg:
		default:
		}
	}
}

func parseWhatsAppJID(raw string) (types.JID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return types.JID{}, fmt.Errorf("empty")
	}

	if strings.Contains(raw, "@") {
		jid, err := types.ParseJID(raw)
		if err != nil {
			return types.JID{}, err
		}
		if jid.User == "" {
			return types.JID{}, fmt.Errorf("missing user part")
		}
		return jid, nil
	}

	digits := whatsAppNonDigit.ReplaceAllString(raw, "")
	if digits == "" {
		return types.JID{}, fmt.Errorf("expected phone number or full JID")
	}
	return types.ParseJID(digits + "@s.whatsapp.net")
}

func extractWhatsAppText(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}
	if c := strings.TrimSpace(msg.GetConversation()); c != "" {
		return c
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		if t := strings.TrimSpace(ext.GetText()); t != "" {
			return t
		}
	}
	if img := msg.GetImageMessage(); img != nil {
		if t := strings.TrimSpace(img.GetCaption()); t != "" {
			return t
		}
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		if t := strings.TrimSpace(vid.GetCaption()); t != "" {
			return t
		}
	}
	return ""
}

func extractQuotedID(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil && ext.ContextInfo != nil {
		return strings.TrimSpace(ext.ContextInfo.GetStanzaID())
	}
	if img := msg.GetImageMessage(); img != nil && img.ContextInfo != nil {
		return strings.TrimSpace(img.ContextInfo.GetStanzaID())
	}
	if vid := msg.GetVideoMessage(); vid != nil && vid.ContextInfo != nil {
		return strings.TrimSpace(vid.ContextInfo.GetStanzaID())
	}
	return ""
}

func writeWhatsAppQRCodePNG(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("empty QR code")
	}

	path := strings.TrimSpace(os.Getenv(envWhatsAppQRPngPath))
	if path == "" {
		path = filepath.Join(os.TempDir(), "consult-human-whatsapp-qr.png")
	}
	expanded, err := config.ExpandPath(path)
	if err != nil {
		return "", err
	}
	if expanded == "" {
		return "", fmt.Errorf("invalid QR PNG path")
	}

	if err := os.MkdirAll(filepath.Dir(expanded), 0o755); err != nil {
		return "", err
	}

	qrCode, err := qr.Encode(code, qr.M)
	if err != nil {
		return "", err
	}
	tmpPath := expanded + ".tmp"
	if err := os.WriteFile(tmpPath, qrCode.PNG(), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, expanded); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return expanded, nil
}

type whatsAppQRStatusWriter struct {
	w             io.Writer
	tty           bool
	linesRendered int
	viewerEmitted bool
}

func newWhatsAppQRStatusWriter(w io.Writer) *whatsAppQRStatusWriter {
	s := &whatsAppQRStatusWriter{w: w}
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		s.tty = true
	}
	return s
}

func (s *whatsAppQRStatusWriter) Update(pngPath string, viewerURL string) {
	if s == nil || s.w == nil {
		return
	}

	linePNG := fmt.Sprintf("WhatsApp QR PNG (refreshed %s): %s", time.Now().Format("15:04:05"), pngPath)
	if !s.tty {
		fmt.Fprintln(s.w, linePNG)
		if strings.TrimSpace(viewerURL) != "" && !s.viewerEmitted {
			fmt.Fprintf(s.w, "WhatsApp QR viewer (auto-refresh): %s\n", viewerURL)
			s.viewerEmitted = true
		}
		return
	}

	lines := []string{linePNG}
	if strings.TrimSpace(viewerURL) != "" {
		lines = append(lines, fmt.Sprintf("WhatsApp QR viewer (auto-refresh): %s", viewerURL))
	}
	s.rewrite(lines)
}

func (s *whatsAppQRStatusWriter) Finish() {
	if s == nil || s.w == nil || !s.tty {
		return
	}
	if s.linesRendered > 0 {
		fmt.Fprintln(s.w)
	}
}

func (s *whatsAppQRStatusWriter) rewrite(lines []string) {
	for i := 0; i < s.linesRendered; i++ {
		// Move up and clear previous status lines.
		fmt.Fprint(s.w, "\x1b[1A\x1b[2K")
	}
	for _, line := range lines {
		fmt.Fprintln(s.w, line)
	}
	s.linesRendered = len(lines)
}
