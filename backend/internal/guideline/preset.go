package guideline

// presetPhraseHeading is a literal copy of the phrase section's heading in the write prompt
// (generation.FieldPhrasesHeading). This context may not import generation (ARCH-7), so
// cmd/api pins the copy with a test: renaming either side changes both.
const presetPhraseHeading = "[분야 상위 글 문구]"

// PresetText is the 상위 노출 단어 사용 preset's line (GUIDE-30): a product constant, never a
// row, config or env value (GUIDE-32), and one Korean line for every target language like the
// section heading it names (GUIDE-15). It binds only the listed phrases, and only where the
// source already says the same thing (GUIDE-31, GUIDE-36); it says nothing about what the
// phrases would improve (QUAL-21).
const PresetText = presetPhraseHeading + "에 나열된 문구는 원문이 이미 같은 내용을 다른 말로 쓴 자리에서만 그 말 대신 쓰고, 원문에 없는 내용을 그 문구 때문에 더하지 않는다."
