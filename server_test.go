package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

const sampleImage = "data:image/jpeg;base64,/9j/AA=="
const sampleBody = `{"name":"PLA Green","image":"` + sampleImage + `","photoHex":"#123456"}`
const sampleCandidate = `{"match":true,"name":"PLA Green","hex":"#aabbcc","method":"manufacturer","sourceUrl":"https://maker.example/pla","note":"Výrobca uvádza HEX."}`

func sampleResponse(candidate string) string {
	return `{"output":[{"type":"web_search_call","action":{"sources":[{"url":"https://maker.example/pla"}]}},{"type":"message","content":[{"type":"output_text","text":` + fmt.Sprintf("%q", candidate) + `}]}]}`
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testApp(fn transportFunc) *app {
	return newApp(config{APIKey: "test-key", Model: "gpt-4.1"}, &http.Client{Transport: fn})
}
func request(a *app, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.10:1234"
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func jsonHeaders() map[string]string { return map[string]string{"Content-Type": "application/json"} }
func TestEmbeddedAssets(t *testing.T) {
	a := newApp(config{}, http.DefaultClient)
	for name, want := range map[string]string{"/": "text/html", "/index.html": "text/html", "/app.mjs": "text/javascript", "/color.mjs": "text/javascript", "/sw.js": "text/javascript", "/style.css": "text/css", "/manifest.webmanifest": "application/manifest+json", "/icon-192.png": "image/png", "/icon-512.png": "image/png", "/icon.svg": "image/svg+xml"} {
		w := request(a, "GET", name, "", nil)
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), want) || w.Body.Len() == 0 {
			t.Errorf("%s: %d %s", name, w.Code, w.Header().Get("Content-Type"))
		}
		if w.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("missing security headers")
		}
	}
	for _, name := range []string{"/.env", "/main.go", "/server.go", "/public/index.html", "/../main.go", "/%2e%2e/main.go", "/missing", "/public/", "/go.mod"} {
		if w := request(a, "GET", name, "", nil); w.Code != 404 {
			t.Errorf("%s: %d", name, w.Code)
		}
	}
	w := request(a, "HEAD", "/", "", nil)
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("HEAD must return headers without content")
	}
	if w := request(a, "POST", "/", "", nil); w.Code != 405 {
		t.Fatal(w.Code)
	}
}
func TestInputValidation(t *testing.T) {
	for _, tc := range []struct {
		name, image, hex string
		valid            bool
	}{
		{" PLA ", sampleImage, "#123abc", true}, {"", sampleImage, "#123456", false}, {strings.Repeat("á", 181), sampleImage, "#123456", false},
		{"PLA", "https://example.com/image", "#123456", false}, {"PLA", "data:image/jpeg;base64,AAAA", "#123456", false},
		{"PLA", sampleImage, "red", false}, {"PLA", "data:image/jpeg;base64,???", "#123456", false},
	} {
		input := identifyInput{Name: tc.name, Image: tc.image, PhotoHex: tc.hex}
		err := input.validate()
		if (err == nil) != tc.valid {
			t.Errorf("%q: %v", tc.name, err)
		}
		if tc.valid && input.Name != "PLA" {
			t.Fatal("name not trimmed")
		}
	}
}
func TestCitedResult(t *testing.T) {
	cases := []struct {
		name, candidate string
		match           bool
	}{
		{"official", sampleCandidate, true},
		{"invented source", strings.ReplaceAll(sampleCandidate, "maker.example", "invented.example"), false},
		{"invalid hex", strings.ReplaceAll(sampleCandidate, "#aabbcc", "red"), false},
		{"unsafe source", strings.ReplaceAll(sampleCandidate, "https://maker.example", "javascript:alert(1)//"), false},
		{"invalid method", strings.ReplaceAll(sampleCandidate, "manufacturer", "guessed"), false},
		{"no match", `{"match":false}`, false},
		{"visual estimate", strings.ReplaceAll(sampleCandidate, "manufacturer", "visual"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var upstream upstreamResponse
			if err := json.Unmarshal([]byte(sampleResponse(tc.candidate)), &upstream); err != nil {
				t.Fatal(err)
			}
			result, err := parseResult(upstream)
			if err != nil || result.Match != tc.match {
				t.Fatalf("%+v %v", result, err)
			}
			if tc.match && (result.Result.Hex != "#AABBCC" || len(result.Result.Sources) != 1) {
				t.Fatalf("%+v", result)
			}
		})
	}
	var upstream upstreamResponse
	if _, err := parseResult(upstream); err == nil {
		t.Fatal("empty response accepted")
	}
	// A citation annotation is also evidence even without action.sources.
	raw := `{"output":[{"type":"message","content":[{"type":"output_text","text":` + fmt.Sprintf("%q", sampleCandidate) + `,"annotations":[{"type":"url_citation","url":"https://maker.example/pla"}]}]}]}`
	_ = json.Unmarshal([]byte(raw), &upstream)
	if result, err := parseResult(upstream); err != nil || !result.Match {
		t.Fatalf("citation rejected: %v", err)
	}
}
func TestIdentifyRequest(t *testing.T) {
	calls := 0
	a := testApp(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://api.openai.com/v1/responses" || r.Header.Get("Authorization") != "Bearer test-key" || r.Method != "POST" {
			t.Fatal("incorrect upstream request")
		}
		var payload struct {
			Model string `json:"model"`
			Store bool   `json:"store"`
			Tools []struct {
				Type string `json:"type"`
			} `json:"tools"`
			Input []struct {
				Content []struct {
					Type     string `json:"type"`
					ImageURL string `json:"image_url"`
				} `json:"content"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "gpt-4.1" || payload.Store || len(payload.Tools) != 1 || payload.Tools[0].Type != "web_search" || payload.Input[0].Content[1].ImageURL != sampleImage {
			t.Fatalf("incorrect payload: %+v", payload)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(sampleResponse(sampleCandidate))), Header: make(http.Header)}, nil
	})
	w := request(a, "POST", "/api/identify", sampleBody, jsonHeaders())
	var result identifyResult
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || !result.Match || result.Result.Hex != "#AABBCC" || calls != 1 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
func TestRequestGuards(t *testing.T) {
	for _, tc := range []struct {
		name, method, body string
		headers            map[string]string
		key, password      string
		status             int
	}{
		{"method", "GET", "", nil, "key", "", 405},
		{"key missing", "POST", sampleBody, jsonHeaders(), "", "", 503},
		{"password missing", "POST", sampleBody, jsonHeaders(), "key", "secret", 401},
		{"password wrong", "POST", sampleBody, map[string]string{"Content-Type": "application/json", "Authorization": "Bearer wrong"}, "key", "secret", 401},
		{"cross origin", "POST", sampleBody, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, "key", "", 403},
		{"null origin", "POST", sampleBody, map[string]string{"Content-Type": "application/json", "Origin": "null"}, "key", "", 403},
		{"content type", "POST", sampleBody, nil, "key", "", 415},
		{"invalid JSON", "POST", "{", jsonHeaders(), "key", "", 400},
		{"invalid input", "POST", "{}", jsonHeaders(), "key", "", 400},
		{"oversize", "POST", strings.Repeat("x", int(bodyLimit)+1), jsonHeaders(), "key", "", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(func(r *http.Request) (*http.Response, error) { t.Fatal("guard contacted API"); return nil, nil })
			a.config.APIKey = tc.key
			a.config.Password = tc.password
			w := request(a, tc.method, "/api/identify", tc.body, tc.headers)
			if w.Code != tc.status {
				t.Fatalf("want %d got %d: %s", tc.status, w.Code, w.Body.String())
			}
			if a.active != 0 {
				t.Fatal("slot leaked")
			}
		})
	}
	a := testApp(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(sampleResponse(`{"match":false}`)))}, nil
	})
	a.config.Password = "secret"
	headers := map[string]string{"Content-Type": "application/json; charset=utf-8", "Authorization": "Bearer secret", "Origin": "http://example.com"}
	if w := request(a, "POST", "http://example.com/api/identify", sampleBody, headers); w.Code != 200 {
		t.Fatalf("valid auth/origin rejected: %s", w.Body.String())
	}
}
func TestUpstreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
		err  error
	}{
		{"quota", 429, "", nil}, {"auth", 401, "", nil}, {"invalid JSON", 200, "{", nil}, {"invalid result", 200, `{"output":[]}`, nil}, {"timeout", 0, "", context.DeadlineExceeded}, {"network", 0, "", fmt.Errorf("https://secret.example?key=do-not-leak")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(func(r *http.Request) (*http.Response, error) {
				if tc.err != nil {
					return nil, tc.err
				}
				return &http.Response{StatusCode: tc.code, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			w := request(a, "POST", "/api/identify", sampleBody, jsonHeaders())
			if w.Code != 502 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "do-not-leak") {
				t.Fatal("upstream details leaked")
			}
			if a.active != 0 {
				t.Fatal("slot leaked")
			}
		})
	}
}
func TestLimits(t *testing.T) {
	a := newApp(config{}, http.DefaultClient)
	for i := 0; i < 6; i++ {
		if !a.reserve("same") {
			t.Fatal("early rate limit")
		}
		a.release()
	}
	if a.reserve("same") {
		t.Fatal("rate limit missing")
	}
	a.clients["same"] = clientWindow{Start: time.Now().Add(-time.Minute), Count: 6}
	if !a.reserve("same") {
		t.Fatal("window did not expire")
	}
	a.release()
	var wg sync.WaitGroup
	release := make(chan struct{})
	results := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok := a.reserve(fmt.Sprint(i))
			results <- ok
			if ok {
				<-release
				a.release()
			}
		}(i)
	}
	accepted := 0
	for i := 0; i < 20; i++ {
		if <-results {
			accepted++
		}
	}
	close(release)
	wg.Wait()
	if accepted != 3 {
		t.Fatalf("concurrent reservations: got %d, want 3", accepted)
	}
	if a.active != 0 {
		t.Fatal("active requests leaked")
	}
	a.config.APIKey = "test-key"
	a.active = 3
	w := request(a, "POST", "/api/identify", sampleBody, jsonHeaders())
	if w.Code != 429 || w.Header().Get("Retry-After") != "60" {
		t.Fatal("concurrency limit response missing")
	}

}
func TestConfig(t *testing.T) {
	dir := t.TempDir()
	filename := dir + "/.env"
	text := "# test\nPORT=4173\nexport HOST=127.0.0.1\nOPENAI_API_KEY='file-key'\nOPENAI_MODEL=\"gpt-4.1\"\nLOOKUP_PASSWORD='a#b=c'\n"
	if err := os.WriteFile(filename, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(filename, func(key string) (string, bool) {
		if key == "OPENAI_API_KEY" {
			return "environment-key", true
		}
		return "", false
	})
	if err != nil || cfg.Port != "4173" || cfg.APIKey != "environment-key" || cfg.Password != "a#b=c" || cfg.Host != "127.0.0.1" {
		t.Fatalf("%+v %v", cfg, err)
	}
	if _, err := loadConfig(filename+"missing", os.LookupEnv); err == nil {
		t.Fatal("explicit missing file accepted")
	}
	if _, err := loadConfig("", func(key string) (string, bool) { return "", key == "PORT" }); err == nil {
		t.Fatal("bad port accepted")
	}
	if err := os.WriteFile(filename, []byte("LOOKUP_PASSWORD='unterminated"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(filename, func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("bad dotenv accepted")
	}
}
