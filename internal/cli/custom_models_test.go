package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Twigpine/zero/internal/config"
	"github.com/Twigpine/zero/internal/zeroruntime"
)

type captureReasoningProvider struct{ effort string }

func (p *captureReasoningProvider) StreamCompletion(ctx context.Context, request zeroruntime.CompletionRequest) (<-chan zeroruntime.StreamEvent, error) {
	p.effort = request.ReasoningEffort
	return echoExecProvider{}.StreamCompletion(ctx, request)
}

func TestExecUsesConfiguredModelReasoningEffort(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME"} {
		t.Setenv(key, root)
	}
	var profile config.ProviderProfile
	if err := json.Unmarshal([]byte(`{"name":"proxy","provider_kind":"openai-compatible","baseURL":"https://example.test/v1","model":"gpt-4.1","models":[{"id":"gpt-4.1","reasoningEfforts":["turbo"]}]}`), &profile); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ requested, want string }{{"turbo", "turbo"}, {"unknown", ""}, {"auto", ""}} {
		t.Run(tc.requested, func(t *testing.T) {
			provider := &captureReasoningProvider{}
			var stdout, stderr bytes.Buffer
			exit := runWithDeps([]string{"exec", "-r", tc.requested, "hello"}, &stdout, &stderr, appDeps{
				getwd: func() (string, error) { return root, nil },
				resolveConfig: func(_ string, _ config.Overrides) (config.ResolvedConfig, error) {
					return config.ResolvedConfig{ActiveProvider: profile.Name, Provider: profile, MaxTurns: 3}, nil
				},
				newProvider: func(config.ProviderProfile) (zeroruntime.Provider, error) { return provider, nil },
			})
			if exit != 0 || provider.effort != tc.want {
				t.Fatalf("exit=%d effort=%q, want %q; stderr=%s", exit, provider.effort, tc.want, stderr.String())
			}
			if tc.requested == "unknown" && !strings.Contains(stderr.String(), "not configured") {
				t.Fatalf("missing unsupported effort warning: %s", stderr.String())
			}
		})
	}
}
