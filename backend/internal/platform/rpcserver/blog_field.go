package rpcserver

import postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"

// BlogFieldFromProto translates the shared 분야 enum to the ASCII id every context stores.
// UNSPECIFIED is 없음, a real answer, so it maps to "" with true; a number this build does not
// know maps to false and never to a 분야. The ids are the quality context's catalogue, which
// this package may not import; cmd/api pins the two lists to each other.
func BlogFieldFromProto(value postpilotv1.BlogField) (string, bool) {
	switch value {
	case postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED:
		return "", true
	case postpilotv1.BlogField_BLOG_FIELD_RESTAURANT:
		return "restaurant", true
	case postpilotv1.BlogField_BLOG_FIELD_CAFE:
		return "cafe", true
	case postpilotv1.BlogField_BLOG_FIELD_DOMESTIC_TRAVEL:
		return "domestic_travel", true
	case postpilotv1.BlogField_BLOG_FIELD_FASHION_BEAUTY:
		return "fashion_beauty", true
	case postpilotv1.BlogField_BLOG_FIELD_PRODUCT_REVIEW:
		return "product_review", true
	case postpilotv1.BlogField_BLOG_FIELD_PARENTING_MARRIAGE:
		return "parenting_marriage", true
	case postpilotv1.BlogField_BLOG_FIELD_PETS:
		return "pets", true
	case postpilotv1.BlogField_BLOG_FIELD_INTERIOR_DIY:
		return "interior_diy", true
	case postpilotv1.BlogField_BLOG_FIELD_DAILY_LIFE:
		return "daily_life", true
	default:
		return "", false
	}
}

// BlogFieldToProto translates a stored ASCII id back to the wire. "" is 없음; an id that is
// not on the list answers false rather than becoming a valid 분야 on the wire.
func BlogFieldToProto(id string) (postpilotv1.BlogField, bool) {
	switch id {
	case "":
		return postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED, true
	case "restaurant":
		return postpilotv1.BlogField_BLOG_FIELD_RESTAURANT, true
	case "cafe":
		return postpilotv1.BlogField_BLOG_FIELD_CAFE, true
	case "domestic_travel":
		return postpilotv1.BlogField_BLOG_FIELD_DOMESTIC_TRAVEL, true
	case "fashion_beauty":
		return postpilotv1.BlogField_BLOG_FIELD_FASHION_BEAUTY, true
	case "product_review":
		return postpilotv1.BlogField_BLOG_FIELD_PRODUCT_REVIEW, true
	case "parenting_marriage":
		return postpilotv1.BlogField_BLOG_FIELD_PARENTING_MARRIAGE, true
	case "pets":
		return postpilotv1.BlogField_BLOG_FIELD_PETS, true
	case "interior_diy":
		return postpilotv1.BlogField_BLOG_FIELD_INTERIOR_DIY, true
	case "daily_life":
		return postpilotv1.BlogField_BLOG_FIELD_DAILY_LIFE, true
	default:
		return postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED, false
	}
}
