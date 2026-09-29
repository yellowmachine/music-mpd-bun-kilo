package snap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

const statusResult = `{"server":{"groups":[{"clients":[
 {"id":"arriba","connected":true,"config":{"name":"Headphones","volume":{"percent":40,"muted":false}},"host":{"name":"pi","ip":"10.0.0.2"}},
 {"id":"b8:27:eb:00:00:01","connected":false,"config":{"name":"","volume":{"percent":80,"muted":true}},"host":{"name":"kitchen","ip":"10.0.0.3"}}
]}]}}`

type request struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// fakeSnapserver answers Server.GetStatus and Client.SetVolume, records
// requests, and can push notifications.
type fakeSnapserver struct {
	ln   net.Listener
	mu   sync.Mutex
	reqs []request
	conn net.Conn
}

func newFakeSnapserver(t *testing.T) *fakeSnapserver {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSnapserver{ln: ln}
	t.Cleanup(func() { ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.conn = c
		f.mu.Unlock()
		sc := bufio.NewScanner(c)
		for sc.Scan() {
			var r request
			json.Unmarshal(sc.Bytes(), &r)
			f.mu.Lock()
			f.reqs = append(f.reqs, r)
			f.mu.Unlock()
			switch r.Method {
			case "Server.GetStatus":
				f.send(`{"id":` + itoa(r.ID) + `,"jsonrpc":"2.0","result":` + statusResult + `}`)
			case "Client.SetVolume":
				var p struct {
					Volume json.RawMessage `json:"volume"`
				}
				json.Unmarshal(r.Params, &p)
				f.send(`{"id":` + itoa(r.ID) + `,"jsonrpc":"2.0","result":{"volume":` + string(p.Volume) + `}}`)
			default:
				f.send(`{"id":` + itoa(r.ID) + `,"jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found"}}`)
			}
		}
	}()
	return f
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// send writes one message; the protocol is one JSON document per line.
func (f *fakeSnapserver) send(msg string) {
	var line bytes.Buffer
	if err := json.Compact(&line, []byte(msg)); err != nil {
		panic(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conn.Write(append(line.Bytes(), "\r\n"...))
}

func (f *fakeSnapserver) requests() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.reqs...)
}

func start(t *testing.T, addr string) (*Snap, <-chan []Client) {
	changes := make(chan []Client, 16)
	s := New(Config{Addr: addr, Timeout: time.Second, OnChange: func(c []Client) { changes <- c }})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return s, changes
}

func next(t *testing.T, ch <-chan []Client) []Client {
	t.Helper()
	select {
	case c := <-ch:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("no change reported")
		return nil
	}
}

func TestLoadsClientsOnConnect(t *testing.T) {
	f := newFakeSnapserver(t)
	s, changes := start(t, f.ln.Addr().String())

	got := next(t, changes)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Name != "Headphones" || got[0].Volume.Percent != 40 || got[0].IP != "10.0.0.2" {
		t.Errorf("client 0 = %+v", got[0])
	}
	if got[1].Name != "kitchen" || !got[1].Volume.Muted { // falls back to the host name
		t.Errorf("client 1 = %+v", got[1])
	}
	if !s.Available() || len(s.Clients()) != 2 {
		t.Error("cache not populated")
	}
}

func TestSetVolumeKeepsUnsetField(t *testing.T) {
	f := newFakeSnapserver(t)
	s, changes := start(t, f.ln.Addr().String())
	next(t, changes)

	muted := true
	if err := s.SetVolume(context.Background(), "arriba", nil, &muted); err != nil {
		t.Fatal(err)
	}
	reqs := f.requests()
	last := reqs[len(reqs)-1]
	var p struct {
		ID     string `json:"id"`
		Volume Volume `json:"volume"`
	}
	json.Unmarshal(last.Params, &p)
	if last.Method != "Client.SetVolume" || p.ID != "arriba" || p.Volume != (Volume{Percent: 40, Muted: true}) {
		t.Errorf("sent %s %s", last.Method, last.Params)
	}
	if got := next(t, changes); got[0].Volume != (Volume{Percent: 40, Muted: true}) {
		t.Errorf("cache not updated from response: %+v", got[0])
	}

	if err := s.SetVolume(context.Background(), "ghost", nil, &muted); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown client: got %v", err)
	}
}

func TestVolumeNotificationUpdatesCache(t *testing.T) {
	f := newFakeSnapserver(t)
	s, changes := start(t, f.ln.Addr().String())
	next(t, changes)

	f.send(`{"jsonrpc":"2.0","method":"Client.OnVolumeChanged","params":{"id":"arriba","volume":{"percent":90,"muted":false}}}`)
	if got := next(t, changes); got[0].Volume.Percent != 90 {
		t.Errorf("got %+v", got[0])
	}
	if s.Clients()[0].Volume.Percent != 90 {
		t.Error("cache not updated")
	}
}

func TestDisabledAndUnavailable(t *testing.T) {
	s := New(Config{})
	s.Run(context.Background()) // returns immediately when disabled
	if s.Available() {
		t.Error("disabled client reports available")
	}
	v := 10
	if err := s.SetVolume(context.Background(), "x", &v, nil); !errors.Is(err, ErrUnavailable) {
		t.Errorf("got %v", err)
	}
}
