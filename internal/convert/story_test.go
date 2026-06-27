package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

func TestStoryAreas(t *testing.T) {
	raw := `[{"position":{"x_percentage":0.1,"y_percentage":0.2,"width_percentage":0.3,"height_percentage":0.4,"rotation_angle":5,"corner_radius_percentage":1},` +
		`"type":{"type":"suggested_reaction","reaction_type":{"type":"emoji","emoji":"👍"},"is_dark":true,"is_flipped":false}}]`

	areas, err := StoryAreas(raw)
	if err != nil {
		t.Fatalf("StoryAreas: %v", err)
	}
	if len(areas) != 1 {
		t.Fatalf("got %d areas, want 1", len(areas))
	}
	r, ok := areas[0].(*tg.MediaAreaSuggestedReaction)
	if !ok {
		t.Fatalf("got %T, want MediaAreaSuggestedReaction", areas[0])
	}
	if r.Coordinates.X != 0.1 || r.Coordinates.Rotation != 5 || r.Coordinates.Radius != 1 {
		t.Errorf("coords = %+v", r.Coordinates)
	}
	if !r.Dark {
		t.Errorf("Dark = false, want true")
	}
	re, ok := r.Reaction.(*tg.ReactionEmoji)
	if !ok || re.Emoticon != "👍" {
		t.Errorf("reaction = %+v", r.Reaction)
	}

	// Empty → nil, nil.
	if a, err := StoryAreas(""); err != nil || a != nil {
		t.Errorf("empty: a=%v err=%v", a, err)
	}

	// Link type.
	link := `[{"position":{"x_percentage":0,"y_percentage":0,"width_percentage":1,"height_percentage":1,"rotation_angle":0,"corner_radius_percentage":0},"type":{"type":"link","url":"https://t.me"}}]`
	areas, _ = StoryAreas(link)
	if _, ok := areas[0].(*tg.MediaAreaURL); !ok {
		t.Errorf("got %T, want MediaAreaURL", areas[0])
	}

	// Weather + unique_gift + location round-trip into the right types.
	other := `[` +
		`{"position":{"x_percentage":0,"y_percentage":0,"width_percentage":1,"height_percentage":1,"rotation_angle":0,"corner_radius_percentage":0},"type":{"type":"weather","temperature":-5,"emoji":"❄️","background_color":255}},` +
		`{"position":{"x_percentage":0,"y_percentage":0,"width_percentage":1,"height_percentage":1,"rotation_angle":0,"corner_radius_percentage":0},"type":{"type":"unique_gift","name":"slug1"}},` +
		`{"position":{"x_percentage":0,"y_percentage":0,"width_percentage":1,"height_percentage":1,"rotation_angle":0,"corner_radius_percentage":0},"type":{"type":"location","latitude":12.3,"longitude":45.6}}` +
		`]`
	areas, _ = StoryAreas(other)
	if len(areas) != 3 {
		t.Fatalf("got %d areas, want 3", len(areas))
	}
	if w, ok := areas[0].(*tg.MediaAreaWeather); !ok || w.TemperatureC != -5 || w.Color != 255 {
		t.Errorf("weather = %+v", areas[0])
	}
	if _, ok := areas[1].(*tg.MediaAreaStarGift); !ok {
		t.Errorf("got %T, want MediaAreaStarGift", areas[1])
	}
	if g, ok := areas[2].(*tg.MediaAreaGeoPoint); !ok {
		t.Errorf("got %T, want MediaAreaGeoPoint", areas[2])
	} else if g.Geo == nil {
		t.Errorf("geo is nil")
	}

	// Invalid area type → error.
	bad := `[{"position":{"x_percentage":0,"y_percentage":0,"width_percentage":1,"height_percentage":1,"rotation_angle":0,"corner_radius_percentage":0},"type":{"type":"nope"}}]`
	if _, err := StoryAreas(bad); err == nil {
		t.Errorf("expected error for invalid area type")
	}
	// Malformed JSON → error.
	if _, err := StoryAreas(`[not json`); err == nil {
		t.Errorf("expected error for malformed JSON")
	}
}
