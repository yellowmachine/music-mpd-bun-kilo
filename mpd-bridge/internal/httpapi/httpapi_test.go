package httpapi

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/events"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/mpd"
	"github.com/yellowmachine/music-mpd-bun-kilo/mpd-bridge/internal/snap"
)

const token = "test-token-0123456789abcdef0123456789"

// fakeMPD records calls; err (if set) is returned by every operation.
type fakeMPD struct {
	mu    sync.Mutex
	calls []string
	err   error
	stamp int64
	all   int // AllSongs calls
}

func (f *fakeMPD) record(format string, args ...any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := format
	for _, a := range args {
		b, _ := json.Marshal(a)
		call += " " + string(b)
	}
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeMPD) Available() bool { return f.err == nil }
func (f *fakeMPD) PlayerState(ctx context.Context) (mpd.PlayerState, error) {
	return mpd.PlayerState{Status: mpd.Status{State: "play", Volume: 30}, Song: &mpd.Song{File: "a.mp3", Artist: "X", Album: "Y"}}, f.record("status")
}
func (f *fakeMPD) Transport(ctx context.Context, a mpd.Action) error { return f.record(string(a)) }
func (f *fakeMPD) PlayID(ctx context.Context, id int) error          { return f.record("playid", id) }
func (f *fakeMPD) Seek(ctx context.Context, s float64) error         { return f.record("seek", s) }
func (f *fakeMPD) SetVolume(ctx context.Context, v int) error        { return f.record("setvol", v) }
func (f *fakeMPD) SetOptions(ctx context.Context, o mpd.PlaybackOptions) error {
	return f.record("options", o)
}
func (f *fakeMPD) Queue(ctx context.Context) ([]mpd.QueueItem, error) {
	return []mpd.QueueItem{{Song: mpd.Song{File: "a.mp3"}, ID: 1}}, f.record("queue")
}
func (f *fakeMPD) AddToQueue(ctx context.Context, uris []string, replace, play bool) error {
	return f.record("add", uris, replace, play)
}
func (f *fakeMPD) ClearQueue(ctx context.Context) error       { return f.record("clear") }
func (f *fakeMPD) DeleteID(ctx context.Context, id int) error { return f.record("deleteid", id) }
func (f *fakeMPD) ListInfo(ctx context.Context, dir string) (mpd.Listing, error) {
	return mpd.Listing{Directories: []mpd.Directory{}, Files: []mpd.Song{}}, f.record("lsinfo", dir)
}
func (f *fakeMPD) DBUpdateStamp(ctx context.Context) (int64, error) {
	return f.stamp, f.record("stats")
}
func (f *fakeMPD) AllSongs(ctx context.Context) ([]mpd.Song, error) {
	f.mu.Lock()
	f.all++
	f.mu.Unlock()
	return []mpd.Song{{File: "a.mp3", Title: "A"}}, f.record("listallinfo")
}
func (f *fakeMPD) Update(ctx context.Context) (int, error) { return 3, f.record("update") }
func (f *fakeMPD) Playlists(ctx context.Context) ([]mpd.StoredPlaylist, error) {
	return []mpd.StoredPlaylist{}, f.record("listplaylists")
}
func (f *fakeMPD) PlaylistSongs(ctx context.Context, name string) ([]mpd.Song, error) {
	return []mpd.Song{}, f.record("listplaylistinfo", name)
}
func (f *fakeMPD) PlaylistAdd(ctx context.Context, name, uri string) error {
	return f.record("playlistadd", name, uri)
}
func (f *fakeMPD) PlaylistDelete(ctx context.Context, name string, pos int) error {
	return f.record("playlistdelete", name, pos)
}
func (f *fakeMPD) PlaylistMove(ctx context.Context, name string, from, to int) error {
	return f.record("playlistmove", name, from, to)
}
func (f *fakeMPD) PlaylistLoad(ctx context.Context, name string, replace, play bool) error {
	return f.record("load", name, replace, play)
}

func (f *fakeMPD) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return ""
	}
	return f.calls[len(f.calls)-1]
}

type fakeSnap struct {
	mu   sync.Mutex
	last string
}

func (f *fakeSnap) Available() bool { return true }
func (f *fakeSnap) Clients() []snap.Client {
	return []snap.Client{{ID: "arriba", Name: "Headphones", Volume: snap.Volume{Percent: 40}}}
}
func (f *fakeSnap) SetVolume(ctx context.Context, id string, p *int, m *bool) error {
	b, _ := json.Marshal(map[string]any{"id": id, "percent": p, "muted": m})
	f.mu.Lock()
	f.last = string(b)
	f.mu.Unlock()
	return nil
}

