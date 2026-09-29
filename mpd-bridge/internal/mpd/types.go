package mpd

import (
	"strconv"
	"strings"
)

// JSON shapes match the app's $lib/mpd.types.ts so the app side changes as
// little as possible.

type Status struct {
	State          string   `json:"state"`  // play | pause | stop
	Volume         int      `json:"volume"` // -1 when MPD has no mixer
	Elapsed        *float64 `json:"elapsed,omitempty"`
	Duration       *float64 `json:"duration,omitempty"`
	Random         bool     `json:"random"`
	Repeat         bool     `json:"repeat"`
	Single         bool     `json:"single"`
	Consume        bool     `json:"consume"`
	Song           *int     `json:"song,omitempty"` // queue position
	SongID         *int     `json:"songid,omitempty"`
	PlaylistLength int      `json:"playlistlength"`
	UpdatingDB     *int     `json:"updating_db,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type Song struct {
	File        string   `json:"file"`
	Title       string   `json:"title,omitempty"`
	Artist      string   `json:"artist,omitempty"`
	Album       string   `json:"album,omitempty"`
	AlbumArtist string   `json:"albumArtist,omitempty"`
	Track       string   `json:"track,omitempty"`
	Date        string   `json:"date,omitempty"`
	Name        string   `json:"name,omitempty"` // stream name, for radio
	Duration    *float64 `json:"duration,omitempty"`
}

type QueueItem struct {
	Song
	ID  int `json:"id"`
	Pos int `json:"pos"`
}

type PlayerState struct {
	Status Status `json:"status"`
	Song   *Song  `json:"song"`
}

type Directory struct {
	Directory string `json:"directory"` // full path from the library root
	Name      string `json:"name"`      // last path segment
}

type Listing struct {
	Directories []Directory `json:"directories"`
	Files       []Song      `json:"files"`
}

type StoredPlaylist struct {
	Playlist     string `json:"playlist"`
	LastModified string `json:"last_modified,omitempty"`
}

// PlaybackOptions holds the toggles for PUT /options; nil fields are left as they are.
type PlaybackOptions struct {
	Random  *bool `json:"random"`
	Repeat  *bool `json:"repeat"`
	Single  *bool `json:"single"`
	Consume *bool `json:"consume"`
}

// entry is one group of a response: it starts at a "file", "directory" or
// "playlist" line and holds the attributes that follow, keyed in lower case.
// Repeated tags (e.g. several Artist lines) keep the first value.
type entry struct {
	kind  string
	value string
	attrs map[string]string
}

func entries(pairs []Pair) []entry {
	var out []entry
	for _, p := range pairs {
		k := strings.ToLower(p.Key)
		switch k {
		case "file", "directory", "playlist":
			out = append(out, entry{kind: k, value: p.Value, attrs: map[string]string{}})
			continue
		}
		if len(out) == 0 {
			continue
		}
		if _, dup := out[len(out)-1].attrs[k]; !dup {
			out[len(out)-1].attrs[k] = p.Value
		}
	}
	return out
}

func songs(pairs []Pair) []Song {
	out := []Song{}
	for _, e := range entries(pairs) {
		if e.kind == "file" {
			out = append(out, songFrom(e))
		}
	}
	return out
}

func songFrom(e entry) Song {
	s := Song{
		File:        e.value,
		Title:       e.attrs["title"],
		Artist:      e.attrs["artist"],
		Album:       e.attrs["album"],
		AlbumArtist: e.attrs["albumartist"],
		Track:       e.attrs["track"],
		Date:        e.attrs["date"],
		Name:        e.attrs["name"],
		Duration:    floatPtr(e.attrs["duration"]),
	}
	if s.Duration == nil {
		s.Duration = floatPtr(e.attrs["time"])
	}
	return s
}

func queueItems(pairs []Pair) []QueueItem {
	out := []QueueItem{}
	for _, e := range entries(pairs) {
		if e.kind != "file" {
			continue
		}
		id, _ := strconv.Atoi(e.attrs["id"])
		pos, _ := strconv.Atoi(e.attrs["pos"])
		out = append(out, QueueItem{Song: songFrom(e), ID: id, Pos: pos})
	}
	return out
}

func parseStatus(pairs []Pair) Status {
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		m[p.Key] = p.Value
	}
	st := Status{
		State:    m["state"],
		Volume:   -1,
		Elapsed:  floatPtr(m["elapsed"]),
		Duration: floatPtr(m["duration"]),
		Random:   m["random"] == "1",
		Repeat:   m["repeat"] == "1",
		// single can also be "oneshot"
		Single:     m["single"] != "" && m["single"] != "0",
		Consume:    m["consume"] != "" && m["consume"] != "0",
		Song:       intPtr(m["song"]),
		SongID:     intPtr(m["songid"]),
		UpdatingDB: intPtr(m["updating_db"]),
		Error:      m["error"],
	}
	if v, err := strconv.Atoi(m["volume"]); err == nil {
		st.Volume = v
	}
	st.PlaylistLength, _ = strconv.Atoi(m["playlistlength"])
	if st.Duration == nil {
		// Older servers only report "time: elapsed:total".
		if _, total, ok := strings.Cut(m["time"], ":"); ok {
			st.Duration = floatPtr(total)
		}
	}
	return st
}

func floatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func intPtr(s string) *int {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}
