package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCustomModelConfigSurvivesModelSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{"activeProvider":"proxy","providers":[{"name":"proxy","provider_kind":"openai-compatible","baseURL":"https://example.test/v1","model":"a","models":[{"id":"a","reasoningEfforts":["low","turbo"]},{"id":"b","reasoningEfforts":[]},{"id":"c"}]}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SetProviderModel(path, "proxy", "b"); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, issues := ValidateBytes(written)
	if len(issues) != 0 {
		t.Fatalf("validation issues: %v", issues)
	}
	profile := cfg.Providers[0]
	if profile.Model != "b" || len(profile.Models) != 3 {
		t.Fatalf("profile after selection = %#v", profile)
	}
	if got, ok := profile.ConfiguredReasoningEfforts("a"); !ok || !reflect.DeepEqual(got, []string{"low", "turbo"}) {
		t.Fatalf("a levels = %v, configured %v", got, ok)
	}
	if got, ok := profile.ConfiguredReasoningEfforts("b"); !ok || got == nil || len(got) != 0 {
		t.Fatalf("b levels = %v, configured %v; want explicit empty list", got, ok)
	}
	if _, ok := profile.ConfiguredReasoningEfforts("c"); ok {
		t.Fatal("omitted levels must use discovery")
	}
}

func TestCustomModelConfigRejectsInvalidEntries(t *testing.T) {
	for _, models := range []string{
		`[{"id":""}]`,
		`[{"id":"a"},{"id":"a"}]`,
		`[{"id":"a","reasoningEfforts":["auto"]}]`,
		`[{"id":"a","reasoningEfforts":["list"]}]`,
		`[{"id":"a","reasoningEfforts":["high","HIGH"]}]`,
		`[{"id":"a","reasoningEfforts":["bad value"]}]`,
	} {
		t.Run(models, func(t *testing.T) {
			data := `{"activeProvider":"proxy","providers":[{"name":"proxy","provider_kind":"openai-compatible","baseURL":"https://example.test/v1","model":"a","models":` + models + `}]}`
			_, issues := ValidateBytes([]byte(data))
			if len(issues) == 0 {
				t.Fatalf("expected invalid config: %s", models)
			}
		})
	}
}

func TestCustomModelLayerMergePreservesEmptyAndCopies(t *testing.T) {
	var base, overlay ProviderProfile
	if err := json.Unmarshal([]byte(`{"name":"proxy","models":[{"id":"a","reasoningEfforts":["low"]}]}`), &base); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"name":"proxy","models":[{"id":"b","reasoningEfforts":[]}]}`), &overlay); err != nil {
		t.Fatal(err)
	}
	merged := mergeProfile(base, overlay)
	if len(merged.Models) != 1 || merged.Models[0].ID != "b" || merged.Models[0].ReasoningEfforts == nil {
		t.Fatalf("merge = %#v", merged.Models)
	}
	merged.Models[0].ID = "changed"
	if strings.TrimSpace(overlay.Models[0].ID) != "b" {
		t.Fatal("merge mutated its source")
	}
}
