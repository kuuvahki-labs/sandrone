// Package webui serves optional Sandrone Web UI static assets.
package webui

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Option func(*options)

type options struct {
	static           fs.FS
	staticDir        string
	reservedPrefixes []string
}

func Handler(opts ...Option) http.Handler {
	cfg := options{reservedPrefixes: defaultReservedPrefixes()}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.static == nil {
		cfg.static = staticFS(cfg.staticDir)
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

func WithStaticDir(dir string) Option {
	return func(cfg *options) {
		cfg.staticDir = dir
	}
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
	body, err := fs.ReadFile(h.static, name)
	if err != nil {
		return false
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	return true
}

func staticFS(staticDir string) fs.FS {
	if staticDir != "" {
		if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
			return os.DirFS(staticDir)
		}
		return nil
	}
	dirs := []string{
		filepath.Join("internal", "entry", "webui", "static"),
		"static",
	}
	if _, filename, _, ok := runtime.Caller(0); ok {
		dirs = append(dirs, filepath.Join(filepath.Dir(filename), "static"))
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			return os.DirFS(dir)
		}
	}
	return nil
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
