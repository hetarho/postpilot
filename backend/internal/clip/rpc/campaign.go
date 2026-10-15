package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func guideInput(value *v1.AdvertiserGuide) *clip.AdvertiserGuide {
	if value == nil {
		return nil
	}
	guide := clip.AdvertiserGuide{Text: value.Text, Keywords: value.Keywords, Hashtags: value.Hashtags}
	if value.MinChars != nil {
		n := int(*value.MinChars)
		guide.MinChars = &n
	}
	if value.MinPhotos != nil {
		n := int(*value.MinPhotos)
		guide.MinPhotos = &n
	}
	return &guide
}
func guideProto(value clip.AdvertiserGuide) *v1.AdvertiserGuide {
	return &v1.AdvertiserGuide{Text: value.Text, Keywords: value.Keywords, Hashtags: value.Hashtags}
}
