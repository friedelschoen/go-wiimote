package commonhid

import (
	"iter"
	"testing"
	"time"
)

type testFeature struct {
	commonFeature

	reqs []modeRequirement
}

func (f *testFeature) acceptEvent(ts time.Time, report []byte) {}
func (f *testFeature) requireModes() []modeRequirement         { return f.reqs }

func seqOf(fs ...feature) iter.Seq[feature] {
	return func(yield func(feature) bool) {
		for _, f := range fs {
			if !yield(f) {
				return
			}
		}
	}
}

func TestRunecount(t *testing.T) {
	t.Parallel()

	if got := runecount("bbaaa", 'b'); got != 2 {
		t.Fatalf("runecount(..., 'b') = %d, want 2", got)
	}
	if got := runecount("bbaaa", 'a'); got != 3 {
		t.Fatalf("runecount(..., 'a') = %d, want 3", got)
	}
	if got := runecount("bbaaa", 'e'); got != 0 {
		t.Fatalf("runecount(..., 'e') = %d, want 0", got)
	}
}

func TestModeMatches(t *testing.T) {
	t.Parallel()

	req := modeRequirement{feature: 'i', min: 10, max: 12}

	if !modeMatches("bbiiiiiiiiiieeeeeeeee", req) {
		t.Fatalf("modeMatches returned false, want true")
	}
	if modeMatches("bbaaa", req) {
		t.Fatalf("modeMatches returned true, want false")
	}
}

func TestFindModeNoMatchReturnsZero(t *testing.T) {
	t.Parallel()

	/*
		buttons + accel + ir + exactly 9 expansion bytes:
		there is no single mode that satisfies all of these.
	*/
	mode := findMode(seqOf(
		&testFeature{reqs: []modeRequirement{{feature: 'b', min: 2, max: 3}}},
		&testFeature{reqs: []modeRequirement{{feature: 'a', min: 3, max: 3}}},
		&testFeature{reqs: []modeRequirement{{feature: 'i', min: 10, max: 16}}},
		&testFeature{reqs: []modeRequirement{{feature: 'e', min: 9, max: 9}}},
	))
	if mode != 0 {
		t.Fatalf("findMode(...) = 0x%02x, want 0x00", mode)
	}
}
