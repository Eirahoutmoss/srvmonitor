// Package alert evaluates flattened metric readings against rules and decides
// when to raise, escalate, or clear an alarm.
//
// The engine is deliberately free of I/O and wall-clock calls: every method
// takes the current time as an argument, so behaviour (for-duration,
// cooldown, escalation, recovery) is fully unit-testable.
package alert

import (
	"strings"
	"time"
)

// Level is the severity of a reading.
type Level int

const (
	OK Level = iota
	Warn
	Crit
)

// MarshalJSON renders a level as its Turkish name in API responses.
func (l Level) MarshalJSON() ([]byte, error) {
	return []byte(`"` + l.String() + `"`), nil
}

func (l Level) String() string {
	switch l {
	case Crit:
		return "KRİTİK"
	case Warn:
		return "UYARI"
	default:
		return "NORMAL"
	}
}

// Comparator says which direction counts as a breach.
type Comparator string

const (
	// GT: a reading above the threshold is a breach (CPU, disk fill).
	GT Comparator = "gt"
	// LT: a reading below the threshold is a breach (free space, DB up=0).
	LT Comparator = "lt"
)

// Rule describes one condition to watch. Key matches a metric name and may
// contain a single "*" wildcard segment to cover every disk or tablespace
// (e.g. "disk.*.percent").
type Rule struct {
	ID       string        `json:"id"`
	Desc     string        `json:"desc"`
	Key      string        `json:"key"`
	Compare  Comparator    `json:"compare"`
	Warn     float64       `json:"warn"`
	Crit     float64       `json:"crit"`
	For      time.Duration `json:"-"`
	Cooldown time.Duration `json:"-"`
}

// level classifies a single value against the rule's thresholds.
func (r Rule) level(v float64) Level {
	switch r.Compare {
	case LT:
		if v < r.Crit {
			return Crit
		}
		if v < r.Warn {
			return Warn
		}
	default: // GT
		if v > r.Crit {
			return Crit
		}
		if v > r.Warn {
			return Warn
		}
	}
	return OK
}

// Matches reports whether a concrete metric key is covered by the rule's
// key pattern, and if so returns the wildcard-captured instance (or "").
func (r Rule) Matches(key string) (instance string, ok bool) {
	if !strings.Contains(r.Key, "*") {
		return "", key == r.Key
	}
	pre, post, _ := strings.Cut(r.Key, "*")
	if !strings.HasPrefix(key, pre) || !strings.HasSuffix(key, post) {
		return "", false
	}
	mid := key[len(pre) : len(key)-len(post)]
	if mid == "" || strings.Contains(mid, ".") {
		return "", false // "*" spans exactly one segment
	}
	return mid, true
}

// Event is emitted when an alarm is raised, escalated or cleared.
type Event struct {
	RuleID    string
	Desc      string
	Key       string
	Level     Level // new level (OK means recovery)
	Previous  Level // level before this event
	Value     float64
	Threshold float64
	Time      time.Time
}

// Recovered reports whether this event is a return to normal.
func (e Event) Recovered() bool { return e.Level == OK }

type state struct {
	level    Level     // last classified level
	since    time.Time // when the current non-OK level began being observed
	fired    Level     // last level we actually emitted an event for
	lastFire time.Time
}

// Engine tracks per-metric state across successive evaluations.
type Engine struct {
	rules  []Rule
	states map[string]*state // keyed by ruleID + "\x00" + concrete metric key
}

// New builds an engine for the given rules.
func New(rules []Rule) *Engine {
	return &Engine{rules: rules, states: map[string]*state{}}
}

// Evaluate feeds one full set of readings to the engine and returns any
// alarm events that occurred at time now.
func (e *Engine) Evaluate(values map[string]float64, now time.Time) []Event {
	var events []Event
	for _, r := range e.rules {
		for key, v := range values {
			if _, ok := r.Matches(key); !ok {
				continue
			}
			if ev, emit := e.step(r, key, v, now); emit {
				events = append(events, ev)
			}
		}
	}
	return events
}

func (e *Engine) step(r Rule, key string, v float64, now time.Time) (Event, bool) {
	id := r.ID + "\x00" + key
	st := e.states[id]
	if st == nil {
		st = &state{level: OK, fired: OK}
		e.states[id] = st
	}

	lvl := r.level(v)

	// Track when the observed level last changed, for the For-duration gate.
	if lvl != st.level {
		st.level = lvl
		st.since = now
	}

	threshold := r.Warn
	if lvl == Crit {
		threshold = r.Crit
	}
	mk := func(l, prev Level) Event {
		return Event{RuleID: r.ID, Desc: r.Desc, Key: key, Level: l,
			Previous: prev, Value: v, Threshold: threshold, Time: now}
	}

	// Recovery: we had fired a breach and are now OK.
	if lvl == OK {
		if st.fired != OK {
			prev := st.fired
			st.fired = OK
			return mk(OK, prev), true
		}
		return Event{}, false
	}

	// Breaching. Require the level to persist for the For duration before
	// firing (unless For is zero).
	if r.For > 0 && now.Sub(st.since) < r.For {
		return Event{}, false
	}

	// Escalation (Warn -> Crit) always fires immediately.
	if lvl > st.fired {
		prev := st.fired
		st.fired = lvl
		st.lastFire = now
		return mk(lvl, prev), true
	}

	// Same level already fired: re-fire only after the cooldown elapses.
	if lvl == st.fired && r.Cooldown > 0 && now.Sub(st.lastFire) >= r.Cooldown {
		st.lastFire = now
		return mk(lvl, lvl), true
	}

	return Event{}, false
}

// Active returns the current non-OK level for every tracked metric.
func (e *Engine) Active() map[string]Level {
	out := map[string]Level{}
	for id, st := range e.states {
		if st.fired != OK {
			_, key, _ := strings.Cut(id, "\x00")
			out[key] = st.fired
		}
	}
	return out
}
