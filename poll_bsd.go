// Copyright 2020 Joshua J Baker. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build darwin netbsd freebsd openbsd dragonfly

package evio

import (
	"syscall"
	"time"
)

type poll struct {
	fd      int
	changes []syscall.Kevent_t
	events  []syscall.Kevent_t
	evfds   []int
}

func newPoll() *poll {
	fd, err := syscall.Kqueue()
	if err != nil {
		panic(err)
	}
	p := new(poll)
	p.fd = fd
	p.events = make([]syscall.Kevent_t, 64)
	p.evfds = make([]int, 0, len(p.evfds))
	p.changes = make([]syscall.Kevent_t, 0, len(p.evfds))
	return p
}

func (p *poll) addRead(fd int) {
	p.changes = append(p.changes, syscall.Kevent_t{Ident: uint64(fd),
		Flags: syscall.EV_ADD, Filter: syscall.EVFILT_READ})
}

func (p *poll) modReadWrite(fd int) {
	p.changes = append(p.changes, syscall.Kevent_t{Ident: uint64(fd),
		Flags: syscall.EV_ADD, Filter: syscall.EVFILT_WRITE})
}

func (p *poll) modRead(fd int) {
	p.changes = append(p.changes, syscall.Kevent_t{Ident: uint64(fd),
		Flags: syscall.EV_DELETE, Filter: syscall.EVFILT_WRITE})
}

// A negative timout is forever.
func (p *poll) wait(timeout time.Duration) []int {
	var n int
	var err error
	if timeout >= 0 {
		var ts syscall.Timespec
		ts.Nsec = int64(timeout)
		n, err = syscall.Kevent(p.fd, p.changes, p.events, &ts)
	} else {
		n, err = syscall.Kevent(p.fd, p.changes, p.events, nil)
	}
	if err != nil && err != syscall.EINTR {
		panic(err)
	}
	p.changes = p.changes[:0]
	p.evfds = p.evfds[:0]
	for i := 0; i < n; i++ {
		p.evfds = append(p.evfds, int(p.events[i].Ident))
	}
	return p.evfds
}

func setKeepAlive(fd, secs int) error {
	// just rely on system tcp keep alives
	return nil
}
