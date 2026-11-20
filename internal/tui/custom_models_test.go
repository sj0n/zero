package tui

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Twigpine/zero/internal/config"
	"github.com/Twigpine/zero/internal/modelregistry"
	"github.com/Twigpine/zero/internal/providermodeldiscovery"
	"github.com/Twigpine/zero/internal/zeroruntime"
)

func TestConfiguredModelsAndEffortsOverrideDiscovery(t *testing.T) {
	var profile config.ProviderProfile
	if err := json.Unmarshal([]byte(`{"name":"proxy","catalogID":"custom-openai-compatible","provider_kind":"openai-compatible","baseURL":"https://example.test/v1","model":"model-a","models":[{"id":"model-a","reasoningEfforts":["low","turbo"]},{"id":"model-b","reasoningEfforts":[]}]}`), &profile); err != nil {
		t.Fatal(err)
	}
	m := newModel(context.Background(), Options{ProviderName: "proxy", ModelName: "model-a", ProviderProfile: profile})
	m.modelPickerLiveByProvider = map[string][]providermodeldiscovery.Model{
		"proxy": {{ID: "model-a", ReasoningEfforts: []string{"high"}}, {ID: "model-live"}},
	}
	want := []modelregistry.ReasoningEffort{"low", "turbo"}
	if got := m.availableReasoningEfforts(); !reflect.DeepEqual(got, want) {
		t.Fatalf("configured efforts = %v, want %v", got, want)
	}
	picker := m.newModelPicker()
	if picker == nil || !contains(pickerValues(picker.items), "model-b") || !contains(pickerValues(picker.items), "model-live") {
		t.Fatalf("picker must include configured and discovered models: %#v", picker)
	}
	m, _ = m.handleEffortCommand("turbo")
	if m.reasoningEffort != "turbo" {
		t.Fatalf("effort = %q, want turbo", m.reasoningEffort)
	}
	m.modelName = "model-b"
	if got := m.availableReasoningEfforts(); len(got) != 0 {
		t.Fatalf("explicit empty efforts = %v, want none", got)
	}
}

func TestConfiguredModelSwitchAndProviderIsolation(t *testing.T) {
	t.Setenv(config.ActiveProviderEnv, "")
	var first, second config.ProviderProfile
	for _, tc := range []struct {
		profile *config.ProviderProfile
		data    string
	}{
		{&first, `{"name":"first","catalogID":"custom-openai-compatible","provider_kind":"openai-compatible","baseURL":"https://first.test/v1","apiKey":"test","model":"same","models":[{"id":"same","reasoningEfforts":["turbo"]},{"id":"other","reasoningEfforts":[]}]}`},
		{&second, `{"name":"second","catalogID":"custom-openai-compatible","provider_kind":"openai-compatible","baseURL":"https://second.test/v1","apiKey":"test","model":"same","models":[{"id":"same","reasoningEfforts":["low"]}]}`},
	} {
		if err := json.Unmarshal([]byte(tc.data), tc.profile); err != nil {
			t.Fatal(err)
		}
	}
	m := newModel(context.Background(), Options{ProviderName: first.Name, ModelName: first.Model, ProviderProfile: first,
		SavedProviders: []config.ProviderProfile{first, second}, NewProvider: func(config.ProviderProfile) (zeroruntime.Provider, error) { return &fakeProvider{}, nil }})
	picker := m.newModelPicker()
	if picker == nil {
		t.Fatal("missing picker")
	}
	var firstSeen, secondSeen bool
	for _, item := range picker.items {
		if item.Value == "same" && item.OwnerProvider == "first" {
			firstSeen = true
		}
		if item.Value == "same" && item.OwnerProvider == "second" {
			secondSeen = true
		}
	}
	if !firstSeen || !secondSeen {
		t.Fatalf("both provider/model pairs must be selectable: %#v", picker.items)
	}
	m.reasoningEffort = "turbo"
	m, status, err := m.handleModelCommand("other")
	if err != nil || m.modelName != "other" || m.reasoningEffort != "" {
		t.Fatalf("switch to empty effort ring: model=%q effort=%q status=%s err=%v", m.modelName, m.reasoningEffort, status, err)
	}
	m.reasoningEffort = "turbo"
	m, status, ok, _, err := m.switchProviderModel("second", "same")
	if err != nil || !ok || m.reasoningEffort != "" || !strings.Contains(status, "Switched") {
		t.Fatalf("cross-provider switch: effort=%q status=%s ok=%v err=%v", m.reasoningEffort, status, ok, err)
	}
	if got := m.availableReasoningEfforts(); !reflect.DeepEqual(got, []modelregistry.ReasoningEffort{"low"}) {
		t.Fatalf("second provider efforts = %v", got)
	}
}