type harness struct {
	mpd    *fakeMPD
	snap   *fakeSnap
	broker *events.Broker
	h      http.Handler
}

func newHarness() *harness {
	h := &harness{mpd: &fakeMPD{stamp: 1700000000}, snap: &fakeSnap{}, broker: events.NewBroker(4, nil)}
	s := New(Options{MPD: h.mpd, Snap: h.snap, Broker: h.broker, Heartbeat: 20 * time.Millisecond})
	h.h = s.Handler(token)
	return h
}

func (h *harness) do(method, path, body string, header ...string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Authorization", "Bearer "+token)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	w := httptest.NewRecorder()
	h.h.ServeHTTP(w, req)
	return w
}

func TestAuth(t *testing.T) {
	h := newHarness()
	cases := []struct {
		name, method, path, auth string
		want                     int
	}{
		{"no header", "GET", "/status", "", 401},
		{"wrong token", "GET", "/status", "Bearer nope", 401},
		{"token prefix", "GET", "/status", "Bearer " + token[:20], 401},
		{"wrong scheme", "GET", "/status", "Basic " + token, 401},
		{"valid", "GET", "/status", "Bearer " + token, 200},
		{"scheme is case-insensitive", "GET", "/status", "bearer " + token, 200},
		{"healthz is public", "GET", "/healthz", "", 200},
		{"unknown route needs auth", "GET", "/nope", "", 401},
		{"unknown route with auth", "GET", "/nope", "Bearer " + token, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, nil)
			if c.auth != "" {
				req.Header.Set("Authorization", c.auth)
			}
			w := httptest.NewRecorder()
			h.h.ServeHTTP(w, req)
			if w.Code != c.want {
				t.Errorf("got %d, want %d", w.Code, c.want)
			}
			if w.Code == 401 && strings.TrimSpace(w.Body.String()) != `{"error":"unauthorized"}` {
				t.Errorf("401 body leaks detail: %s", w.Body)
			}
		})
	}
}

func TestVolumeValidation(t *testing.T) {
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"value":50}`, 204},
		{`{"value":0}`, 204},
		{`{"value":100}`, 204},
		{`{"value":101}`, 400},
		{`{"value":-1}`, 400},
		{`{"value":50.5}`, 400},
		{`{"value":"50"}`, 400},
		{`{}`, 400},
		{`{"value":50,"extra":1}`, 400},
		{`{"value":50}{"value":60}`, 400},
		{``, 400},
	} {
		h := newHarness()
		if w := h.do("PUT", "/volume", c.body); w.Code != c.want {
			t.Errorf("%s: got %d, want %d (%s)", c.body, w.Code, c.want, w.Body)
		}
	}
	h := newHarness()
	h.do("PUT", "/volume", `{"value":73}`)
	if got := h.mpd.last(); got != "setvol 73" {
		t.Errorf("forwarded %q", got)
	}
}

func TestQueueURIValidation(t *testing.T) {
	for _, c := range []struct {
		uri  string
		want int
	}{
		{"Blur/The Great Escape/01.flac", 204},
		{"Live: 1999/track.mp3", 204}, // a colon is not a scheme
		{"/", 204},
		{"http://radio.example/stream", 204},
		{"https://vps.example/audio/1?sig=abc", 204},
		{"", 400},
		{"../etc/passwd", 400},
		{"Blur/../../etc", 400},
		{"/etc/passwd", 400},
		{"file:///etc/passwd", 400},
		{"smb://nas/share/a.mp3", 400},
		{"http://", 400},
		{"a\nclear", 400},
	} {
		h := newHarness()
		body, _ := json.Marshal(map[string]any{"uris": []string{c.uri}})
		if w := h.do("POST", "/queue", string(body)); w.Code != c.want {
			t.Errorf("%q: got %d, want %d (%s)", c.uri, w.Code, c.want, w.Body)
		}
	}
}

func TestQueueReplaceAndPlay(t *testing.T) {
	h := newHarness()
	w := h.do("POST", "/queue", `{"uris":["a.mp3","b.mp3"],"replace":true,"play":true}`)
	if w.Code != 204 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if got := h.mpd.last(); got != `add ["a.mp3","b.mp3"] true true` {
		t.Errorf("forwarded %q", got)
	}

	many := make([]string, maxQueueURIs+1)
	for i := range many {
		many[i] = "a.mp3"
	}
	body, _ := json.Marshal(map[string]any{"uris": many})
	if w := h.do("POST", "/queue", string(body)); w.Code != 400 {
		t.Errorf("too many uris: got %d", w.Code)
	}
	if w := h.do("POST", "/queue", `{"uris":[]}`); w.Code != 400 {
		t.Errorf("no uris: got %d", w.Code)
	}
}

func TestBodyTooLarge(t *testing.T) {
	h := newHarness()
	big := `{"uris":["` + strings.Repeat("a", maxBodyBytes) + `"]}`
	if w := h.do("POST", "/queue", big); w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got %d", w.Code)
	}
}

func TestMPDErrorsMapToStatus(t *testing.T) {
	h := newHarness()
	h.mpd.err = mpd.ErrUnavailable
	for _, path := range []string{"/status", "/queue", "/playlists"} {
		w := h.do("GET", path, "")
		if w.Code != 503 || !strings.Contains(w.Body.String(), "mpd unavailable") {
			t.Errorf("%s: got %d %s", path, w.Code, w.Body)
		}
	}
	if w := h.do("POST", "/player/play", ""); w.Code != 503 {
		t.Errorf("play: got %d", w.Code)
	}
	if w := h.do("GET", "/healthz", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"mpd":false`) {
		t.Errorf("healthz: got %d %s", w.Code, w.Body)
	}

	h.mpd.err = &mpd.Error{Code: mpd.AckNoExist, Message: "No such playlist"}
	if w := h.do("GET", "/playlists/Nope", ""); w.Code != 404 || !strings.Contains(w.Body.String(), "No such playlist") {
		t.Errorf("ACK 50: got %d %s", w.Code, w.Body)
	}
	h.mpd.err = &mpd.Error{Code: mpd.AckSystem, Message: "internal detail"}
	if w := h.do("POST", "/player/next", ""); w.Code != 502 || strings.Contains(w.Body.String(), "internal detail") {
		t.Errorf("ACK 52: got %d %s", w.Code, w.Body)
	}
}

