// Copyright 2020 Joshua J Baker. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package evio

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestEvioLite(t *testing.T) {
	addr := ":9991"
	var opened int
	var events Events
	events.Serving = func(s Server) (action Action) {
		go func() {
			c, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			var data [64]byte
			n, _ := c.Read(data[:])
			if string(data[:n]) != "HI THERE" {
				t.Fatalf("expected '%s', got '%s'", " THERE", data[:n])
			}
			c.Write([]byte("HELLO"))
			n, _ = c.Read(data[:])
			if string(data[:n]) != "HELLO" {
				t.Fatalf("expected '%s', got '%s'", "HELLO", data[:n])
			}
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := net.Dial("tcp", addr)
				if err != nil {
					t.Fatal(err)
				}
				defer c.Close()
				var data [64]byte
				n, _ := c.Read(data[:])
				if string(data[:n]) != "HI THERE" {
					t.Fatalf("expected '%s', got '%s'", " THERE", data[:n])
				}
				n, _ = c.Read(data[:])
				if string(data[:n]) != "HERE" {
					t.Fatalf("expected '%s', got '%s'", "HERE", data[:n])
				}
				n, _ = c.Read(data[:])
				if n != 0 {
					t.Fatalf("expected zero")
				}
				// add 15 connections
				for i := 0; i < 15; i++ {
					net.Dial("tcp", addr)
				}

			}()
			wg.Wait()

			c.Write([]byte("SHUTDOWN"))
			n, _ = c.Read(data[:])
			if string(data[:n]) != "GOOD BYE" {
				t.Fatalf("expected '%s', got '%s'", "GOOD BYE", data[:n])
			}
		}()
		return
	}
	preWriteCount := 0
	events.PreWrite = func() {
		preWriteCount++
	}
	var c2 Conn
	var max int
	events.Opened = func(c Conn) (out []byte, action Action) {
		if c.LocalAddr().String() == "" {
			t.Fatal("should not be empty")
		}
		if c.RemoteAddr().String() == "" {
			t.Fatal("should not be empty")
		}
		max++
		opened++
		if opened == 2 {
			c2 = c
		}
		return []byte("HI THERE"), None
	}
	events.Closed = func(c Conn) (action Action) {
		opened--
		return
	}
	events.Data = func(c Conn, in []byte) (out []byte, action Action) {
		if string(in) == "SHUTDOWN" {
			return []byte("GOOD BYE"), Shutdown
		}
		return in, None
	}
	numTicks := 0
	var c2n int
	events.Tick = func(now time.Time) (delay time.Duration, action Action) {
		numTicks++
		if numTicks == 1 {
			return -10, None
		}
		delay = time.Second / 10
		if c2 != nil {
			if c2n == 0 {
				c2.Write([]byte("HERE"))
			} else if c2n == 1 {
				c2.Close()
				c2 = nil
			}
			c2n++
		}
		return
	}
	if err := Serve(events, "tcp://"+addr); err != nil {
		t.Fatal(err)
	}
	if preWriteCount == 0 {
		t.Fatal("expected preWrites")
	}
	if opened != 0 {
		t.Fatal("expected zero")
	}
	if max != 17 {
		t.Fatalf("expected 17, got %v", max)
	}
	// should not cause problems
	c2.Write(nil)
	c2.Close()
}
