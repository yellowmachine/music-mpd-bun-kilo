// Package mpd implements the small subset of the MPD protocol the bridge needs.
//
// It deliberately exposes no way to send an arbitrary command line: callers
// build commands from a fixed name plus arguments, and every argument is
// quoted and rejected if it contains a line break, so request data can never
// inject extra commands.
package mpd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// ACK error codes, from MPD's src/protocol/Ack.hxx.
const (
	AckNotList       = 1
	AckArg           = 2
	AckPassword      = 3
	AckPermission    = 4
	AckUnknown       = 5
	AckNoExist       = 50
	AckPlaylistMax   = 51
	AckSystem        = 52
	AckPlaylistLoad  = 53
	AckUpdateAlready = 54
	AckPlayerSync    = 55
	AckExist         = 56
)

var (
	// ErrUnavailable means there is no usable connection to MPD right now.
	ErrUnavailable = errors.New("mpd unavailable")
	// ErrInvalidArg means an argument can't be sent safely (line breaks, NUL).
	ErrInvalidArg = errors.New("mpd: invalid argument")
)

// Error is an ACK response from MPD. The connection is still usable after it.
type Error struct {
	Code    int
	Index   int
	Command string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("mpd: %s (ACK %d@%d {%s})", e.Message, e.Code, e.Index, e.Command)
}

// Pair is one "key: value" line of a response, in the order received.
type Pair struct {
	Key   string
	Value string
}

// Cmd is a command name plus its (unquoted) arguments.
type Cmd struct {
	Name string
	Args []string
}

func command(name string, args ...string) Cmd { return Cmd{Name: name, Args: args} }

func (c Cmd) encode() (string, error) {
	var b strings.Builder
	b.WriteString(c.Name)
	for _, a := range c.Args {
		if strings.ContainsAny(a, "\r\n\x00") {
			return "", ErrInvalidArg
		}
		b.WriteString(` "`)
		for i := 0; i < len(a); i++ {
			if a[i] == '"' || a[i] == '\\' {
				b.WriteByte('\\')
			}
			b.WriteByte(a[i])
		}
		b.WriteByte('"')
	}
	b.WriteByte('\n')
	return b.String(), nil
}

// Conn is a single protocol connection. It is not safe for concurrent use.
type Conn struct {
	nc      net.Conn
	r       *bufio.Reader
	timeout time.Duration
	Version string
}

// Dial connects, reads the greeting and authenticates if password is set.
func Dial(ctx context.Context, addr, password string, timeout time.Duration) (*Conn, error) {
	d := net.Dialer{Timeout: timeout, KeepAlive: 15 * time.Second}
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	c := &Conn{nc: nc, r: bufio.NewReaderSize(nc, 64<<10), timeout: timeout}

	_ = nc.SetDeadline(time.Now().Add(timeout))
	line, err := c.readLine()
	if err != nil {
		nc.Close()
		return nil, err
	}
	if !strings.HasPrefix(line, "OK MPD ") {
		nc.Close()
		return nil, fmt.Errorf("mpd: unexpected greeting %q", line)
	}
	c.Version = strings.TrimPrefix(line, "OK MPD ")

	if password != "" {
		if _, err := c.Exec(ctx, command("password", password)); err != nil {
			nc.Close()
			return nil, err
		}
	}
	return c, nil
}

// Close closes the connection. It unblocks any in-flight read, including Idle.
func (c *Conn) Close() error { return c.nc.Close() }

// Exec runs one command. The deadline is ctx's if it has one, otherwise the
// connection's default timeout.
func (c *Conn) Exec(ctx context.Context, cmd Cmd) ([]Pair, error) {
	line, err := cmd.encode()
	if err != nil {
		return nil, err
	}
	return c.roundTrip(ctx, line)
}

// ExecList runs cmds atomically as a command list. MPD stops at the first
// failing command and reports it as an *Error.
func (c *Conn) ExecList(ctx context.Context, cmds []Cmd) ([]Pair, error) {
	var b strings.Builder
	b.WriteString("command_list_begin\n")
	for _, cmd := range cmds {
		line, err := cmd.encode()
		if err != nil {
			return nil, err
		}
		b.WriteString(line)
	}
	b.WriteString("command_list_end\n")
	return c.roundTrip(ctx, b.String())
}

// Idle blocks until one of subsystems changes and returns the changed names.
// MPD disables its client timeout while idling, so no deadline is set; abort
// it by closing the connection.
func (c *Conn) Idle(subsystems ...string) ([]string, error) {
	line, err := command("idle", subsystems...).encode()
	if err != nil {
		return nil, err
	}
	_ = c.nc.SetDeadline(time.Time{})
	if _, err := io.WriteString(c.nc, line); err != nil {
		return nil, err
	}
	pairs, err := c.readResponse()
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, p := range pairs {
		if p.Key == "changed" {
			changed = append(changed, p.Value)
		}
	}
	return changed, nil
}

func (c *Conn) roundTrip(ctx context.Context, req string) ([]Pair, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.timeout)
	}
	_ = c.nc.SetDeadline(deadline)
	if _, err := io.WriteString(c.nc, req); err != nil {
		return nil, err
	}
	return c.readResponse()
}

func (c *Conn) readResponse() ([]Pair, error) {
	var pairs []Pair
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		switch {
		case line == "OK":
			return pairs, nil
		case strings.HasPrefix(line, "ACK "):
			return nil, parseAck(line)
		}
		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			return nil, fmt.Errorf("mpd: malformed response line %q", line)
		}
		pairs = append(pairs, Pair{Key: k, Value: v})
	}
}

func (c *Conn) readLine() (string, error) {
	line, err := c.r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}

// parseAck parses "ACK [code@index] {command} message".
func parseAck(line string) *Error {
	e := &Error{Code: AckUnknown}
	rest := strings.TrimPrefix(line, "ACK ")
	if strings.HasPrefix(rest, "[") {
		if end := strings.IndexByte(rest, ']'); end > 0 {
			code, idx, _ := strings.Cut(rest[1:end], "@")
			if n, err := strconv.Atoi(code); err == nil {
				e.Code = n
			}
			e.Index, _ = strconv.Atoi(idx)
			rest = strings.TrimSpace(rest[end+1:])
		}
	}
	if strings.HasPrefix(rest, "{") {
		if end := strings.IndexByte(rest, '}'); end > 0 {
			e.Command = rest[1:end]
			rest = strings.TrimSpace(rest[end+1:])
		}
	}
	e.Message = rest
	return e
}