func TestPlaylistRoutes(t *testing.T) {
	h := newHarness()
	check := func(method, path, body string, want int, call string) {
		t.Helper()
		w := h.do(method, path, body)
		if w.Code != want {
			t.Errorf("%s %s: got %d, want %d (%s)", method, path, w.Code, want, w.Body)
			return
		}
		if call != "" && h.mpd.last() != call {
			t.Errorf("%s %s: forwarded %q, want %q", method, path, h.mpd.last(), call)
		}
	}
	check("GET", "/playlists/My%20Favs", "", 200, `listplaylistinfo "My Favs"`)
	check("POST", "/playlists/Favs/songs", `{"uri":"a.mp3"}`, 204, `playlistadd "Favs" "a.mp3"`)
	check("DELETE", "/playlists/Favs/songs/3", "", 204, `playlistdelete "Favs" 3`)
	check("POST", "/playlists/Favs/move", `{"from":1,"to":0}`, 204, `playlistmove "Favs" 1 0`)
	check("POST", "/playlists/Favs/load", "", 204, `load "Favs" false false`)
	check("POST", "/playlists/Favs/load", `{"replace":true,"play":true}`, 204, `load "Favs" true true`)

	check("GET", "/playlists/a%2Fb", "", 400, "")
	check("DELETE", "/playlists/Favs/songs/-1", "", 400, "")
	check("POST", "/playlists/Favs/move", `{"from":1}`, 400, "")
	check("POST", "/playlists/Favs/songs", `{"uri":"../x"}`, 400, "")
}

func TestOtherRoutes(t *testing.T) {
	h := newHarness()
	for _, c := range []struct {
		method, path, body string
		want               int
		call               string
	}{
		{"POST", "/player/toggle", "", 204, "toggle"},
		{"POST", "/player/playid", `{"id":4}`, 204, "playid 4"},
		{"POST", "/player/playid", `{"id":-4}`, 400, ""},
		{"POST", "/player/seek", `{"seconds":12.5}`, 204, "seek 12.5"},
		{"POST", "/player/seek", `{"seconds":-1}`, 400, ""},
		{"POST", "/player/rewind", "", 404, ""},
		{"GET", "/player/play", "", 405, ""},
		{"PUT", "/options", `{"random":true}`, 204, `options {"random":true,"repeat":null,"single":null,"consume":null}`},
		{"PUT", "/options", `{}`, 400, ""},
		{"DELETE", "/queue/12", "", 204, "deleteid 12"},
		{"DELETE", "/queue/abc", "", 400, ""},
		{"DELETE", "/queue", "", 204, "clear"},
		{"GET", "/library/ls?path=Blur", "", 200, `lsinfo "Blur"`},
		{"GET", "/library/ls", "", 200, `lsinfo ""`},
		{"GET", "/library/ls?path=../x", "", 400, ""},
		{"POST", "/library/update", "", 202, "update"},
	} {
		w := h.do(c.method, c.path, c.body)
		if w.Code != c.want {
			t.Errorf("%s %s: got %d, want %d (%s)", c.method, c.path, w.Code, c.want, w.Body)
			continue
		}
		if c.call != "" && h.mpd.last() != c.call {
			t.Errorf("%s %s: forwarded %q, want %q", c.method, c.path, h.mpd.last(), c.call)
		}
	}
}

