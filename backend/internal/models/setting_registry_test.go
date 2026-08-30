package models

import "testing"

func TestUnstatedVisibilityKeepsAValueOnTheServer(t *testing.T) {
	// A definition added without thinking about visibility must not reach a
	// browser. Exposing a credential has to be something someone typed.
	var forgotten SettingDefinition

	if forgotten.Visibility != VisibilityServerOnly {
		t.Fatalf("zero visibility = %q, want server-only", forgotten.Visibility)
	}
}

func TestRegistryKnowsTheMapsKey(t *testing.T) {
	definition, ok := LookupSetting(SettingGoogleMapsAPIKey)
	if !ok {
		t.Fatal("the Maps key must be a known setting")
	}
	if definition.Visibility != VisibilityBrowser {
		t.Fatalf("visibility = %q, want browser: the map cannot run server-side", definition.Visibility)
	}
	if definition.Label == "" {
		t.Fatal("a setting needs a label the settings page can show")
	}
}

func TestRegistryRejectsAnUnknownName(t *testing.T) {
	if _, ok := LookupSetting("anything_at_all"); ok {
		t.Fatal("the store must not accept names nobody declared")
	}
}

func TestDefinitionsCannotBeMutatedByCallers(t *testing.T) {
	// The registry decides who may read a credential. A caller that could edit
	// the returned slice could quietly widen that.
	first := SettingDefinitions()
	first[0].Visibility = VisibilityBrowser
	first[0].Name = "tampered"

	second := SettingDefinitions()
	if second[0].Name == "tampered" {
		t.Fatal("SettingDefinitions must hand out a copy")
	}
}
