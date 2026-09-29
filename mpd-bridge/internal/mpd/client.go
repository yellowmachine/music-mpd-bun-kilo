package mpd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/retry"
)

// Config configures a Client.
type Config struct {
	Addr     string
	Password string
	// Timeout bounds dialing and each command that has no ctx deadline.
	Timeout time.Duration
	// PingInterval keeps the command connection alive; MPD drops clients
	// that stay silent longer than its connection_timeout (60 s by default).
	PingInterval time.Duration
	Logger       *slog.Logger
	// OnAvailability is called whenever the command connection comes up or
	// goes down. It must not block.
	OnAvailability func(available bool)
}

// Client owns one command connection to MPD, shared by all requests and
// serialized by a mutex, and reconnects it in the background.
type Client struct {
	cfg       Config
	log       *slog.Logger
	mu        sync.Mutex
	conn      *Conn // guarded by mu
	available atomic.Bool
	lost      chan struct{}
}

func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.PingInterval == 0 {
		cfg.PingInterval = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Client{cfg: cfg, log: cfg.Logger.With("component", "mpd"), lost: make(chan struct{}, 1)}
}

// Available reports whether the command connection is currently up.
func (c *Client) Available() bool { return c.available.Load() }

// Run keeps the command connection up until ctx is done.
func (c *Client) Run(ctx context.Context) {
	bo := retry.Backoff{}
	for {
		conn, err := Dial(ctx, c.cfg.Addr, c.cfg.Password, c.cfg.Timeout)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			d := bo.Next()
			c.log.Warn("connect failed", "addr", c.cfg.Addr, "err", err, "retry_in", d.Round(time.Millisecond).String())
			if !retry.Sleep(ctx, d) {
				return
			}
			continue
		}
		bo.Reset()
		c.log.Info("connected", "addr", c.cfg.Addr, "version", conn.Version)
		c.setConn(conn)

		c.keepAlive(ctx)
		if ctx.Err() != nil {
			c.mu.Lock()
			c.dropLocked(nil)
			c.mu.Unlock()
			return
		}
	}
}

func (c *Client) setConn(conn *Conn) {
	c.mu.Lock()
	c.conn = conn
	select { // discard a stale signal from the previous connection
	case <-c.lost:
	default:
	}
	c.mu.Unlock()
	c.setAvailable(true)
}

// keepAlive pings periodically and returns once the connection is lost.
func (c *Client) keepAlive(ctx context.Context) {
	t := time.NewTicker(c.cfg.PingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.lost:
			return
		case <-t.C:
			_ = c.do(ctx, func(conn *Conn) error {
				_, err := conn.Exec(ctx, command("ping"))
				return err
			})
		}
	}
}

// do runs fn with exclusive use of the connection. Any error other than an
// ACK means the connection is in an unknown state, so it is dropped and the
// reconnect loop takes over.
func (c *Client) do(ctx context.Context, fn func(*Conn) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return ErrUnavailable
	}
	err := fn(c.conn)
	if err == nil {
		return nil
	}
	var ack *Error
	if errors.As(err, &ack) || errors.Is(err, ErrInvalidArg) {
		return err
	}
	if ctx.Err() != nil {
		// The caller gave up mid-response; the stream is out of sync.
		c.dropLocked(ctx.Err())
		return ctx.Err()
	}
	c.dropLocked(err)
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}

func (c *Client) dropLocked(cause error) {
	if c.conn == nil {
		return
	}
	c.conn.Close()
	c.conn = nil
	if cause != nil {
		c.log.Warn("connection lost", "err", cause)
	}
	select {
	case c.lost <- struct{}{}:
	default:
	}
	c.setAvailable(false)
}

func (c *Client) setAvailable(v bool) {
	if c.available.Swap(v) != v && c.cfg.OnAvailability != nil {
		c.cfg.OnAvailability(v)
	}
}

// Watch holds a dedicated connection in idle mode and calls onChange with the
// changed subsystems. onConnect runs after every (re)connect, since changes
// made while disconnected were missed. Returns when ctx is done.
func (c *Client) Watch(ctx context.Context, subsystems []string, onConnect func(), onChange func([]string)) {
	log := c.log.With("conn", "idle")
	bo := retry.Backoff{}
	for ctx.Err() == nil {
		conn, err := Dial(ctx, c.cfg.Addr, c.cfg.Password, c.cfg.Timeout)
		if err != nil {
			if !retry.Sleep(ctx, bo.Next()) {
				return
			}
			continue
		}
		bo.Reset()
		log.Info("watching", "subsystems", subsystems)
		stop := context.AfterFunc(ctx, func() { conn.Close() })

		onConnect()
		for {
			changed, err := conn.Idle(subsystems...)
			if err != nil {
				if ctx.Err() == nil {
					log.Warn("idle connection lost", "err", err)
				}
				break
			}
			onChange(changed)
		}
		stop()
		conn.Close()
	}
}
