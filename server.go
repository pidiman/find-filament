package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

//go:embed public
var assets embed.FS

const bodyLimit int64 = 3 * 1024 * 1024

type clientWindow struct {
	Start time.Time
	Count int
}
type app struct {
	config  config
	client  *http.Client
	mu      sync.Mutex
	clients map[string]clientWindow
	active  int
}

func newApp(cfg config, client *http.Client) *app {
	return &app{config: cfg, client: client, clients: make(map[string]clientWindow)}
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
	if r.URL.Path == "/api/identify" {
		a.identifyHandler(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		fail(w, 405, "Nepodporovaná metóda.")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		fail(w, 404, "Nenájdené.")
		return
	}
	for _, segment := range strings.Split(name, "/") {
		if strings.HasPrefix(segment, ".") {
			fail(w, 404, "Nenájdené.")
			return
		}
	}
	data, err := assets.ReadFile("public/" + name)
	if err != nil {
		fail(w, 404, "Nenájdené.")
		return
	}
	contentType := mime.TypeByExtension(path.Ext(name))
	switch path.Ext(name) {
	case ".mjs", ".js":
		contentType = "text/javascript; charset=utf-8"
	case ".webmanifest":
		contentType = "application/manifest+json"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
func (a *app) reserve(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for key, value := range a.clients {
		if now.Sub(value.Start) >= time.Minute {
			delete(a.clients, key)
		}
	}
	window, exists := a.clients[ip]
	if !exists {
		window.Start = now
	}
	if window.Count >= 6 || a.active >= 3 || (!exists && len(a.clients) >= 10000) {
		return false
	}
	window.Count++
	a.clients[ip] = window
	a.active++
	return true
}
func (a *app) release() { a.mu.Lock(); a.active--; a.mu.Unlock() }
func (a *app) identifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		fail(w, 405, "Použi POST.")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Host != r.Host || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			fail(w, 403, "Cudzí pôvod požiadavky.")
			return
		}
	}
	if a.config.Password != "" {
		expected := sha256.Sum256([]byte("Bearer " + a.config.Password))
		got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(expected[:], got[:]) != 1 {
			fail(w, 401, "Online vyhľadávanie vyžaduje heslo. Zadaj ho v nastaveniach.")
			return
		}
	}
	if a.config.APIKey == "" {
		fail(w, 503, "Online vyhľadávanie ešte nie je nastavené. Na serveri doplň OPENAI_API_KEY.")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		fail(w, 415, "Vyžaduje sa JSON.")
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !a.reserve(ip) {
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "Príliš veľa vyhľadávaní. Skús o minútu.")
		return
	}
	defer a.release()
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, bodyLimit))
	if err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			fail(w, 413, "Fotografia je príliš veľká.")
		} else {
			fail(w, 400, "Požiadavku sa nepodarilo prečítať.")
		}
		return
	}
	var input identifyInput
	if err := json.Unmarshal(data, &input); err != nil {
		fail(w, 400, "Neplatný JSON.")
		return
	}
	if err := input.validate(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	result, err := a.identify(r.Context(), input)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		fail(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, result)
}
