package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Twigpine/zero/internal/config"
	"github.com/Twigpine/zero/internal/providers"
	"github.com/Twigpine/zero/internal/zeroruntime"
)

func TestExecReasoningDiagnosticMatchesRequest(t *testing.T) {
	if os.Getenv("ZERO_REASONING_CLI_TEST_CHILD") == "1" {
		var body map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
		}))
		defer server.Close()
		root := os.Getenv("ZERO_REASONING_CLI_TEST_ROOT")
		profile := config.ProviderProfile{Name: "proxy", ProviderKind: config.ProviderKindOpenAICompatible,
			BaseURL: server.URL, Model: "model-a", APIKey: "test-key",
			Models: []config.ProviderModel{{ID: "model-a", ReasoningEfforts: []string{"turbo"}}}}
		var stdout, stderr bytes.Buffer
		exit := runWithDeps([]string{"exec", "-r", "turbo", "private prompt"}, &stdout, &stderr, appDeps{
			getwd: func() (string, error) { return root, nil },
			resolveConfig: func(_ string, _ config.Overrides) (config.ResolvedConfig, error) {
				return config.ResolvedConfig{ActiveProvider: profile.Name, Provider: profile, MaxTurns: 3}, nil
			},
			newProvider: func(p config.ProviderProfile) (zeroruntime.Provider, error) {
				return providers.New(p, providers.Options{HTTPClient: server.Client()})
			},
		})
		if exit != 0 || body["model"] != "model-a" || body["reasoning_effort"] != "turbo" {
			t.Fatalf("exit=%d body effort=%v stdout=%s stderr=%s", exit, body["reasoning_effort"], stdout.String(), stderr.String())
		}
		return
	}

	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestExecReasoningDiagnosticMatchesRequest$")
	cmd.Env = append(os.Environ(), "ZERO_REASONING_CLI_TEST_CHILD=1", "ZERO_REASONING_DIAGNOSTIC=1", "ZERO_REASONING_CLI_TEST_ROOT="+root,
		"HOME="+root, "USERPROFILE="+root, "APPDATA="+root, "LOCALAPPDATA="+root,
		"XDG_CONFIG_HOME="+root, "XDG_CACHE_HOME="+root, "XDG_DATA_HOME="+root)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("child failed: %v: stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `model="model-a" reasoning_effort="turbo"`) {
		t.Fatalf("missing diagnostic: %s", stderr.String())
	}
	for _, secret := range []string{"private prompt", "test-key"} {
		if strings.Contains(stderr.String(), secret) {
			t.Fatalf("diagnostic leaked %q", secret)
		}
	}
}
