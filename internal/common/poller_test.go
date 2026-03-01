package common

import (
	"errors"
	"sync"
	"testing"
)

type pollStep[T any] struct {
	ev   T
	cont bool
	err  error
}

type fakeDriver[T any] struct {
	mu        sync.Mutex
	fd        int
	fdCalls   int
	pollCalls int
	steps     []pollStep[T]
}

func (d *fakeDriver[T]) FD() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fdCalls++
	return d.fd
}

func (d *fakeDriver[T]) Poll() (T, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var zero T
	d.pollCalls++

	if len(d.steps) == 0 {
		return zero, false, errors.New("no more steps")
	}
	s := d.steps[0]
	d.steps = d.steps[1:]
	return s.ev, s.cont, s.err
}

func TestPollerWait_ContKeepsDontWaitTrue(t *testing.T) {
	d := &fakeDriver[int]{
		fd: -1,
		steps: []pollStep[int]{
			{ev: 1, cont: true, err: nil}, // cont=true => dontwait blijft true
			{ev: 2, cont: false, err: nil},
		},
	}
	p := NewPoller(d)

	ev, err := p.Wait(0)
	if err != nil || ev != 1 {
		t.Fatalf("first Wait: expected (1,nil), got (%v,%v)", ev, err)
	}

	// Als dontwait true blijft, mag FD() niet aangeroepen worden.
	if d.fdCalls != 0 {
		t.Fatalf("expected FD() not called yet, got %d", d.fdCalls)
	}

	ev, err = p.Wait(0)
	if err != nil || ev != 2 {
		t.Fatalf("second Wait: expected (2,nil), got (%v,%v)", ev, err)
	}
	if d.fdCalls != 0 {
		t.Fatalf("expected FD() still not called, got %d", d.fdCalls)
	}
}
