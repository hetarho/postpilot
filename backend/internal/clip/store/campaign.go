package store

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
)

func encodeGuideItems(items []string) string {
	raw, _ := json.Marshal(append([]string{}, items...))
	return string(raw)
}
func decodeAdvertiserGuide(text, keywords, hashtags string) (clip.AdvertiserGuide, error) {
	guide := clip.AdvertiserGuide{Text: text}
	if err := json.Unmarshal([]byte(keywords), &guide.Keywords); err != nil {
		return guide, err
	}
	if err := json.Unmarshal([]byte(hashtags), &guide.Hashtags); err != nil {
		return guide, err
	}
	return clip.NormalizeAdvertiserGuide(guide)
}
