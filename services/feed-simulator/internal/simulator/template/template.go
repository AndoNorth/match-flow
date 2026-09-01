// Package template loads match templates from JSON files - each one
// either a literal, fully scripted event sequence, or a set of bounded
// random constraints (exact or ranged goal/card counts, randomly
// timed). football.NewFromTemplate turns a Template into a schedule
// the engine walks minute-by-minute; this package only loads and
// validates the config shape, it knows nothing about how a schedule
// gets built or played.
package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Kind selects how a Template's fields are interpreted.
type Kind string

const (
	KindLiteral Kind = "literal"
	KindBounded Kind = "bounded"
)

// Range is an inclusive [Min, Max] count - Min == Max pins an exact
// count (e.g. "always exactly 1 yellow card").
type Range struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// ScriptedEvent is one entry in a literal Template's Events list.
// Team and CardType are only meaningful for their respective event
// Types ("goal"/"card" and "card" respectively) - ignored otherwise.
type ScriptedEvent struct {
	Type     string `json:"type"`
	Team     string `json:"team,omitempty"`
	CardType string `json:"card_type,omitempty"`
	Minute   int    `json:"minute"`
}

// Template is one named match profile, loaded from
// services/feed-simulator/templates/<name>.json.
type Template struct {
	Name        string          `json:"name"`
	Kind        Kind            `json:"kind"`
	Events      []ScriptedEvent `json:"events,omitempty"`
	HomeGoals   Range           `json:"home_goals,omitempty"`
	AwayGoals   Range           `json:"away_goals,omitempty"`
	YellowCards Range           `json:"yellow_cards,omitempty"`
	RedCards    Range           `json:"red_cards,omitempty"`
}

// Load reads and validates one template file.
func Load(path string) (Template, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a caller-controlled template file, not user input
	if err != nil {
		return Template{}, fmt.Errorf("read template %s: %w", path, err)
	}

	var t Template
	if err := json.Unmarshal(data, &t); err != nil {
		return Template{}, fmt.Errorf("parse template %s: %w", path, err)
	}

	if err := t.validate(); err != nil {
		return Template{}, fmt.Errorf("invalid template %s: %w", path, err)
	}
	return t, nil
}

// LoadAll reads every *.json file in dir, keyed by each template's own
// Name field (not its filename - the two may differ).
func LoadAll(dir string) (map[string]Template, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("glob templates dir %s: %w", dir, err)
	}

	templates := make(map[string]Template, len(matches))
	for _, path := range matches {
		t, err := Load(path)
		if err != nil {
			return nil, err
		}
		templates[t.Name] = t
	}
	return templates, nil
}

func (t Template) validate() error {
	if t.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch t.Kind {
	case KindLiteral:
		if len(t.Events) == 0 {
			return fmt.Errorf("literal template %q has no events", t.Name)
		}
	case KindBounded:
		for _, r := range []struct {
			field string
			r     Range
		}{
			{"home_goals", t.HomeGoals}, {"away_goals", t.AwayGoals},
			{"yellow_cards", t.YellowCards}, {"red_cards", t.RedCards},
		} {
			if r.r.Min < 0 || r.r.Max < r.r.Min {
				return fmt.Errorf("bounded template %q has an invalid %s range", t.Name, r.field)
			}
		}
	default:
		return fmt.Errorf("template %q has unknown kind %q", t.Name, t.Kind)
	}
	return nil
}
