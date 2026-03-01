package common

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/friedelschoen/go-wiimote"
	"golang.org/x/sys/unix"
)

// ErrWouldBlock means: Poll() has no event available *right now*.
// The poller should wait for readability and retry.
var ErrWouldBlock = errors.New("would block; wait readable and retry")

// pollerDriver defines a source that can be polled for events or data.
type pollerDriver[T any] interface {
	// FD returns a non-blocking file descriptor. When it becomes readable,
	// Poll() is expected to return data immediately.
	FD() int

	// Poll attempts to retrieve an event or data without blocking.
	//
	// Return values:
	//   T:     the retrieved data (invalid if err == ErrWouldBlock)
	//   bool:  indicates whether more data is immediately available without waiting
	//   error: nil on success. ErrWouldBlock means "no event right now".
	//          Any other error aborts the attempt.
	Poll() (T, bool, error)
}

// poller drives a PollMonitor using poll(2) or retry logic.
type poller[T any] struct {
	drv  pollerDriver[T]
	fd   int
	wait bool
}

// NewPoller creates a new poller for the given driver.
// The poller initially assumes Poll() should be called without waiting.
func NewPoller[T any](drv pollerDriver[T]) wiimote.Poller[T] {
	return &poller[T]{drv: drv, fd: -1}
}

func (p *poller[T]) Poll() (T, bool, error) {
	return p.drv.Poll()
}

// WaitReadable waits until the driver FD is readable or a timeout passes.
// timeout < 0 means "wait forever".
func (p *poller[T]) WaitReadable(timeout time.Duration) error {
	if p.fd < 0 {
		p.fd = p.drv.FD()
	}
	if p.fd < 0 {
		// Driver does not provide an FD; caller must rely on retry.
		return nil
	}

	fds := []unix.PollFd{{
		Fd:     int32(p.fd),
		Events: unix.POLLIN,
	}}

	ms := -1
	if timeout >= 0 {
		ms = int(timeout.Milliseconds())
	}

	for {
		n, err := unix.Poll(fds, ms)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				// interrupted by signal; retry
				continue
			}
			return err
		}

		// timeout
		if n == 0 {
			return nil
		}

		re := fds[0].Revents
		if re&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			return fmt.Errorf("poll revents=%#x", re)
		}
		// POLLIN (or friends) means: readable
		return nil
	}
}

func (p *poller[T]) Wait(timeout time.Duration) (T, error) {
	for {
		if p.wait {
			if err := p.WaitReadable(timeout); err != nil {
				var zero T
				return zero, err
			}
		}

		ev, more, err := p.drv.Poll()
		switch {
		case err == nil:
			p.wait = !more
			return ev, nil

		case errors.Is(err, ErrWouldBlock):
			// nothing available now; next iteration should wait for readability.
			p.wait = true
			continue

		default:
			// hard error
			var zero T
			return zero, err
		}
	}
}

func (p *poller[T]) drain(yield func(T)) {
	for {
		ev, more, err := p.drv.Poll()
		switch {
		case err == nil:
			yield(ev)
			if !more {
				return
			}
			continue

		case errors.Is(err, ErrWouldBlock):
			return

		default:
			log.Printf("error while polling for event: %v", err)
			return
		}
	}
}

func (p *poller[T]) Handle(yield func(T)) error {
	for {
		p.drain(yield)
		if err := p.WaitReadable(-1); err != nil {
			return err
		}
	}
}

func (p *poller[T]) Stream(ch chan<- T) {
	p.Handle(func(ev T) { ch <- ev })
}
