// Package notify delivers alarm notifications over e-mail and SMS.
package notify

import (
	"log"
	"reflect"
	"strings"

	"srvmon/internal/alert"
	"srvmon/internal/format"
)

// Message is a rendered, channel-agnostic notification.
type Message struct {
	Severity string
	Subject  string
	Body     string
}

// FromEvent builds a human-readable message from an alarm event.
func FromEvent(server string, e alert.Event) Message {
	sev := e.Level.String()
	verb := "eşiği aştı"
	if e.Recovered() {
		sev = "NORMAL"
		return Message{
			Severity: sev,
			Subject:  "[NORMALE DÖNDÜ] " + server + " · " + e.Desc,
			Body: server + " sunucusunda \"" + e.Desc + "\" durumu normale döndü.\n" +
				"Metrik: " + e.Key + "\nDeğer: " + format.Percent(e.Value),
		}
	}
	return Message{
		Severity: sev,
		Subject:  "[" + sev + "] " + server + " · " + e.Desc,
		Body: server + " sunucusunda alarm.\n" +
			"Durum: " + sev + "\nAçıklama: " + e.Desc + " " + verb + "\n" +
			"Metrik: " + e.Key + "\nDeğer: " + format.Percent(e.Value) +
			"\nEşik: " + format.Percent(e.Threshold) +
			"\nZaman: " + e.Time.Format("2006-01-02 15:04:05"),
	}
}

// Channel sends one rendered message.
type Channel interface {
	Name() string
	Send(Message) error
}

// Router dispatches a message to the channels a rule selected.
type Router struct {
	channels map[string]Channel
}

// NewRouter builds a router from the available channels. Both a plain nil and
// a typed-nil pointer wrapped in the interface are skipped, so a disabled
// channel (which its constructor returns as a nil *Email/*SMS) never registers.
func NewRouter(chs ...Channel) *Router {
	m := map[string]Channel{}
	for _, c := range chs {
		if c == nil {
			continue
		}
		if rv := reflect.ValueOf(c); rv.Kind() == reflect.Pointer && rv.IsNil() {
			continue
		}
		m[c.Name()] = c
	}
	return &Router{channels: m}
}

// Dispatch sends msg to each named channel that exists and is enabled.
func (r *Router) Dispatch(names []string, msg Message) {
	for _, n := range names {
		n = strings.TrimSpace(strings.ToLower(n))
		ch, ok := r.channels[n]
		if !ok {
			continue
		}
		if err := ch.Send(msg); err != nil {
			log.Printf("bildirim gönderilemedi (%s): %v", n, err)
		} else {
			log.Printf("bildirim gönderildi (%s): %s", n, msg.Subject)
		}
	}
}
