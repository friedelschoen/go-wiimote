package linuxkernel

import (
	"testing"
	"time"
)

func testCTimeRoundtrip(t *testing.T, orig time.Time) {
	next := cTime(cTimeMake(orig))

	if next.Sub(orig) > 1*time.Millisecond {
		t.Errorf("cTime and cTimeMake do not convert equally: expected %v, got %v", orig, next)
	}
}

func TestCTime(t *testing.T) {
	testCTimeRoundtrip(t, time.Time{})
	testCTimeRoundtrip(t, time.Now())
}
