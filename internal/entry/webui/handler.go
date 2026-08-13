// Package webui serves optional Sandrone Web UI static assets.
package webui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

type Option func(*options)

type options struct {
	static           fs.FS
	reservedPrefixes []string
}

func Handler(opts ...Option) http.Handler {
	cfg := options{reservedPrefixes: defaultReservedPrefixes()}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.static == nil {
		cfg.static = embeddedStaticFS()
	}
	return &handler{
		static:           cfg.static,
		reservedPrefixes: normalizeReservedPrefixes(cfg.reservedPrefixes),
	}
}

func HandlerWithFS(static fs.FS, opts ...Option) http.Handler {
	all := make([]Option, 0, 1+len(opts))
	all = append(all, func(cfg *options) {
		cfg.static = static
	})
	all = append(all, opts...)
	return Handler(all...)
}

func WithReservedPrefixes(prefixes ...string) Option {
	return func(cfg *options) {
		cfg.reservedPrefixes = prefixes
	}
}

type handler struct {
	static           fs.FS
	reservedPrefixes []string
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if isReservedPath(r.URL.Path, h.reservedPrefixes) {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "." || name == "" {
		if h.serveStatic(w, r, "index.html") {
			return
		}
		http.NotFound(w, r)
		return
	}
	if h.serveStatic(w, r, name) {
		return
	}
	if isManagementPath(r.URL.Path, h.reservedPrefixes) {
		if h.serveStatic(w, r, "index.html") {
			return
		}
		http.NotFound(w, r)
		return
	}
	http.NotFound(w, r)
}

func (h *handler) serveStatic(w http.ResponseWriter, r *http.Request, name string) bool {
	if h.static == nil {
		return false
	}
	if _, err := fs.Stat(h.static, name); err != nil {
		return false
	}
	representationName := name
	if _, err := fs.Stat(h.static, name+".br"); err == nil {
		w.Header().Set("Vary", "Accept-Encoding")
		if acceptsBrotli(r.Header.Get("Accept-Encoding")) {
			representationName = name + ".br"
			w.Header().Set("Content-Encoding", "br")
		}
	}
	body, err := fs.ReadFile(h.static, representationName)
	if err != nil {
		return false
	}
	w.Header().Set("Cache-Control", cacheControlFor(name))
	w.Header().Set("ETag", contentETag(body))
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	return true
}

func acceptsBrotli(header string) bool {
	for _, coding := range strings.Split(header, ",") {
		parts := strings.Split(coding, ";")
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "br") {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(parameter, "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				return false
			}
			quality = parsed
		}
		return quality > 0
	}
	return false
}

func cacheControlFor(name string) string {
	switch {
	case strings.HasPrefix(name, "assets/"):
		return "public, max-age=31536000, immutable"
	case strings.HasPrefix(name, "brand/"):
		return "public, max-age=86400"
	default:
		return "no-cache"
	}
}

func contentETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func defaultReservedPrefixes() []string {
	return []string{"/v1", "/mcp", "/s"}
}

func normalizeReservedPrefixes(prefixes []string) []string {
	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		prefix = path.Clean(prefix)
		if prefix == "." {
			continue
		}
		if prefix != "/" {
			prefix = strings.TrimRight(prefix, "/")
		}
		out = append(out, prefix)
	}
	return out
}

func isReservedPath(rawPath string, prefixes []string) bool {
	cleanPath := path.Clean("/" + rawPath)
	for _, prefix := range prefixes {
		if prefix == "/" {
			return true
		}
		if cleanPath == prefix || strings.HasPrefix(cleanPath, prefix+"/") {
			return true
		}
	}
	return false
}

func isManagementPath(rawPath string, reservedPrefixes []string) bool {
	if rawPath == "/" {
		return true
	}
	if isReservedPath(rawPath, reservedPrefixes) {
		return false
	}
	return !strings.HasPrefix(path.Base(rawPath), ".")
}
