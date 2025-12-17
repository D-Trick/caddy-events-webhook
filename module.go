package eventwebhook

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"go.uber.org/zap"
)

type EventWebhook struct {
	Logger *zap.Logger

	URL string `json:"url,omitempty"`
	Method string `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout caddy.Duration `json:"timeout,omitempty"`
}

func init() {
	caddy.RegisterModule(EventWebhook{})
}

func (EventWebhook) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "events.handlers.webhook",
		New: func() caddy.Module { return new(EventWebhook) },
	}
}

func (w *EventWebhook) Provision(ctx caddy.Context) error {
	w.Logger = ctx.Logger(w)
	
	if w.Method == "" {
		w.Method = "POST"
	}
	if w.Timeout == 0 {
		w.Timeout = caddy.Duration(30 * time.Second)
	}
	if w.Headers == nil {
		w.Headers = make(map[string]string)
	}
	
	return nil
}

// Caddy Event Handle
func (w *EventWebhook) Handle(ctx caddy.Context, e caddy.Event) error {
	w.Logger.Debug("handling event",
		zap.String("event_name", e.Name()),
		zap.String("webhook_url", w.URL))
	
	go w.sendWebhook(e)
	
	return nil
}

// HTTP Request
func (w *EventWebhook) sendWebhook(e caddy.Event) {
	var eventName = e.Name()
	var requestBody []byte
	var err error
	
	payload := map[string]interface{}{
		"event": eventName,
		"eventTimestamp": e.Timestamp().Format(time.RFC3339),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if e.Data != nil {
		payload["data"] = e.Data
	}
	
	requestBody, err = json.Marshal(payload)
	if err != nil {
		w.Logger.Error("failed to marshal webhook payload", 
			zap.String("event", eventName),
			zap.Error(err))
		return
	}
	
	client := &http.Client{
		Timeout: time.Duration(w.Timeout),
	}
	
	req, err := http.NewRequest(w.Method, w.URL, bytes.NewBuffer(requestBody))
	if err != nil {
		w.Logger.Error("failed to create webhook request",
			zap.String("event", eventName),
			zap.Error(err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range w.Headers {
		req.Header.Set(key, value)
	}
	
	resp, err := client.Do(req)
	if err != nil {
		w.Logger.Error("failed to send webhook",
			zap.String("event", eventName),
			zap.String("url", w.URL),
			zap.Error(err))
		return
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.Logger.Debug("webhook sent successfully",
			zap.String("event", eventName),
			zap.String("url", w.URL),
			zap.Int("status", resp.StatusCode))
	} else {
		w.Logger.Warn("webhook returned non-2xx status",
			zap.String("event", eventName),
			zap.String("url", w.URL),
			zap.Int("status", resp.StatusCode),
			zap.String("response", string(body)))
	}
}

// UnmarshalCaddyfile은 Caddyfile 설정을 파싱합니다
func (w *EventWebhook) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	// Arg: URL
	if d.NextArg() {
		w.URL = d.Val()
	} else {
		return d.Errf("webhook URL is not configured")
	}
	
	// Parse configuration inside the block
	for d.NextBlock(0) {
		switch d.Val() {
		case "header":
			if !d.NextArg() {
				return d.ArgErr()
			}
			key := d.Val()
			if !d.NextArg() {
				return d.ArgErr()
			}
			value := d.Val()
			w.Headers[key] = value
			
		case "timeout":
			if !d.NextArg() {
				return d.ArgErr()
			}
			dur, err := time.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid timeout duration: %v", err)
			}
			w.Timeout = caddy.Duration(dur)
		default:
			return d.Errf("unrecognized subdirective: %s", d.Val())
		}
	}
	
	if w.URL == "" {
		return d.Errf("webhook URL is required")
	}
	
	return nil
}

// Interface guards
var (
	_ caddy.Provisioner     = (*EventWebhook)(nil)
	_ caddyfile.Unmarshaler = (*EventWebhook)(nil)
	_ caddy.Module          = (*EventWebhook)(nil)
)