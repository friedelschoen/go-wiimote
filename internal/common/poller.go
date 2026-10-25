package common

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

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
	//   error: nil on success. ErrWouldBlock means "no event right now".
	//          Any other error aborts the attempt.
	PollEvent() (T, error)
}

// poller drives a PollMonitor using poll(2) or retry logic.
type poller[T any] struct {
	drv pollerDriver[T]
	fd  int
}

// NewPoller creates a new poller for the given driver.
// The poller initially assumes Poll() should be called without waiting.
func NewPoller[T any](drv pollerDriver[T]) wiimote.Poller[T] {
	return &poller[T]{drv: drv, fd: -1}
}

func (p *poller[T]) PollEvent() (T, error) {
	return p.drv.PollEvent()
}

// WaitReadable waits until the driver FD is readable or a timeout passes.
// timeout < 0 means "wait forever".
func (p *poller[T]) waitReadable(ctx context.Context) error {
	if p.fd < 0 {
		p.fd = p.drv.FD()
	}
	if p.fd < 0 {
		return errors.New("driver has no pollable file descriptor")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	cancelFD, err := unix.Eventfd(
		0,
		unix.EFD_CLOEXEC|unix.EFD_NONBLOCK,
	)
	if err != nil {
		return fmt.Errorf("eventfd: %w", err)
	}
	defer unix.Close(cancelFD)

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			var buf [8]byte
			binary.NativeEndian.PutUint64(buf[:], 1)
			_, _ = unix.Write(cancelFD, buf[:])

		case <-done:
		}
	}()

	fds := []unix.PollFd{
		{
			Fd:     int32(p.fd),
			Events: unix.POLLIN,
		},
		{
			Fd:     int32(cancelFD),
			Events: unix.POLLIN,
		},
	}

	for {
		_, err := unix.Poll(fds, -1)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err
		}

		if fds[1].Revents&unix.POLLIN != 0 {
			return ctx.Err()
		}

		revents := fds[0].Revents

		if revents&unix.POLLNVAL != 0 {
			return fmt.Errorf("poll: invalid fd")
		}

		if revents&unix.POLLERR != 0 {
			return fmt.Errorf("poll: device error")
		}

		if revents&unix.POLLHUP != 0 {
			return fmt.Errorf("poll: device disconnected")
		}

		if revents&unix.POLLIN != 0 {
			return nil
		}
	}
}

func (p *poller[T]) NextEvent(ctx context.Context) (T, error) {
	for {
		ev, err := p.drv.PollEvent()

		switch {
		case err == nil:
			return ev, nil

		case !errors.Is(err, ErrWouldBlock):
			return ev, err
		}

		if err := p.waitReadable(ctx); err != nil {
			var zero T
			return zero, err
		}
	}
}
