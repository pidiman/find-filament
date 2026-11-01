package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func routerResponse(candidate, citation string) []byte {
	message := map[string]any{"content": candidate}
	if citation != "" {
		message["annotations"] = []any{map[string]any{"type": "url_citation", "url_citation": map[string]string{"url": citation, "title": "Manufacturer"}}}
	}
	data, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": message}}})
	return data
}
func TestOpenRouterCitations(t *testing.T) {
	for _, tc := range []struct {
		name, candidate, citation string
		match                     bool
	}{
		{"cited", sampleCandidate, "https://maker.example/pla", true},
		{"uncited", sampleCandidate, "", false},
		{"fabricated source", sampleCandidate, "https://different.example/pla", false},
		{"invalid hex", strings.ReplaceAll(sampleCandidate, "#aabbcc", "red"), "https://maker.example/pla", false},
		{"no match", `{"match":false}`, "https://maker.example/pla", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseOpenRouterResult(routerResponse(tc.candidate, tc.citation))
			if err != nil || result.Match != tc.match {
				t.Fatalf("%+v %v", result, err)
			}
			if tc.match && result.Result.Hex != "#AABBCC" {
				t.Fatal(result)
			}
		})
	}
	for _, data := range []string{`{`, `{}`, `{"error":{"message":"no credit"}}`, `{"choices":[{"message":{"content":null}}]}`} {
		if _, err := parseOpenRouterResult([]byte(data)); err == nil {
			t.Fatalf("accepted invalid response: %s", data)
		}
	}
}
func TestProviderConfig(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		env                  map[string]string
		provider, key, model string
		invalid              bool
	}{
		{name: "legacy default", env: map[string]string{"OPENAI_API_KEY": "openai-secret"}, provider: "openai", key: "openai-secret", model: "gpt-4.1"},
		{name: "openrouter default", env: map[string]string{"AI_PROVIDER": "openrouter", "OPENROUTER_API_KEY": "router-secret", "OPENAI_API_KEY": "wrong-secret"}, provider: "openrouter", key: "router-secret", model: defaultOpenRouterModel},
		{name: "custom model", env: map[string]string{"AI_PROVIDER": " OPENROUTER ", "OPENROUTER_API_KEY": " router-secret ", "OPENROUTER_MODEL": "google/gemini-2.5-flash"}, provider: "openrouter", key: "router-secret", model: "google/gemini-2.5-flash"},
		{name: "never reuse wrong key", env: map[string]string{"AI_PROVIDER": "openrouter", "OPENAI_API_KEY": "must-not-use"}, provider: "openrouter", model: defaultOpenRouterModel},
		{name: "openai never uses router key", env: map[string]string{"OPENROUTER_API_KEY": "must-not-use"}, provider: "openai", model: "gpt-4.1"},
		{name: "blank model default", env: map[string]string{"AI_PROVIDER": "openrouter", "OPENROUTER_MODEL": ""}, provider: "openrouter", model: defaultOpenRouterModel},
		{name: "bad provider", env: map[string]string{"AI_PROVIDER": "unknown"}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadConfig("", func(key string) (string, bool) { v, ok := tc.env[key]; return v, ok })
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid provider accepted")
				}
				return
			}
			if err != nil || cfg.Provider != tc.provider || cfg.APIKey != tc.key || cfg.Model != tc.model {
				t.Fatalf("unexpected provider configuration (error: %v)", err)
			}
		})
	}
}
func TestOpenRouterRequest(t *testing.T) {
	calls := 0
	a := testApp(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://openrouter.ai/api/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer router-key" || r.Header.Get("X-Title") != "Find Filament" {
			t.Fatal("incorrect router request")
		}
		var payload struct {
			Model   string `json:"model"`
			Plugins []struct {
				ID         string `json:"id"`
				Engine     string `json:"engine"`
				MaxResults int    `json:"max_results"`
			} `json:"plugins"`
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Format struct {
				Type string `json:"type"`
			} `json:"response_format"`
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != defaultOpenRouterModel || len(payload.Plugins) != 1 || payload.Plugins[0].ID != "web" || payload.Plugins[0].Engine != "exa" || payload.Plugins[0].MaxResults != 5 || payload.Format.Type != "json_object" {
			t.Fatal("missing model, search or JSON mode")
		}
		if len(payload.Messages) != 2 || payload.Messages[0].Role != "system" || payload.Messages[1].Role != "user" {
			t.Fatal("wrong messages")
		}
		var content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			ImageURL struct {
				URL string `json:"url"`
			} `json:"image_url"`
		}
		if err := json.Unmarshal(payload.Messages[1].Content, &content); err != nil {
			t.Fatal(err)
		}
		if len(content) != 2 || content[0].Type != "text" || !strings.Contains(content[0].Text, "PLA Green") || content[1].Type != "image_url" || content[1].ImageURL.URL != sampleImage {
			t.Fatal("missing product or image")
		}
		var fields map[string]any
		_ = json.Unmarshal(raw, &fields)
		if fields["tools"] != nil || fields["input"] != nil || fields["instructions"] != nil {
			t.Fatal("OpenAI request fields leaked into router request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(routerResponse(sampleCandidate, "https://maker.example/pla"))))}, nil
	})
	a.config.Provider = "openrouter"
	a.config.APIKey = "router-key"
	a.config.Model = defaultOpenRouterModel
	w := request(a, "POST", "/api/identify", sampleBody, jsonHeaders())
	var result identifyResult
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || !result.Match || calls != 1 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
func TestOpenRouterMissingKeyAndCredit(t *testing.T) {
	a := testApp(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 402, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"private account details"}}`))}, nil
	})
	a.config.Provider = "openrouter"
	a.config.APIKey = ""
	w := request(a, "POST", "/api/identify", sampleBody, jsonHeaders())
	if w.Code != 503 || !strings.Contains(w.Body.String(), "OPENROUTER_API_KEY") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	a.config.APIKey = "router-key"
	w = request(a, "POST", "/api/identify", sampleBody, jsonHeaders())
	if w.Code != 502 || !strings.Contains(w.Body.String(), "kredit") || strings.Contains(w.Body.String(), "private account") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	a.config.Provider = "invalid"
	if _, err := a.identify(context.Background(), identifyInput{Name: "PLA", Image: sampleImage, PhotoHex: "#123456"}); err == nil {
		t.Fatal("invalid provider used")
	}
}
