// Copyright 2020 Joshua J Baker. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build linux

package evio

import (
	"syscall"
	"time"
)

type poll struct {
	fd     int
	events []syscall.EpollEvent
	evfds  []int
}

func newPoll() *poll {
	fd, err := syscall.EpollCreate1(0)
	if err != nil {
		panic(err)
	}
	p := new(poll)
	p.fd = fd
	p.events = make([]syscall.EpollEvent, 64)
	p.evfds = make([]int, 0, len(p.evfds))
	return p
}

func (p *poll) addRead(fd int) {
	if err := syscall.EpollCtl(p.fd, syscall.EPOLL_CTL_ADD, fd,
		&syscall.EpollEvent{Fd: int32(fd),
			Events: syscall.EPOLLIN,
		},
	); err != nil {
		panic(err)
	}
}

func (p *poll) modReadWrite(fd int) {
	if err := syscall.EpollCtl(p.fd, syscall.EPOLL_CTL_MOD, fd,
		&syscall.EpollEvent{Fd: int32(fd),
			Events: syscall.EPOLLIN | syscall.EPOLLOUT,
		},
	); err != nil {
		panic(err)
	}
}

func (p *poll) modRead(fd int) {
	if err := syscall.EpollCtl(p.fd, syscall.EPOLL_CTL_MOD, fd,
		&syscall.EpollEvent{Fd: int32(fd),
			Events: syscall.EPOLLIN,
		},
	); err != nil {
		panic(err)
	}
}

// A negative timout is forever.
func (p *poll) wait(timeout time.Duration) []int {
	var n int
	var err error
	if timeout >= 0 {
		n, err = syscall.EpollWait(p.fd, p.events,
			int(timeout/time.Millisecond))
	} else {
		n, err = syscall.EpollWait(p.fd, p.events, -1)
	}
	if err != nil && err != syscall.EINTR {
		panic(err)
	}
	p.evfds = p.evfds[:0]
	for i := 0; i < n; i++ {
		p.evfds = append(p.evfds, int(p.events[i].Fd))
	}
	return p.evfds
}

func setKeepAlive(fd, secs int) error {
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET,
		syscall.SO_KEEPALIVE, 1); err != nil {
		return err
	}
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP,
		syscall.TCP_KEEPINTVL, secs); err != nil {
		return err
	}
	return syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_KEEPIDLE,
		secs)
}
