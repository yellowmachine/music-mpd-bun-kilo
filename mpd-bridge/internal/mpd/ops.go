package mpd

import (
	"context"
	"fmt"
	"path"
	"strconv"
)

// This file is the complete list of MPD commands the bridge can send.

func (c *Client) exec(ctx context.Context, cmd Cmd) ([]Pair, error) {
	var out []Pair
	err := c.do(ctx, func(conn *Conn) error {
		p, err := conn.Exec(ctx, cmd)
		out = p
		return err
	})
	return out, err
}

func (c *Client) execOK(ctx context.Context, cmds ...Cmd) error {
	return c.do(ctx, func(conn *Conn) error {
		var err error
		if len(cmds) == 1 {
			_, err = conn.Exec(ctx, cmds[0])
		} else {
			_, err = conn.ExecList(ctx, cmds)
		}
		return err
	})
}

func itoa(n int) string { return strconv.Itoa(n) }

func (c *Client) PlayerState(ctx context.Context) (PlayerState, error) {
	var ps PlayerState
	err := c.do(ctx, func(conn *Conn) error {
		st, err := conn.Exec(ctx, command("status"))
		if err != nil {
			return err
		}
		cur, err := conn.Exec(ctx, command("currentsong"))
		if err != nil {
			return err
		}
		ps.Status = parseStatus(st)
		if s := songs(cur); len(s) > 0 {
			ps.Song = &s[0]
		}
		return nil
	})
	return ps, err
}

type Action string

const (
	ActPlay     Action = "play"
	ActPause    Action = "pause"
	ActResume   Action = "resume"
	ActToggle   Action = "toggle"
	ActStop     Action = "stop"
	ActNext     Action = "next"
	ActPrevious Action = "previous"
)

var Actions = []Action{ActPlay, ActPause, ActResume, ActToggle, ActStop, ActNext, ActPrevious}

func (c *Client) Transport(ctx context.Context, a Action) error {
	switch a {
	case ActPlay:
		return c.execOK(ctx, command("play"))
	case ActPause:
		return c.execOK(ctx, command("pause", "1"))
	case ActResume:
		return c.execOK(ctx, command("pause", "0"))
	case ActStop:
		return c.execOK(ctx, command("stop"))
	case ActNext:
		return c.execOK(ctx, command("next"))
	case ActPrevious:
		return c.execOK(ctx, command("previous"))
	case ActToggle:
		// Under one lock so nothing else changes the state in between.
		return c.do(ctx, func(conn *Conn) error {
			st, err := conn.Exec(ctx, command("status"))
			if err != nil {
				return err
			}
			if parseStatus(st).State == "play" {
				_, err = conn.Exec(ctx, command("pause", "1"))
			} else {
				_, err = conn.Exec(ctx, command("play"))
			}
			return err
		})
	}
	return fmt.Errorf("unknown action %q", a)
}

func (c *Client) PlayID(ctx context.Context, id int) error {
	return c.execOK(ctx, command("playid", itoa(id)))
}

func (c *Client) Seek(ctx context.Context, seconds float64) error {
	return c.execOK(ctx, command("seekcur", strconv.FormatFloat(seconds, 'f', 3, 64)))
}

func (c *Client) SetVolume(ctx context.Context, v int) error {
	return c.execOK(ctx, command("setvol", itoa(v)))
}

