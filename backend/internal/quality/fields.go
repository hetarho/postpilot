// Package quality owns the two observations the product makes outside a single post: how an
// account's published posts look next to each other, and which phrases rank for a 분야 on
// Naver (QUAL-1).
//
// It is a new flat context without an ARCH edit. ARCH-5's context list is already stale — it
// omits clip and memory, and the wrapper packages devseed, fxrate, googleauth, mail and
// tosspay — so the list does not gate a context that follows ARCH-5's own shape.
package quality

import "strings"

// Field is one 분야 of the product's own list (QUAL-23). ID is the stable ASCII identifier
// SQL stores, Name is how the product names it, and Query is what the daily phrase batch
// sends to the search API for it.
type Field struct {
	ID, Name, Query string
}

// fields is the v1 list in QUAL-23's order. Query is derived from Name once, here, so the
// two cannot drift apart.
var fields = func() []Field {
	named := []struct{ id, name string }{
		{"restaurant", "맛집"},
		{"cafe", "카페"},
		{"domestic_travel", "국내여행"},
		{"fashion_beauty", "패션·미용"},
		{"product_review", "상품리뷰"},
		{"parenting_marriage", "육아·결혼"},
		{"pets", "반려동물"},
		{"interior_diy", "인테리어·DIY"},
		{"daily_life", "일상·생각"},
	}
	out := make([]Field, 0, len(named))
	for _, field := range named {
		out = append(out, Field{ID: field.id, Name: field.name, Query: strings.ReplaceAll(field.name, "·", " ")})
	}
	return out
}()

// Fields returns the v1 list in order. The slice is a copy, so a caller cannot reorder the
// catalogue for everyone else.
func Fields() []Field {
	return append([]Field(nil), fields...)
}

// FieldByID finds a 분야 by its ASCII id.
func FieldByID(id string) (Field, bool) {
	for _, field := range fields {
		if field.ID == id {
			return field, true
		}
	}
	return Field{}, false
}

// Known reports whether id names a 분야 on the list. It is what the post and guideline
// contexts' own FieldDirectory ports are wired to, so neither imports this package.
func Known(id string) bool {
	_, ok := FieldByID(id)
	return ok
}
