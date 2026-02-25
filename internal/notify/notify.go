// Package notify provides a notification dispatcher that routes events to configured adapters.
package notify

import (
	"fmt"
	"log"
)

// Sender can send a plain text message.
type Sender interface {
	Send(msg string) error
}

// WebhookFirer can fire a webhook event.
type WebhookFirer interface {
	Fire(event string, payload interface{})
}

// Dispatcher routes notification events to Telegram and webhooks.
type Dispatcher struct {
	telegram Sender
	webhook  WebhookFirer
}

// New creates a Dispatcher. Both telegram and webhook may be nil (disabled).
func New(telegram Sender, webhook WebhookFirer) *Dispatcher {
	return &Dispatcher{telegram: telegram, webhook: webhook}
}

// Send dispatches a notification event to all configured adapters.
func (d *Dispatcher) Send(event string, payload interface{}) {
	msg := formatEvent(event, payload)
	if d.telegram != nil {
		if err := d.telegram.Send(msg); err != nil {
			log.Printf("notify: telegram send: %v", err)
		}
	}
	if d.webhook != nil {
		d.webhook.Fire(event, payload)
	}
}

// SendTelegram sends a message only via Telegram.
func (d *Dispatcher) SendTelegram(msg string) {
	if d.telegram == nil {
		return
	}
	if err := d.telegram.Send(msg); err != nil {
		log.Printf("notify: telegram: %v", err)
	}
}

func formatEvent(event string, payload interface{}) string {
	return fmt.Sprintf("[%s] %v", event, payload)
}