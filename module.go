package caddy_events_webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyevents"
	"go.uber.org/zap"
)

type EventsWebhook struct {
	Logger *zap.Logger

	URL string `json:"url,omitempty"`
	Method string `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout caddy.Duration `json:"timeout,omitempty"`
}

func init() {
	caddy.RegisterModule(EventsWebhook{})
}

func (EventsWebhook) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "events.handlers.webhook",
		New: func() caddy.Module { return new(EventsWebhook) },
	}
}

func (ew *EventsWebhook) Provision(ctx caddy.Context) error {
	ew.Logger = ctx.Logger(ew)

	if ew.Method == "" {
		ew.Method = "POST"
	}
	if ew.Timeout == 0 {
		ew.Timeout = caddy.Duration(30 * time.Second)
	}
	if ew.Headers == nil {
		ew.Headers = make(map[string]string)
	}

	ew.Logger.Info("module loaded");
	
	return nil
}

// Caddy Event Handle
func (ew *EventsWebhook) Handle(ctx context.Context, e caddy.Event) error {
	ew.Logger.Debug("handling event",
		zap.String("event_name", e.Name()),
		zap.String("webhook_url", ew.URL))

	go ew.sendWebhook(e)
	
	return nil
}

// HTTP Request
func (ew *EventsWebhook) sendWebhook(e caddy.Event) {
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
		ew.Logger.Error("JSON serialization failed for webhook payload", 
			zap.String("event", eventName),
			zap.Error(err))
		return
	}
	
	client := &http.Client{
		Timeout: time.Duration(ew.Timeout),
	}
	
	req, err := http.NewRequest(ew.Method, ew.URL, bytes.NewBuffer(requestBody))
	if err != nil {
		ew.Logger.Error("failed to create webhook request",
			zap.String("event", eventName),
			zap.Error(err))
		return
	}

	req.Header.Set("User-Agent", "Caddy Event Webhook")
	for key, value := range ew.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		ew.Logger.Error("failed to send webhook",
			zap.String("event", eventName),
			zap.String("url", ew.URL),
			zap.Error(err))
		return
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		ew.Logger.Debug("webhook sent successfully",
			zap.String("event", eventName),
			zap.Int("status", resp.StatusCode),
			zap.String("url", ew.URL))
	} else {
		ew.Logger.Warn("webhook returned non-2xx status",
			zap.String("event", eventName),
			zap.Int("status", resp.StatusCode),
			zap.String("url", ew.URL),
			zap.String("response", string(body)))
	}
}

func (ew *EventsWebhook) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.NextArg()
	// Arg: URL
	if d.NextArg() {
		ew.URL = d.Val()
	} else {
		return d.Errf("webhook URL is not configured")
	}
	
	// Parse configuration inside the block
	for d.NextBlock(0) {
		switch d.Val() {
			case "header":
				if ew.Headers == nil {
					ew.Headers = make(map[string]string)
				}

				if !d.NextArg() {
					return d.ArgErr()
				}
				key := d.Val()
				if !d.NextArg() {
					return d.ArgErr()
				}
				value := d.Val()
				ew.Headers[key] = value
				
			case "timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}
				dur, err := time.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("invalid timeout duration: %v", err)
				}
				ew.Timeout = caddy.Duration(dur)
			default:
				return d.Errf("unrecognized subdirective: %s", d.Val())
		}
	}
	
	if ew.URL == "" {
		return d.Errf("webhook URL is required")
	}
	
	return nil
}

var (
	_ caddy.Module          = (*EventsWebhook)(nil)
	_ caddy.Provisioner     = (*EventsWebhook)(nil)
	_ caddyevents.Handler   = (*EventsWebhook)(nil)
	_ caddyfile.Unmarshaler = (*EventsWebhook)(nil)
)