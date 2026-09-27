package alert

import (
	"testing"
	"time"
)

func rule() Rule {
	return Rule{ID: "cpu", Desc: "CPU", Key: "cpu.percent", Compare: GT,
		Warn: 80, Crit: 90, For: 0, Cooldown: 5 * time.Minute}
}

func TestLevels(t *testing.T) {
	r := rule()
	cases := []struct {
		v    float64
		want Level
	}{{50, OK}, {80, OK}, {80.1, Warn}, {90, Warn}, {90.1, Crit}}
	for _, c := range cases {
		if got := r.level(c.v); got != c.want {
			t.Errorf("level(%v)=%v want %v", c.v, got, c.want)
		}
	}
}

func TestLTComparator(t *testing.T) {
	r := Rule{Compare: LT, Warn: 20, Crit: 10} // free space %
	if r.level(25) != OK || r.level(15) != Warn || r.level(5) != Crit {
		t.Fatal("LT classification wrong")
	}
}

func TestRaiseEscalateRecover(t *testing.T) {
	e := New([]Rule{rule()})
	t0 := time.Now()

	// warn
	ev := e.Evaluate(map[string]float64{"cpu.percent": 85}, t0)
	if len(ev) != 1 || ev[0].Level != Warn || ev[0].Previous != OK {
		t.Fatalf("expected warn raise, got %+v", ev)
	}
	// still warn, within cooldown -> no event
	if ev := e.Evaluate(map[string]float64{"cpu.percent": 86}, t0.Add(time.Minute)); len(ev) != 0 {
		t.Fatalf("expected no repeat, got %+v", ev)
	}
	// escalate to crit immediately despite cooldown
	ev = e.Evaluate(map[string]float64{"cpu.percent": 95}, t0.Add(2*time.Minute))
	if len(ev) != 1 || ev[0].Level != Crit || ev[0].Previous != Warn {
		t.Fatalf("expected escalation to crit, got %+v", ev)
	}
	// recovery
	ev = e.Evaluate(map[string]float64{"cpu.percent": 10}, t0.Add(3*time.Minute))
	if len(ev) != 1 || !ev[0].Recovered() || ev[0].Previous != Crit {
		t.Fatalf("expected recovery, got %+v", ev)
	}
	// staying ok -> silence
	if ev := e.Evaluate(map[string]float64{"cpu.percent": 12}, t0.Add(4*time.Minute)); len(ev) != 0 {
		t.Fatalf("expected silence after recovery, got %+v", ev)
	}
}

func TestForDurationGate(t *testing.T) {
	r := rule()
	r.For = 2 * time.Minute
	e := New([]Rule{r})
	t0 := time.Now()
	if ev := e.Evaluate(map[string]float64{"cpu.percent": 95}, t0); len(ev) != 0 {
		t.Fatalf("should not fire before For elapses, got %+v", ev)
	}
	if ev := e.Evaluate(map[string]float64{"cpu.percent": 95}, t0.Add(90*time.Second)); len(ev) != 0 {
		t.Fatalf("still within For, got %+v", ev)
	}
	ev := e.Evaluate(map[string]float64{"cpu.percent": 95}, t0.Add(2*time.Minute))
	if len(ev) != 1 || ev[0].Level != Crit {
		t.Fatalf("should fire after For, got %+v", ev)
	}
}

func TestCooldownRepeat(t *testing.T) {
	e := New([]Rule{rule()})
	t0 := time.Now()
	e.Evaluate(map[string]float64{"cpu.percent": 85}, t0)
	if ev := e.Evaluate(map[string]float64{"cpu.percent": 85}, t0.Add(5*time.Minute)); len(ev) != 1 {
		t.Fatalf("expected re-fire after cooldown, got %+v", ev)
	}
}

func TestWildcardMatch(t *testing.T) {
	r := Rule{ID: "disk", Key: "disk.*.percent", Compare: GT, Warn: 80, Crit: 90}
	if _, ok := r.Matches("disk.C:.percent"); !ok {
		t.Error("should match disk.C:.percent")
	}
	if inst, _ := r.Matches("disk.C:.percent"); inst != "C:" {
		t.Errorf("instance = %q want C:", inst)
	}
	if _, ok := r.Matches("disk.a.b.percent"); ok {
		t.Error("wildcard must not span segments")
	}
	if _, ok := r.Matches("mem.percent"); ok {
		t.Error("should not match unrelated key")
	}
}

func TestWildcardIndependentState(t *testing.T) {
	r := Rule{ID: "disk", Desc: "Disk", Key: "disk.*.percent", Compare: GT, Warn: 80, Crit: 90}
	e := New([]Rule{r})
	t0 := time.Now()
	ev := e.Evaluate(map[string]float64{"disk.C:.percent": 95, "disk.D:.percent": 50}, t0)
	if len(ev) != 1 || ev[0].Key != "disk.C:.percent" {
		t.Fatalf("only C: should fire, got %+v", ev)
	}
	if len(e.Active()) != 1 {
		t.Fatalf("one active alarm expected, got %v", e.Active())
	}
}

func TestActive(t *testing.T) {
	e := New([]Rule{rule()})
	e.Evaluate(map[string]float64{"cpu.percent": 95}, time.Now())
	act := e.Active()
	if act["cpu.percent"] != Crit {
		t.Fatalf("active = %v", act)
	}
}
