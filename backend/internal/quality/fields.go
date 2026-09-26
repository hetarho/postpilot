// Package quality owns the one observation the product makes outside a single post: how an
// account's published posts look next to each other (QUAL-1). It also holds the product's 분야
// list (QUAL-23), which the post and guideline contexts validate against.
//
// It is a new flat context without an ARCH edit. ARCH-5's context list is already stale — it
// omits clip and memory, and the wrapper packages devseed, fxrate, googleauth, mail and
// tosspay — so the list does not gate a context that follows ARCH-5's own shape.
package quality

// Field is one 분야 of the product's own list (QUAL-23). ID is the stable ASCII identifier
// SQL stores, and Name is how the product names it.
type Field struct {
	ID, Name string
}

// fields is the v1 list in QUAL-23's order.
var fields = []Field{
	{ID: "restaurant", Name: "맛집"},
	{ID: "cafe", Name: "카페"},
	{ID: "domestic_travel", Name: "국내여행"},
	{ID: "fashion_beauty", Name: "패션·미용"},
	{ID: "product_review", Name: "상품리뷰"},
	{ID: "parenting_marriage", Name: "육아·결혼"},
	{ID: "pets", Name: "반려동물"},
	{ID: "interior_diy", Name: "인테리어·DIY"},
	{ID: "daily_life", Name: "일상·생각"},
}

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
