package mpd

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeMPD is a minimal MPD server. respond gets each command line (command
// lists are expanded) and returns the response body without the final "OK",
// or a line starting with "ACK" to fail.
type fakeMPD struct {
	ln      net.Listener
	respond func(cmd string) string

	mu    sync.Mutex
	got   []string
	conns []net.Conn
}

func newFakeMPD(t *testing.T, respond func(cmd string) string) *fakeMPD {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeMPD{ln: ln, respond: respond}
	t.Cleanup(f.close)
	go f.serve()
	return f
}

func (f *fakeMPD) addr() string { return f.ln.Addr().String() }

func (f *fakeMPD) close() {
	f.ln.Close()
	f.dropConns()
}

// dropConns closes every open client connection, as if MPD restarted.
func (f *fakeMPD) dropConns() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.conns {
		c.Close()
	}
	f.conns = nil
}

func (f *fakeMPD) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.got...)
}

func (f *fakeMPD) serve() {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.conns = append(f.conns, c)
		f.mu.Unlock()
		go f.handle(c)
	}
}

func (f *fakeMPD) handle(c net.Conn) {
	defer c.Close()
	w := bufio.NewWriter(c)
	w.WriteString("OK MPD 0.24.0\n")
	w.Flush()

	var list []string
	inList := false
	sc := bufio.NewScanner(c)
	for sc.Scan() {
		line := sc.Text()
		f.mu.Lock()
		f.got = append(f.got, line)
		f.mu.Unlock()

		switch {
		case line == "command_list_begin":
			inList, list = true, nil
			continue
		case inList && line != "command_list_end":
			list = append(list, line)
			continue
		case line == "command_list_end":
			inList = false
		default:
			list = []string{line}
		}

		reply := "OK\n"
		for _, cmd := range list {
			body := f.respond(cmd)
			if strings.HasPrefix(body, "ACK") {
				reply = body + "\n"
				break
			}
			reply = body + "OK\n"
		}
		w.WriteString(reply)
		w.Flush()
	}
}

// startClient runs a Client against addr and waits until it is connected.
func startClient(t *testing.T, addr string) *Client {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient(Config{Addr: addr, Timeout: time.Second})
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	waitFor(t, c.Available)
	return c
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
