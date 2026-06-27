package convert

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"
)

// storyAreaPosition mirrors the Bot API StoryAreaPosition JSON.
type storyAreaPosition struct {
	XPercentage         float64 `json:"x_percentage"`
	YPercentage         float64 `json:"y_percentage"`
	WidthPercentage     float64 `json:"width_percentage"`
	HeightPercentage    float64 `json:"height_percentage"`
	RotationAngle       float64 `json:"rotation_angle"`
	CornerRadiusPercent float64 `json:"corner_radius_percentage"`
}

type reactionTypeJSON struct {
	Type     string `json:"type"`
	Emoji    string `json:"emoji"`
	CustomID string `json:"custom_emoji_id"`
}

// StoryAreas parses the Bot API "areas" parameter into a list of MTProto MediaArea
// for stories.sendStory/editStory. Mirrors Client.cpp get_input_story_areas +
// get_input_story_area_type. Supported types: suggested_reaction, link, weather,
// unique_gift, location. Returns (nil, nil) when raw is empty.
func StoryAreas(raw string) ([]tg.MediaAreaClass, error) {
	if raw == "" {
		return nil, nil
	}
	var rawAreas []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &rawAreas); err != nil {
		return nil, errors.New("can't parse story areas JSON object")
	}
	out := make([]tg.MediaAreaClass, 0, len(rawAreas))
	for _, a := range rawAreas {
		area, err := parseStoryArea(a)
		if err != nil {
			return nil, err
		}
		out = append(out, area)
	}
	return out, nil
}

func parseStoryArea(a map[string]json.RawMessage) (tg.MediaAreaClass, error) {
	posRaw, ok := a["position"]
	if !ok {
		return nil, errors.New("can't parse InputStoryArea: position is required")
	}
	var pos storyAreaPosition
	if err := json.Unmarshal(posRaw, &pos); err != nil {
		return nil, fmt.Errorf("can't parse InputStoryArea: %v", err)
	}
	coords := &tg.MediaAreaCoordinates{
		X:        pos.XPercentage,
		Y:        pos.YPercentage,
		W:        pos.WidthPercentage,
		H:        pos.HeightPercentage,
		Rotation: pos.RotationAngle,
		Radius:   pos.CornerRadiusPercent,
	}
	typeRaw, ok := a["type"]
	if !ok {
		return nil, errors.New("can't parse InputStoryArea: type is required")
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(typeRaw, &head); err != nil {
		return nil, fmt.Errorf("can't parse InputStoryArea: %v", err)
	}
	switch head.Type {
	case "suggested_reaction":
		var s struct {
			ReactionType reactionTypeJSON `json:"reaction_type"`
			IsDark       bool             `json:"is_dark"`
			IsFlipped    bool             `json:"is_flipped"`
		}
		if err := json.Unmarshal(typeRaw, &s); err != nil {
			return nil, fmt.Errorf("can't parse InputStoryArea: %v", err)
		}
		r, err := parseReactionType(s.ReactionType)
		if err != nil {
			return nil, err
		}
		return &tg.MediaAreaSuggestedReaction{Coordinates: coords, Reaction: r, Dark: s.IsDark, Flipped: s.IsFlipped}, nil
	case "link":
		var s struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(typeRaw, &s); err != nil {
			return nil, fmt.Errorf("can't parse InputStoryArea: %v", err)
		}
		return &tg.MediaAreaURL{Coordinates: coords, URL: s.URL}, nil
	case "weather":
		var s struct {
			Temperature     float64 `json:"temperature"`
			Emoji           string  `json:"emoji"`
			BackgroundColor int64   `json:"background_color"`
		}
		if err := json.Unmarshal(typeRaw, &s); err != nil {
			return nil, fmt.Errorf("can't parse InputStoryArea: %v", err)
		}
		return &tg.MediaAreaWeather{Coordinates: coords, Emoji: s.Emoji, TemperatureC: s.Temperature, Color: int32(s.BackgroundColor)}, nil
	case "unique_gift":
		var s struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(typeRaw, &s); err != nil {
			return nil, fmt.Errorf("can't parse InputStoryArea: %v", err)
		}
		return &tg.MediaAreaStarGift{Coordinates: coords, Slug: s.Name}, nil
	case "location":
		var s struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}
		if err := json.Unmarshal(typeRaw, &s); err != nil {
			return nil, fmt.Errorf("can't parse InputStoryArea: %v", err)
		}
		return &tg.MediaAreaGeoPoint{Coordinates: coords, Geo: &tg.GeoPoint{Lat: s.Latitude, Long: s.Longitude}}, nil
	default:
		return nil, errors.New("invalid story area type specified")
	}
}

func parseReactionType(r reactionTypeJSON) (tg.ReactionClass, error) {
	switch r.Type {
	case "emoji":
		return &tg.ReactionEmoji{Emoticon: r.Emoji}, nil
	case "custom_emoji":
		id, err := strconv.ParseInt(r.CustomID, 10, 64)
		if err != nil {
			return nil, errors.New("invalid custom_emoji_id")
		}
		return &tg.ReactionCustomEmoji{DocumentID: id}, nil
	case "paid":
		return &tg.ReactionPaid{}, nil
	default:
		return nil, errors.New("invalid ReactionType type")
	}
}
