package template

// Ceilings are the operator-set half of Limits (TEMPLATE_*). The struct is field-for-field
// identical to config.TemplateCeilings, so the composition root converts one into the other.
type Ceilings struct {
	NameMaxChars        int
	DescriptionMaxChars int
	BodyMaxChars        int
	TitleAreaMaxChars   int
	MaxPerAccount       int
	MaxRepeatExpansion  int
	PhotoRowMax         int
	AskLabelMaxChars    int
	AskMaxPerBody       int
}

// NumberBounds are the POST option range a template's two generation numbers seed (TEMPLATE-47),
// passed in and not owned here: the number is a SEED for that option, and a template able to
// store one the post refuses would make an assignment fail at a place the user never typed
// anything. The length has a floor and no ceiling, exactly as the post's own option does.
type NumberBounds struct {
	TargetLengthMin int
	TagCountMin     int
	TagCountMax     int
}

// NewLimits is the one way a Limits is built: the ceilings and the number bounds, field for field.
// NewService still refuses an invalid result.
func NewLimits(c Ceilings, n NumberBounds) Limits {
	return Limits{
		NameMaxChars: c.NameMaxChars, DescriptionMaxChars: c.DescriptionMaxChars,
		BodyMaxChars: c.BodyMaxChars, TitleAreaMaxChars: c.TitleAreaMaxChars,
		MaxPerAccount: c.MaxPerAccount, MaxRepeatExpansion: c.MaxRepeatExpansion,
		PhotoRowMax: c.PhotoRowMax, AskLabelMaxChars: c.AskLabelMaxChars, AskMaxPerBody: c.AskMaxPerBody,
		TargetLengthMin: n.TargetLengthMin, TagCountMin: n.TagCountMin, TagCountMax: n.TagCountMax,
	}
}
