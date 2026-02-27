package common

import (
	"errors"
	"log"
	"time"

	"github.com/friedelschoen/go-wiimote"
	"golang.org/x/sys/unix"
)

// ErrPollAgain is returned by a PollDriver to mark the poll invalid.
var ErrPollAgain = errors.New("invalid polling, should retrying")

// pollerDriver defines a source that can be polled for events or data.
type pollerDriver[T any] interface {
	// FD returns a non-blocking file descriptor. When it becomes readable,
	// Poll() is expected to return data immediately.
	FD() int

	// Poll attempts to retrieve an event or data.
	//
	// Return values:
	//   T:     the retrieved data (invalid if error == ErrRetry)
	//   bool:  indicates whether more data is immediately available without
	//          waiting for I/O readiness
	//   error: nil on success. If ErrRetry is returned, the call should be
	//          repeated without waiting. Any other error aborts the attempt.
	Poll() (T, bool, error)
}

// poller drives a PollMonitor using poll(2) or retry logic.
type poller[T any] struct {
	drv  pollerDriver[T]
	fd   int
	wait bool
}

// Newpoller creates a new poller for the given monitor.
// The poller initially assumes that Poll() should be called without waiting.
func NewPoller[T any](drv pollerDriver[T]) wiimote.Poller[T] {
	return &poller[T]{
		drv: drv,
		fd:  -1,
	}
}

func (p *poller[T]) Poll() (T, bool, error) {
	return p.drv.Poll()
}

func (p *poller[T]) Wait(timeout time.Duration) (T, error) {
	for {
		if p.wait {
			if p.fd == -1 {
				p.fd = p.drv.FD()
			}
			if p.fd >= 0 {
				fds := [1]unix.PollFd{{
					Fd:     int32(p.fd),
					Events: unix.POLLIN,
				}}
				dur := -1
				if timeout >= 0 {
					dur = int(timeout.Milliseconds())
				}
				unix.Poll(fds[:], dur)
			}
		}
		ev, moredata, err := p.drv.Poll()
		p.wait = !moredata
		if errors.Is(err, ErrPollAgain) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return ev, err
	}
}

func (p *poller[T]) drain(yield func(T)) {
	for {
		ev, _, err := p.drv.Poll()
		if errors.Is(err, ErrPollAgain) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if err != nil {
			log.Printf("error while polling for event: %v", err)
			continue
		}
		yield(ev)
	}
}

func (p *poller[T]) Handle(yield func(T)) {
	p.drain(yield)
	for {
		if p.fd == -1 {
			p.fd = p.drv.FD()
		}
		if p.fd >= 0 {
			fds := [...]unix.PollFd{{
				Fd:     int32(p.fd),
				Events: unix.POLLIN,
			}}
			unix.Poll(fds[:], -1)
		} else {
			time.Sleep(100 * time.Millisecond)
		}
		p.drain(yield)
	}
}

func (p *poller[T]) Stream(ch chan<- T) {
	p.Handle(func(ev T) {
		ch <- ev
	})
}
