package mpd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEncodeQuotesAndRejectsLineBreaks(t *testing.T) {
	got, err := command("add", `Artist/Say "hi" \ bye`).encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := `add "Artist/Say \"hi\" \\ bye"` + "\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, bad := range []string{"a\nclear", "a\rb", "a\x00b"} {
		if _, err := command("add", bad).encode(); !errors.Is(err, ErrInvalidArg) {
			t.Errorf("%q: got %v, want ErrInvalidArg", bad, err)
		}
	}
}

func TestParseAck(t *testing.T) {
	e := parseAck("ACK [50@1] {load} No such playlist")
	if e.Code != AckNoExist || e.Index != 1 || e.Command != "load" || e.Message != "No such playlist" {
		t.Errorf("got %+v", e)
	}
	if e := parseAck("ACK garbage"); e.Code != AckUnknown || e.Message != "garbage" {
		t.Errorf("got %+v", e)
	}
}

func TestPlayerState(t *testing.T) {
	f := newFakeMPD(t, func(cmd string) string {
		switch cmd {
		case "status":
			return "volume: 42\nrepeat: 1\nrandom: 0\nsingle: oneshot\nconsume: 0\nplaylistlength: 3\n" +
				"state: play\nsong: 1\nsongid: 7\nelapsed: 12.5\nduration: 200.25\n"
		case "currentsong":
			return "file: A/B/01.flac\nArtist: First\nArtist: Second\nTitle: Song\nAlbum: Album\nduration: 200.25\nPos: 1\nId: 7\n"
		}
		return "ACK [5@0] {} unknown command"
	})
	c := startClient(t, f.addr())

	ps, err := c.PlayerState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	st := ps.Status
	if st.State != "play" || st.Volume != 42 || !st.Repeat || st.Random || !st.Single || st.Consume ||
		st.PlaylistLength != 3 || *st.Song != 1 || *st.SongID != 7 || *st.Elapsed != 12.5 || *st.Duration != 200.25 {
		t.Errorf("status = %+v", st)
	}
	if ps.Song == nil || ps.Song.File != "A/B/01.flac" || ps.Song.Artist != "First" || ps.Song.Title != "Song" {
		t.Errorf("song = %+v", ps.Song)
	}
}

func TestPlayerStateNoMixerNoSong(t *testing.T) {
	f := newFakeMPD(t, func(cmd string) string {
		if cmd == "status" {
			return "state: stop\n"
		}
		return ""
	})
	c := startClient(t, f.addr())
	ps, err := c.PlayerState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ps.Status.Volume != -1 || ps.Song != nil {
		t.Errorf("got %+v", ps)
	}
}

func TestAckIsReturnedAndConnectionKept(t *testing.T) {
	f := newFakeMPD(t, func(cmd string) string {
		if strings.HasPrefix(cmd, "listplaylistinfo") {
			return "ACK [50@0] {listplaylistinfo} No such playlist"
		}
		return ""
	})
	c := startClient(t, f.addr())

	_, err := c.PlaylistSongs(context.Background(), "nope")
	var ack *Error
	if !errors.As(err, &ack) || ack.Code != AckNoExist {
		t.Fatalf("got %v, want ACK 50", err)
	}
	if !c.Available() {
		t.Error("an ACK must not drop the connection")
	}
}