func (c *Client) SetOptions(ctx context.Context, o PlaybackOptions) error {
	var cmds []Cmd
	for _, opt := range []struct {
		name string
		v    *bool
	}{{"random", o.Random}, {"repeat", o.Repeat}, {"single", o.Single}, {"consume", o.Consume}} {
		if opt.v != nil {
			cmds = append(cmds, command(opt.name, boolArg(*opt.v)))
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return c.execOK(ctx, cmds...)
}

func boolArg(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (c *Client) Queue(ctx context.Context) ([]QueueItem, error) {
	p, err := c.exec(ctx, command("playlistinfo"))
	if err != nil {
		return nil, err
	}
	return queueItems(p), nil
}

// AddToQueue adds uris in one command list. replace clears the queue first;
// play starts playback afterwards (from the top when replace is set).
func (c *Client) AddToQueue(ctx context.Context, uris []string, replace, play bool) error {
	cmds := make([]Cmd, 0, len(uris)+2)
	if replace {
		cmds = append(cmds, command("clear"))
	}
	for _, u := range uris {
		cmds = append(cmds, command("add", u))
	}
	if play {
		cmds = append(cmds, command("play"))
	}
	return c.execOK(ctx, cmds...)
}

func (c *Client) ClearQueue(ctx context.Context) error {
	return c.execOK(ctx, command("clear"))
}

func (c *Client) DeleteID(ctx context.Context, id int) error {
	return c.execOK(ctx, command("deleteid", itoa(id)))
}

func (c *Client) ListInfo(ctx context.Context, dir string) (Listing, error) {
	p, err := c.exec(ctx, command("lsinfo", dir))
	if err != nil {
		return Listing{}, err
	}
	l := Listing{Directories: []Directory{}, Files: []Song{}}
	for _, e := range entries(p) {
		switch e.kind {
		case "directory":
			l.Directories = append(l.Directories, Directory{Directory: e.value, Name: path.Base(e.value)})
		case "file":
			l.Files = append(l.Files, songFrom(e))
		}
	}
	return l, nil
}

// DBUpdateStamp returns the Unix time of the last database update, which
// identifies the current version of the library.
func (c *Client) DBUpdateStamp(ctx context.Context) (int64, error) {
	p, err := c.exec(ctx, command("stats"))
	if err != nil {
		return 0, err
	}
	for _, kv := range p {
		if kv.Key == "db_update" {
			return strconv.ParseInt(kv.Value, 10, 64)
		}
	}
	return 0, nil
}

// AllSongs returns every song in the library (listallinfo), flattened.
func (c *Client) AllSongs(ctx context.Context) ([]Song, error) {
	p, err := c.exec(ctx, command("listallinfo"))
	if err != nil {
		return nil, err
	}
	return songs(p), nil
}

// Update starts a database rescan and returns its job id.
func (c *Client) Update(ctx context.Context) (int, error) {
	p, err := c.exec(ctx, command("update"))
	if err != nil {
		return 0, err
	}
	for _, kv := range p {
		if kv.Key == "updating_db" {
			return strconv.Atoi(kv.Value)
		}
	}
	return 0, nil
}

func (c *Client) Playlists(ctx context.Context) ([]StoredPlaylist, error) {
	p, err := c.exec(ctx, command("listplaylists"))
	if err != nil {
		return nil, err
	}
	out := []StoredPlaylist{}
	for _, e := range entries(p) {
		if e.kind == "playlist" {
			out = append(out, StoredPlaylist{Playlist: e.value, LastModified: e.attrs["last-modified"]})
		}
	}
	return out, nil
}

func (c *Client) PlaylistSongs(ctx context.Context, name string) ([]Song, error) {
	p, err := c.exec(ctx, command("listplaylistinfo", name))
	if err != nil {
		return nil, err
	}
	return songs(p), nil
}

func (c *Client) PlaylistAdd(ctx context.Context, name, uri string) error {
	return c.execOK(ctx, command("playlistadd", name, uri))
}

func (c *Client) PlaylistDelete(ctx context.Context, name string, pos int) error {
	return c.execOK(ctx, command("playlistdelete", name, itoa(pos)))
}

func (c *Client) PlaylistMove(ctx context.Context, name string, from, to int) error {
	return c.execOK(ctx, command("playlistmove", name, itoa(from), itoa(to)))
}

// PlaylistLoad appends a stored playlist to the queue, optionally replacing
// the queue and starting playback.
func (c *Client) PlaylistLoad(ctx context.Context, name string, replace, play bool) error {
	var cmds []Cmd
	if replace {
		cmds = append(cmds, command("clear"))
	}
	cmds = append(cmds, command("load", name))
	if play {
		cmds = append(cmds, command("play"))
	}
	return c.execOK(ctx, cmds...)
}
