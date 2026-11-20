package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Twigpine/zero/internal/config"
	"github.com/Twigpine/zero/internal/zeroruntime"
)

func TestConfiguredCustomReasoningEffortReachesEndpoint(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	var profile config.ProviderProfile
	if err := json.Unmarshal([]byte(`{"name":"proxy","provider_kind":"openai-compatible","model":"model-a","models":[{"id":"model-a","reasoningEfforts":["turbo","none"]}]}`), &profile); err != nil {
		t.Fatal(err)
	}
	profile.BaseURL = server.URL
	provider, err := New(profile, Options{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ requested, want string }{{"turbo", "turbo"}, {"TURBO", "turbo"}, {"none", "none"}, {"", ""}, {"bogus", ""}, {"high", ""}} {
		body = nil
		stream, err := provider.StreamCompletion(context.Background(), zeroruntime.CompletionRequest{ReasoningEffort: tc.requested})
		if err != nil {
			t.Fatal(err)
		}
		for range stream {
		}
		got, present := body["reasoning_effort"]
		if tc.want == "" && present {
			t.Fatalf("effort %q must be omitted, got %v", tc.requested, got)
		}
		if tc.want != "" && got != tc.want {
			t.Fatalf("effort %q = %v, want %q", tc.requested, got, tc.want)
		}
	}
}

func TestConfiguredEffortsSetProviderCapabilities(t *testing.T) {
	profile := config.ProviderProfile{Name: "proxy", ProviderKind: config.ProviderKindOpenAICompatible,
		BaseURL: "https://example.test/v1", Model: "gpt-4.1",
		Models: []config.ProviderModel{{ID: "gpt-4.1", ReasoningEfforts: []string{"turbo"}}}}
	provider, err := NewTurnSessionProvider(profile, Options{})
	if err != nil {
		t.Fatal(err)
	}
	caps := provider.Capabilities()
	if !caps.SupportsReasoning || len(caps.ReasoningEfforts) != 1 || caps.ReasoningEfforts[0] != "turbo" {
		t.Fatalf("configured capabilities = %#v", caps)
	}
	profile.Models[0].ReasoningEfforts = []string{}
	provider, err = NewTurnSessionProvider(profile, Options{})
	if err != nil {
		t.Fatal(err)
	}
	caps = provider.Capabilities()
	if caps.SupportsReasoning || len(caps.ReasoningEfforts) != 0 {
		t.Fatalf("empty configured capabilities = %#v", caps)
	}
}