func TestAddToQueueUsesOneCommandList(t *testing.T) {
	f := newFakeMPD(t, func(string) string { return "" })
	c := startClient(t, f.addr())

	err := c.AddToQueue(context.Background(), []string{"Blur/Song.mp3", "http://radio.example/stream"}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"command_list_begin",
		"clear",
		`add "Blur/Song.mp3"`,
		`add "http://radio.example/stream"`,
		"play",
		"command_list_end",
	}
	if got := f.commands(); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestToggle(t *testing.T) {
	state := "play"
	f := newFakeMPD(t, func(cmd string) string {
		if cmd == "status" {
			return "state: " + state + "\n"
		}
		return ""
	})
	c := startClient(t, f.addr())

	if err := c.Transport(context.Background(), ActToggle); err != nil {
		t.Fatal(err)
	}
	if got := f.commands(); !slices.Equal(got, []string{"status", `pause "1"`}) {
		t.Errorf("playing: got %q", got)
	}
}

func TestUnavailableAndReconnect(t *testing.T) {
	c := NewClient(Config{Addr: "127.0.0.1:1", Timeout: 100 * time.Millisecond})
	if _, err := c.Queue(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("before connecting: got %v, want ErrUnavailable", err)
	}

	f := newFakeMPD(t, func(cmd string) string {
		if cmd == "playlistinfo" {
			return "file: a.mp3\nPos: 0\nId: 1\nfile: b.mp3\nPos: 1\nId: 2\n"
		}
		return ""
	})
	c = startClient(t, f.addr())
	q, err := c.Queue(context.Background())
	if err != nil || len(q) != 2 || q[1].ID != 2 || q[1].File != "b.mp3" {
		t.Fatalf("got %+v, %v", q, err)
	}

	f.dropConns()
	if _, err := c.Queue(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("after drop: got %v, want ErrUnavailable", err)
	}
	waitFor(t, c.Available) // Run reconnects on its own
	if _, err := c.Queue(context.Background()); err != nil {
		t.Fatalf("after reconnect: %v", err)
	}
}

func TestListingsGroupEntries(t *testing.T) {
	f := newFakeMPD(t, func(cmd string) string {
		switch {
		case cmd == "listallinfo":
			// A directory's Last-Modified must not be attributed to the file before it.
			return "directory: Blur\nLast-Modified: 2020\nfile: Blur/1.mp3\nTitle: One\nTime: 100\n" +
				"directory: Blur/Live\nLast-Modified: 2021\nfile: Blur/Live/2.mp3\nTitle: Two\n"
		case strings.HasPrefix(cmd, "lsinfo"):
			return "directory: Blur/Live\nLast-Modified: 2021\nfile: Blur/1.mp3\nTitle: One\nplaylist: Blur/x.m3u\n"
		case cmd == "listplaylists":
			return "playlist: Favs\nLast-Modified: 2024-01-01T00:00:00Z\nplaylist: Other\n"
		}
		return ""
	})
	c := startClient(t, f.addr())
	ctx := context.Background()

	all, err := c.AllSongs(ctx)
	if err != nil || len(all) != 2 || all[0].Title != "One" || *all[0].Duration != 100 || all[1].File != "Blur/Live/2.mp3" {
		t.Errorf("AllSongs = %+v, %v", all, err)
	}

	l, err := c.ListInfo(ctx, "Blur")
	if err != nil || len(l.Directories) != 1 || l.Directories[0].Name != "Live" || len(l.Files) != 1 {
		t.Errorf("ListInfo = %+v, %v", l, err)
	}
	if got := f.commands(); !slices.Contains(got, `lsinfo "Blur"`) {
		t.Errorf("commands = %q", got)
	}

	pl, err := c.Playlists(ctx)
	if err != nil || len(pl) != 2 || pl[0].LastModified == "" || pl[1].Playlist != "Other" {
		t.Errorf("Playlists = %+v, %v", pl, err)
	}
}

func TestWatchReportsChanges(t *testing.T) {
	release := make(chan string, 1)
	f := newFakeMPD(t, func(cmd string) string {
		if strings.HasPrefix(cmd, "idle") {
			return <-release
		}
		return ""
	})
	c := NewClient(Config{Addr: f.addr(), Timeout: time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	connected := make(chan struct{}, 1)
	changes := make(chan []string, 1)
	done := make(chan struct{})
	go func() {
		c.Watch(ctx, []string{"player", "mixer"},
			func() { connected <- struct{}{} },
			func(s []string) { changes <- s })
		close(done)
	}()

	<-connected
	release <- "changed: player\nchanged: mixer\n"
	if got := <-changes; !slices.Equal(got, []string{"player", "mixer"}) {
		t.Errorf("got %q", got)
	}

	cancel()
	release <- "" // unblock the fake's pending idle
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Watch did not return after cancel")
	}
}
