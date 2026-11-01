package main

import (
	"encoding/json"
	"errors"
)

func openRouterPayload(model, description, image string) map[string]any {
	return map[string]any{
		"model":           model,
		"max_tokens":      4096,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []any{
			map[string]string{"role": "system", "content": identifyInstructions},
			map[string]any{"role": "user", "content": []any{
				map[string]string{"type": "text", "text": description},
				map[string]any{"type": "image_url", "image_url": map[string]string{"url": image}},
			}},
		},
		// Exa runs a search before answering even on models without native search.
		// This also keeps JSON mode compatible with Gemini 2.5 and other vision models.
		"plugins": []any{map[string]any{"id": "web", "engine": "exa", "max_results": 5,
			"search_prompt": "Use these web search results to identify the exact filament on its original manufacturer website. Return only the requested JSON object. Set sourceUrl to the exact manufacturer URL from these results; never invent a URL or use a reseller as manufacturer. If none can be verified, return {\"match\":false}."}},
	}
}

func parseOpenRouterResult(data []byte) (identifyResult, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content     string `json:"content"`
				Annotations []struct {
					Type     string `json:"type"`
					Citation source `json:"url_citation"`
				} `json:"annotations"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &response); err != nil || len(response.Choices) == 0 {
		return identifyResult{}, errors.New("Vyhľadávanie nevrátilo platnú odpoveď.")
	}
	message := response.Choices[0].Message
	sources := make(map[string]bool)
	for _, annotation := range message.Annotations {
		if annotation.Type == "url_citation" {
			if u := sourceURL(annotation.Citation.URL); u != "" {
				sources[u] = true
			}
		}
	}
	return parseCandidate(message.Content, sources)
}
