package httpapi

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	maxURILength          = 2048
	maxLibraryPathLength  = 1024
	maxPlaylistNameLength = 128
	maxSnapIDLength       = 128
	maxQueueURIs          = 500
)

// validURI accepts a library path (see validLibraryPath) or an http(s) stream
// URL. MPD treats anything containing "://" as a URL, so any other scheme
// (file://, smb://, …) is rejected.
func validURI(s string) error {
	if strings.Contains(s, "://") {
		if len(s) > maxURILength || !printable(s) {
			return errors.New("invalid stream URL")
		}
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("stream URLs must be http:// or https://")
		}
		return nil
	}
	if s == "/" { // the whole library
		return nil
	}
	if s == "" {
		return errors.New("uri is required")
	}
	return validLibraryPath(s)
}

// validLibraryPath accepts a path relative to MPD's music directory. The
// empty string is the root.
func validLibraryPath(s string) error {
	if len(s) > maxLibraryPathLength || !printable(s) {
		return errors.New("invalid path")
	}
	if strings.HasPrefix(s, "/") {
		return errors.New("path must be relative to the library root")
	}
	for seg := range strings.SplitSeq(s, "/") {
		if seg == ".." {
			return errors.New("path must not contain '..'")
		}
	}
	return nil
}

func validPlaylistName(s string) error {
	if s == "" || len(s) > maxPlaylistNameLength || !printable(s) ||
		strings.Contains(s, "/") || s == "." || s == ".." {
		return errors.New("invalid playlist name")
	}
	return nil
}

func validSnapID(s string) error {
	if s == "" || len(s) > maxSnapIDLength {
		return errors.New("invalid client id")
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == ':':
		default:
			return errors.New("invalid client id")
		}
	}
	return nil
}

// printable reports whether s is valid UTF-8 without control characters.
func printable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, c := range s {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}
