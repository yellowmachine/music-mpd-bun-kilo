// Package snap talks to Snapserver over its TCP control port (1705), where
// requests, responses and notifications are newline-delimited JSON-RPC 2.0.
// One connection serves both requests and notifications.
package snap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/retry"
)

var (
	ErrUnavailable = errors.New("snap unavailable")
	ErrNotFound    = errors.New("snap client not found")
)

// RPCError is an error object returned by Snapserver.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("snap: %s (%d)", e.Message, e.Code) }

// JSON shapes match the app's SnapClient / SnapVolume types.

type Volume struct {
	Percent int  `json:"percent"`
	Muted   bool `json:"muted"`
}

type Client struct {
	ID        string `json:"id"`
	Connected bool   `json:"connected"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	IP        string `json:"ip"`
	Volume    Volume `json:"volume"`
}

type Config struct {
	// Addr is host:port of the control port. Empty disables the client:
	// it reports unavailable and never connects.
	Addr    string
	Timeout time.Duration
	Logger  *slog.Logger
	// OnChange receives the full client list whenever it changes (nil or
	// empty while disconnected). OnAvailability reports connection changes.
	// Neither may block.
	OnChange       func([]Client)
	OnAvailability func(bool)
}

type Snap struct {
	cfg       Config
	log       *slog.Logger
	available atomic.Bool
	nextID    atomic.Int64
	refresh   chan struct{}

	mu      sync.Mutex
	conn    net.Conn               // guarded by mu
	pending map[int64]chan message // guarded by mu
	clients []Client               // guarded by mu
}

func New(cfg Config) *Snap {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Snap{
		cfg:     cfg,
		log:     cfg.Logger.With("component", "snap"),
		refresh: make(chan struct{}, 1),
		pending: map[int64]chan message{},
	}
}

func (s *Snap) Available() bool { return s.available.Load() }

// Clients returns a copy of the cached client list.
func (s *Snap) Clients() []Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.clients)
}

type message struct {
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

// Run keeps the connection up until ctx is done.
func (s *Snap) Run(ctx context.Context) {
	if s.cfg.Addr == "" {
		s.log.Info("disabled")
		return
	}
	bo := retry.Backoff{}
	for ctx.Err() == nil {
		d := net.Dialer{Timeout: s.cfg.Timeout, KeepAlive: 15 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", s.cfg.Addr)
		if err != nil {
			wait := bo.Next()
			s.log.Warn("connect failed", "addr", s.cfg.Addr, "err", err, "retry_in", wait.Round(time.Millisecond).String())
			if !retry.Sleep(ctx, wait) {
				return
			}
			continue
		}
		bo.Reset()
		s.log.Info("connected", "addr", s.cfg.Addr)
		s.serve(ctx, conn)
	}
}

// serve runs one connection until it fails or ctx ends.
func (s *Snap) serve(ctx context.Context, conn net.Conn) {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	s.setAvailable(true)

	connCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(connCtx, func() { conn.Close() })
	var wg sync.WaitGroup
	wg.Go(func() { s.refresher(connCtx) })
	s.requestRefresh()

	err := s.readLoop(conn)
	if ctx.Err() == nil {
		s.log.Warn("connection lost", "err", err)
	}

	cancel()
	stop()
	conn.Close()
	wg.Wait()

	s.mu.Lock()
	s.conn = nil
	for id, ch := range s.pending {
		close(ch)
		delete(s.pending, id)
	}
	s.clients = nil
	s.mu.Unlock()
	s.setAvailable(false)
	s.emit(nil)
}

func (s *Snap) readLoop(conn net.Conn) error {
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64<<10), 4<<20) // Server.GetStatus grows with the number of clients
	for sc.Scan() {
		var m message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			s.log.Debug("ignoring malformed message", "err", err)
			continue
		}
		if m.ID != nil {
			s.mu.Lock()
			ch, ok := s.pending[*m.ID]
			delete(s.pending, *m.ID)
			s.mu.Unlock()
			if ok {
				ch <- m
			}
			continue
		}
		s.handleNotification(m)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("closed by server")
}

func (s *Snap) handleNotification(m message) {
	switch m.Method {
	case "Client.OnVolumeChanged":
		var p struct {
			ID     string `json:"id"`
			Volume Volume `json:"volume"`
		}
		if json.Unmarshal(m.Params, &p) == nil && s.updateVolume(p.ID, p.Volume) {
			return
		}
		s.requestRefresh()
	case "Client.OnConnect", "Client.OnDisconnect", "Client.OnNameChanged",
		"Server.OnUpdate", "Group.OnStreamChanged", "Group.OnNameChanged":
		s.requestRefresh()
	}
}

// updateVolume patches one client in the cache and reports whether it was found.
func (s *Snap) updateVolume(id string, v Volume) bool {
	s.mu.Lock()
	i := slices.IndexFunc(s.clients, func(c Client) bool { return c.ID == id })
	if i < 0 {
		s.mu.Unlock()
		return false
	}
	s.clients[i].Volume = v
	list := slices.Clone(s.clients)
	s.mu.Unlock()
	s.emit(list)
	return true
}

func (s *Snap) requestRefresh() {
	select {
	case s.refresh <- struct{}{}:
	default:
	}
}

// refresher reloads the full client list on request. It runs apart from
// readLoop because the reload waits for a response that readLoop delivers.
func (s *Snap) refresher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.refresh:
		}
		var res struct {
			Server serverStatus `json:"server"`
		}
		cctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
		err := s.call(cctx, "Server.GetStatus", nil, &res)
		cancel()
		if err != nil {
			s.log.Warn("refresh failed", "err", err)
			continue
		}
		list := res.Server.clients()
		s.mu.Lock()
		s.clients = list
		s.mu.Unlock()
		s.emit(slices.Clone(list))
	}
}

func (s *Snap) call(ctx context.Context, method string, params, result any) error {
	id := s.nextID.Add(1)
	req, err := json.Marshal(struct {
		ID      int64  `json:"id"`
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}{id, "2.0", method, params})
	if err != nil {
		return err
	}

	ch := make(chan message, 1)
	s.mu.Lock()
	conn := s.conn
	if conn == nil {
		s.mu.Unlock()
		return ErrUnavailable
	}
	s.pending[id] = ch
	_ = conn.SetWriteDeadline(time.Now().Add(s.cfg.Timeout))
	_, err = conn.Write(append(req, '\n'))
	s.mu.Unlock()
	if err != nil {
		conn.Close() // readLoop notices and reconnects
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	select {
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return ctx.Err()
	case m, ok := <-ch:
		if !ok {
			return ErrUnavailable
		}
		if m.Error != nil {
			return m.Error
		}
		if result != nil {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	}
}

// SetVolume changes a client's volume and/or mute. A nil field keeps the
// client's current value, since Snapserver needs both.
func (s *Snap) SetVolume(ctx context.Context, id string, percent *int, muted *bool) error {
	if !s.Available() {
		return ErrUnavailable
	}
	s.mu.Lock()
	i := slices.IndexFunc(s.clients, func(c Client) bool { return c.ID == id })
	var v Volume
	if i >= 0 {
		v = s.clients[i].Volume
	}
	s.mu.Unlock()
	if i < 0 {
		return ErrNotFound
	}
	if percent != nil {
		v.Percent = *percent
	}
	if muted != nil {
		v.Muted = *muted
	}

	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	var res struct {
		Volume Volume `json:"volume"`
	}
	if err := s.call(ctx, "Client.SetVolume", map[string]any{"id": id, "volume": v}, &res); err != nil {
		return err
	}
	// Snapserver doesn't notify the connection that made the change, so
	// apply it to the cache from the response.
	s.updateVolume(id, res.Volume)
	return nil
}

func (s *Snap) emit(list []Client) {
	if s.cfg.OnChange != nil {
		s.cfg.OnChange(list)
	}
}

func (s *Snap) setAvailable(v bool) {
	if s.available.Swap(v) != v && s.cfg.OnAvailability != nil {
		s.cfg.OnAvailability(v)
	}
}

type serverStatus struct {
	Groups []struct {
		Clients []struct {
			ID        string `json:"id"`
			Connected bool   `json:"connected"`
			Config    struct {
				Name   string `json:"name"`
				Volume Volume `json:"volume"`
			} `json:"config"`
			Host struct {
				Name string `json:"name"`
				IP   string `json:"ip"`
			} `json:"host"`
		} `json:"clients"`
	} `json:"groups"`
}

func (st serverStatus) clients() []Client {
	out := []Client{}
	for _, g := range st.Groups {
		for _, c := range g.Clients {
			name := c.Config.Name
			if name == "" {
				name = c.Host.Name
			}
			if name == "" {
				name = c.ID
			}
			out = append(out, Client{
				ID:        c.ID,
				Connected: c.Connected,
				Name:      name,
				Host:      c.Host.Name,
				IP:        c.Host.IP,
				Volume:    c.Config.Volume,
			})
		}
	}
	return out
}
