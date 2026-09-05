package models

// SettingVisibility says who may read a setting's value.
type SettingVisibility string

const (
	// VisibilityServerOnly keeps a value on the backend. It is deliberately the
	// zero value: a definition that says nothing about visibility does not
	// reach a browser, so exposing a credential takes a decision someone typed
	// rather than one they forgot.
	VisibilityServerOnly SettingVisibility = ""
	// VisibilityBrowser delivers a value to any authenticated user, for
	// features that cannot run anywhere else.
	VisibilityBrowser SettingVisibility = "browser"
)

// SettingGoogleMapsAPIKey drives the site map and address autocomplete.
const SettingGoogleMapsAPIKey = "google_maps_api_key"

// SettingGoogleMapsMapID names the Cloud map the pins are drawn on. Google's
// advanced markers render nothing without one, so an installation that leaves
// it unset gets a map with no plant on it.
const SettingGoogleMapsMapID = "google_maps_map_id"

// SettingDefinition describes a setting the installation understands.
type SettingDefinition struct {
	Name        string
	Label       string
	Description string
	Visibility  SettingVisibility
}

var settingRegistry = []SettingDefinition{
	{
		Name:        SettingGoogleMapsAPIKey,
		Label:       "Google Maps API key",
		Description: "Enables the site map and address autocomplete. This key is delivered to the browser and cannot be kept secret — restrict it to this site in Google Cloud Console.",
		Visibility:  VisibilityBrowser,
	},
	{
		Name:        SettingGoogleMapsMapID,
		Label:       "Google Maps Map ID",
		Description: "Identifies the map style the pins are drawn on. Create one under Map Management in the same Google Cloud project as the key above, with map type JavaScript and Vector. It is delivered to the browser and is not a secret.",
		Visibility:  VisibilityBrowser,
	},
}

// SettingDefinitions returns every known setting. The result is a copy: the
// registry decides who may read a credential, and a caller able to edit it
// could widen that silently.
func SettingDefinitions() []SettingDefinition {
	out := make([]SettingDefinition, len(settingRegistry))
	copy(out, settingRegistry)
	return out
}

// LookupSetting finds a definition by name. Names outside the registry are
// rejected, so the store cannot be filled with arbitrary keys.
func LookupSetting(name string) (SettingDefinition, bool) {
	for _, definition := range settingRegistry {
		if definition.Name == name {
			return definition, true
		}
	}
	return SettingDefinition{}, false
}
