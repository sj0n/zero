package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Twigpine/zero/internal/zeroruntime"
)

func TestReasoningDiagnostic(t *testing.T) {
	const secret = "sk-proj-ABCDEFGH1234567890abcdefghijkl"
	const headerSecret = "HEADER_SECRET_12345678"
	if os.Getenv("ZERO_DIAGNOSTIC_TEST_CHILD") == "1" {
		var body map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		}))
		defer server.Close()
		model, level, apiKey := "model-a", os.Getenv("ZERO_DIAGNOSTIC_TEST_LEVEL"), secret
		if level == "sensitive" {
			model = "before-" + secret[:9] + "\x00" + secret[9:] + "-after"
			level = headerSecret[:8] + "\x1b" + headerSecret[8:]
		}
		invalidHeader := level == "credential-control"
		if invalidHeader {
			apiKey = "opaque" + "\x00" + "value"
			model = "before-" + apiKey + "-after"
			level = "turbo"
		}
		allowed := []string{"turbo", level}
		if level == "unsupported" {
			allowed = []string{"turbo"}
		}
		provider, err := New(Options{BaseURL: server.URL, Model: model, APIKey: apiKey,
			CustomHeaders: map[string]string{"X-Custom-Key": headerSecret}, ReasoningEfforts: allowed})
		if err != nil {
			t.Fatal(err)
		}
		request := zeroruntime.CompletionRequest{
			ReasoningEffort: level,
			Messages:        []zeroruntime.Message{{Role: zeroruntime.MessageRoleUser, Content: "PRIVATE_PROMPT_123456"}},
		}
		if level == "marshal-failure" {
			request.Tools = []zeroruntime.ToolDefinition{{Name: "invalid", Parameters: map[string]any{"bad": make(chan int)}}}
			if _, err := provider.StreamCompletion(context.Background(), request); err == nil {
				t.Fatal("expected marshaling failure")
			}
			return
		}
		stream, err := provider.StreamCompletion(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		for range stream {
		}
		if invalidHeader {
			if body != nil {
				t.Fatal("invalid credential must fail before dispatch")
			}
			return
		}
		if body["model"] != model {
			t.Fatal("model was not sent")
		}
		if level == "" || level == "unsupported" {
			if _, ok := body["reasoning_effort"]; ok {
				t.Fatal("unexpected effort")
			}
		} else if body["reasoning_effort"] != level {
			t.Fatal("wrong effort")
		}
		return
	}

	for _, tc := range []struct {
		name, enabled, level, expected string
	}{
		{"disabled", "", "turbo", ""},
		{"explicit false", "0", "turbo", ""},
		{"custom", "1", "turbo", `model="model-a" reasoning_effort="turbo"`},
		{"omitted", "true", "", `model="model-a" reasoning_effort=<omitted>`},
		{"unsupported", "1", "unsupported", `model="model-a" reasoning_effort=<omitted>`},
		{"marshal failure", "1", "marshal-failure", ""},
		{"invalid credential", "1", "credential-control", `model="before-[REDACTED]-after" reasoning_effort="turbo"`},
		{"secrets split by control bytes", "1", "sensitive", `model="before-[REDACTED]-after" reasoning_effort="[REDACTED]"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestReasoningDiagnostic$")
			cmd.Env = append(os.Environ(), "ZERO_DIAGNOSTIC_TEST_CHILD=1", "ZERO_REASONING_DIAGNOSTIC="+tc.enabled, "ZERO_DIAGNOSTIC_TEST_LEVEL="+tc.level)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("child failed: %v: stdout=%s stderr=%s", err, stdout.String(), stderr.String())
			}
			text := stderr.String()
			if strings.Contains(stdout.String(), "reasoning_effort=") {
				t.Fatalf("diagnostic leaked to stdout: %s", stdout.String())
			}
			if tc.expected == "" && strings.Contains(text, "reasoning_effort=") {
				t.Fatalf("diagnostic not disabled: %s", text)
			}
			if tc.expected != "" && !strings.Contains(text, tc.expected) {
				t.Fatalf("diagnostic %q does not contain %q", text, tc.expected)
			}
			for _, forbidden := range []string{secret, headerSecret, "PRIVATE_PROMPT_123456", "\x00", "\x1b"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("diagnostic leaked private data or control bytes: %q", text)
				}
			}
		})
	}
}
