// Copyright 2020 Joshua J Baker. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package evio

import (
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

// Action that occurs after the completion of an event.
type Action int

const (
	// None indicates that no action should occur following an event.
	None Action = iota
	// Close the connection.
	Close
	// Shutdown the server.
	Shutdown
)

// Server ...
type Server struct {
	// The addrs parameter is an array of listening addresses that align
	// with the addr strings passed to the Serve function.
	Addrs []net.Addr
}

// Conn ...
type Conn interface {
	// Context returns a user-defined context.
	Context() interface{}
	// SetContext sets a user-defined context.
	SetContext(interface{})
	// AddrIndex is the index of server addr that was passed to the Serve call.
	AddrIndex() int
	// LocalAddr is the connection's local socket address.
	LocalAddr() net.Addr
	// RemoteAddr is the connection's remote peer address.
	RemoteAddr() net.Addr
	// Write data to connection.
	Write(data []byte)
	// Close the connection.
	Close()
}

// Events ...
type Events struct {
	// Serving fires when the server can accept connections. The server
	// parameter has information and various utilities.
	Serving func(server Server) (action Action)
	// Opened fires when a new connection has opened.
	// The info parameter has information about the connection such as
	// it's local and remote address.
	// Use the out return value to write data to the connection.
	Opened func(c Conn) (out []byte, action Action)
	// Closed fires when a connection has closed.
	// The err parameter is the last known connection error.
	Closed func(c Conn) (action Action)
	// PreWrite fires just before any data is written to any client socket.
	PreWrite func()
	// Data fires when a connection sends the server data.
	// The in parameter is the incoming data.
	// Use the out return value to write data to the connection.
	Data func(c Conn, in []byte) (out []byte, action Action)
	// Tick fires immediately after the server starts and will fire again
	// following the duration specified by the delay return value.
	Tick func(now time.Time) (delay time.Duration, action Action)
}

// conn ...
type conn struct {
	write  bool             // connection requesting write events
	fd     int              // file descriptor
	oidx   int              // output write index
	out    []byte           // output buffer
	action Action           // last known action
	ctx    interface{}      // user-defined context
	poll   *poll            // connection poll
	raddr  net.Addr         // remote address
	laddr  net.Addr         // local address
	saddr  int              // index of server address
	sa     syscall.Sockaddr // socket address of fd
}

func (c *conn) Close() {
	if c.poll == nil {
		return
	}
	if c.action == None {
		c.action = Close
	}
	if !c.write {
		c.poll.modReadWrite(c.fd)
		c.write = true
	}
}

func (c *conn) Write(data []byte) {
	if c.poll == nil {
		return
	}
	if c.action == None {
		c.out = append(c.out, data...)
		if !c.write {
			c.poll.modReadWrite(c.fd)
			c.write = true
		}
	}
}

func (c *conn) SetContext(ctx interface{}) { c.ctx = ctx }
func (c *conn) Context() interface{}       { return c.ctx }
func (c *conn) AddrIndex() int             { return c.saddr }
func (c *conn) LocalAddr() net.Addr        { return c.laddr }
func (c *conn) RemoteAddr() net.Addr {
	if c.raddr == nil {
		switch sa := c.sa.(type) {
		case *syscall.SockaddrInet4:
			c.raddr = &net.TCPAddr{
				IP:   append([]byte{}, sa.Addr[:]...),
				Port: sa.Port,
			}
		case *syscall.SockaddrInet6:
			var zone string
			if sa.ZoneId != 0 {
				ifi, err := net.InterfaceByIndex(int(sa.ZoneId))
				if err == nil {
					zone = ifi.Name
				}
			}
			c.raddr = &net.TCPAddr{
				IP:   append([]byte{}, sa.Addr[:]...),
				Port: sa.Port,
				Zone: zone,
			}
		case *syscall.SockaddrUnix:
			c.raddr = &net.UnixAddr{Net: "unix", Name: sa.Name}
		}
	}
	return c.raddr
}

// Serve ...
func Serve(events Events, addr ...string) error {
	var lns []net.Listener
	var lfs []*os.File
	var lfds []int
	defer func() {
		for i := range lns {
			syscall.Close(lfds[i])
			lfs[i].Close()
			lns[i].Close()
		}
	}()

	p := newPoll()

	for _, address := range addr {
		network := "tcp"
		if strings.Contains(address, "://") {
			network = strings.Split(address, "://")[0]
			address = strings.Split(address, "://")[1]
		}
		if network == "unix" {
			os.RemoveAll(address)
		}
		ln, err := net.Listen(network, address)
		if err != nil {
			return err
		}
		var lnf *os.File
		switch netln := ln.(type) {
		case *net.TCPListener:
			lnf, err = netln.File()
		case *net.UnixListener:
			lnf, err = netln.File()
		}
		if err != nil {
			ln.Close()
			return err
		}
		lfd := int(lnf.Fd())
		lns = append(lns, ln)
		lfs = append(lfs, lnf)
		lfds = append(lfds, lfd)
		if err := syscall.SetNonblock(lfd, true); err != nil {
			return err
		}
		p.addRead(lfd)
	}

	conns := make(map[int]*conn)
	defer func() {
		for cfd, c := range conns {
			c.poll = nil
			syscall.Close(cfd)
			if events.Closed != nil {
				events.Closed(c)
			}

		}
	}()
	if events.Serving != nil {
		var s Server
		for _, ln := range lns {
			s.Addrs = append(s.Addrs, ln.Addr())
		}
		if events.Serving(s) == Shutdown {
			return nil
		}
	}
	var lastTick time.Time
	var delay time.Duration = -1
	if events.Tick != nil {
		delay = 0
	}

	packet := make([]byte, 4096)
	var shutdown bool
	for !shutdown {
		fds := p.wait(delay)
	nextfd:
		for _, fd := range fds {
			for i, lfd := range lfds {
				if lfd == fd {
					fd, sa, err := syscall.Accept(lfd)
					if err != nil {
						if err == syscall.EAGAIN {
							continue nextfd
						}
						panic(err)
					}
					if _, ok := lns[i].(*net.TCPListener); ok {
						if err := setKeepAlive(fd, 300); err != nil {
							syscall.Close(fd)
							continue nextfd
						}
					}
					if err := syscall.SetNonblock(fd, true); err != nil {
						syscall.Close(fd)
						continue nextfd
					}
					p.addRead(fd)
					c := &conn{fd: fd, sa: sa, poll: p, saddr: i,
						laddr: lns[i].Addr()}
					conns[c.fd] = c
					if events.Opened != nil {
						out, action := events.Opened(c)
						if len(out) > 0 || action != None {
							c.out = append(c.out, out...)
							c.action = action
							c.write = true
							p.modReadWrite(fd)
						}
					}
					continue nextfd
				}
			}
			c := conns[fd]
			if len(c.out)-c.oidx > 0 {
				if events.PreWrite != nil {
					events.PreWrite()
				}
				for {
					n, err := syscall.Write(c.fd, c.out[c.oidx:])
					if err != nil {
						if err != syscall.EAGAIN {
							if c.action < Close {
								c.action = Close
							}
							break
						}
					}
					c.oidx += n
					if c.oidx < len(c.out) {
						continue
					}
					break
				}
				c.oidx = 0
				if cap(c.out) > 4096 {
					c.out = nil
				} else {
					c.out = c.out[:0]
				}
				if c.action == None {
					c.write = false
					p.modRead(c.fd)
				}
			} else if c.action >= Close {
				c.poll = nil
				syscall.Close(c.fd)
				delete(conns, c.fd)
				if events.Closed != nil {
					action := events.Closed(c)
					if c.action == Shutdown || action == Shutdown {
						shutdown = true
						break
					}
				}
			} else {
				n, err := syscall.Read(c.fd, packet[:])
				if err != nil || n == 0 {
					if err != syscall.EAGAIN {
						c.action = Close
					}
					continue
				}
				if events.Data != nil {
					out, action := events.Data(c, packet[:n])
					if len(out) > 0 || action != None {
						c.out = append(c.out, out...)
						c.action = action
						c.write = true
						p.modReadWrite(fd)
					}
				}
			}
		}
		if events.Tick != nil {
			now := time.Now()
			if now.Sub(lastTick) > delay {
				var action Action
				lastTick = now
				delay, action = events.Tick(now)
				if delay < 0 {
					delay = 0
				}
				if action == Shutdown {
					return nil
				}
			}
		}
	}
	return nil
}