func TestLibraryAllCachesAndCompresses(t *testing.T) {
	h := newHarness()

	w := h.do("GET", "/library/all", "", "Accept-Encoding", "gzip")
	if w.Code != 200 || w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("got %d, encoding %q", w.Code, w.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		DBUpdate int64      `json:"db_update"`
		Songs    []mpd.Song `json:"songs"`
	}
	if err := json.NewDecoder(zr).Decode(&body); err != nil || body.DBUpdate != 1700000000 || len(body.Songs) != 1 {
		t.Fatalf("body = %+v, %v", body, err)
	}
	etag := w.Header().Get("ETag")

	if w := h.do("GET", "/library/all", "", "If-None-Match", etag); w.Code != 304 {
		t.Errorf("If-None-Match: got %d", w.Code)
	}
	if w := h.do("GET", "/library/all", ""); w.Code != 200 || w.Header().Get("Content-Encoding") != "" {
		t.Errorf("plain: got %d, encoding %q", w.Code, w.Header().Get("Content-Encoding"))
	}
	if h.mpd.all != 1 {
		t.Errorf("listallinfo ran %d times, want 1 (cached)", h.mpd.all)
	}

	h.mpd.stamp++ // database updated
	h.do("GET", "/library/all", "")
	if h.mpd.all != 2 {
		t.Errorf("cache not invalidated by db_update change")
	}
}

func TestSnapRoutes(t *testing.T) {
	h := newHarness()
	if w := h.do("GET", "/snap/clients", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"arriba"`) {
		t.Errorf("clients: got %d %s", w.Code, w.Body)
	}
	if w := h.do("PUT", "/snap/clients/b8:27:eb:00:00:01/volume", `{"muted":true}`); w.Code != 204 {
		t.Errorf("mute: got %d %s", w.Code, w.Body)
	}
	if h.snap.last != `{"id":"b8:27:eb:00:00:01","muted":true,"percent":null}` {
		t.Errorf("forwarded %s", h.snap.last)
	}
	for _, body := range []string{`{}`, `{"percent":101}`} {
		if w := h.do("PUT", "/snap/clients/arriba/volume", body); w.Code != 400 {
			t.Errorf("%s: got %d", body, w.Code)
		}
	}
	if w := h.do("PUT", "/snap/clients/bad%20id/volume", `{"muted":true}`); w.Code != 400 {
		t.Errorf("bad id: got %d", w.Code)
	}
}

func TestEventsStream(t *testing.T) {
	h := newHarness()
	srv := httptest.NewServer(h.h)
	defer srv.Close()

	if resp, err := http.Get(srv.URL + "/events"); err != nil || resp.StatusCode != 401 {
		t.Fatalf("unauthenticated: %v %v", resp.StatusCode, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type %q", ct)
	}

	lines := make(chan string, 100)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	next := func() string {
		select {
		case l := <-lines:
			return l
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for stream")
			return ""
		}
	}

	if l := next(); l != "event: snapshot" {
		t.Fatalf("first line %q", l)
	}
	var snap struct {
		Status      *mpd.Status     `json:"status"`
		Song        *mpd.Song       `json:"song"`
		Queue       []mpd.QueueItem `json:"queue"`
		SnapClients []snap.Client   `json:"snap_clients"`
		Connection  map[string]bool `json:"connection"`
	}
	data := strings.TrimPrefix(next(), "data: ")
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		t.Fatalf("snapshot %q: %v", data, err)
	}
	if snap.Status == nil || snap.Status.Volume != 30 || snap.Song.Artist != "X" || len(snap.Queue) != 1 ||
		len(snap.SnapClients) != 1 || !snap.Connection["mpd"] {
		t.Errorf("snapshot = %+v", snap)
	}
	next() // blank line ending the event

	h.broker.Publish("mixer", map[string]int{"volume": 55})
	var got []string
	for len(got) < 2 {
		if l := next(); l != "" && l != ": ping" {
			got = append(got, l)
		}
	}
	if !slices.Equal(got, []string{"event: mixer", `data: {"volume":55}`}) {
		t.Errorf("got %q", got)
	}

	// Heartbeat interval is 20 ms in the harness.
	for l := next(); l != ": ping"; l = next() {
	}

	h.broker.Close() // shutdown ends the stream
	for range lines {
	}
}
