package voice

const analysisPrompt = `당신은 한국어 문체 분석가입니다. 제공된 모든 글 샘플을 함께 분석해 한국어 문체 가이드를 작성하세요.

아래 아홉 섹션을 반드시 이 순서와 제목으로 작성하세요. 첫 줄부터 1번 섹션으로 시작하고 서문은 쓰지 마세요.
1. 종결어미 분포: ~다 / ~해요 / ~습니다 및 혼용 비율과 조건
2. 평균 문장 길이
3. 문단당 문장 수
4. 자주 쓰는 접속어와 부사
5. 말버릇과 시그니처 표현
6. 이모지·감탄사·말줄임표 사용 여부와 빈도
7. 비유와 숫자 사용 방식
8. 절대 사용하지 않는 표현 (never uses): 관찰되지 않은 상투적 AI 표현을 명시
9. 1인칭 표현: 나 / 저 / 우리 / 없음

근거가 약한 항목은 지어내지 말고 "관찰되지 않음"이라고 쓰세요. 출력은 한국어 일반 텍스트로 작성하세요.`

const englishAnalysisPrompt = `You are an English writing-style analyst. Analyze all supplied samples together and produce an English style guide.

Write exactly these nine sections in this order, starting with section 1 and no preface.
1. Register and formality: formal/conversational balance and contraction use
2. Average sentence length in words and characters
3. Statement/question/exclamation/fragment cadence
4. Connectives and adverbs
5. Passive voice and nominalization tendencies
6. Opening, closing, paragraph, heading, list, and emoji habits
7. Lexical habits and signature expressions
8. Expressions the author never uses
9. Six axes: involvement, narrativity, persuasion overtness, abstractness, addressee focus, humor

Do not invent unsupported traits; write "not observed" when evidence is weak. Never translate English behavior into Korean ending categories. Return English plain text.`

func analysisPromptForLanguage(language Language) string {
	if language == LanguageEnglish {
		return englishAnalysisPrompt
	}
	return analysisPrompt
}

// The seeding prompt takes a DESCRIPTION of a wanted register, not a writing sample. It must
// produce the same nine-section shape the analysis produces, because everything downstream —
// hasRequiredAnalysisShapeForLanguage, the prompt projection, the profile editors — reads one
// styleguide format. The "do not quote the description" rule matters: the input is a request
// written by the user about their writing, and a model left to itself will echo its wording as
// if it were observed prose.
const seedPrompt = `당신은 한국어 문체 설계자입니다. 사용자가 원하는 말투를 한 문장 또는 몇 문장으로 설명하면, 그 설명대로 글을 쓸 때 따라야 할 한국어 문체 가이드를 작성하세요.

입력은 글 샘플이 아니라 "이런 말투로 쓰고 싶다"는 요청입니다. 입력 문장을 인용하거나 그 문장 자체의 길이·어미·표현을 측정하지 마세요. 요청이 가리키는 말투를 구체적인 규칙으로 풀어 쓰는 것이 목표입니다.

아래 아홉 섹션을 반드시 이 순서와 제목으로 작성하세요. 첫 줄부터 1번 섹션으로 시작하고 서문은 쓰지 마세요.
1. 종결어미 분포: ~다 / ~해요 / ~습니다 및 혼용 비율과 조건
2. 평균 문장 길이
3. 문단당 문장 수
4. 자주 쓰는 접속어와 부사
5. 말버릇과 시그니처 표현
6. 이모지·감탄사·말줄임표 사용 여부와 빈도
7. 비유와 숫자 사용 방식
8. 절대 사용하지 않는 표현 (never uses): 이 말투와 어울리지 않는 상투적 AI 표현을 명시
9. 1인칭 표현: 나 / 저 / 우리 / 없음

요청이 다루지 않은 항목은 요청과 어긋나지 않는 범위에서 가장 자연스러운 기본값을 고르고, 근거가 전혀 없으면 "관찰되지 않음"이라고 쓰세요. 출력은 한국어 일반 텍스트로 작성하세요.`

const englishSeedPrompt = `You are an English writing-style designer. The user describes the register they want in a sentence or two; produce the English style guide that writing in that register must follow.

The input is a request, not a sample. Do not quote it, and do not measure the length, endings, or phrasing of the request itself. Your task is to expand what it asks for into concrete rules.

Write exactly these nine sections in this order, starting with section 1 and no preface.
1. Register and formality: formal/conversational balance and contraction use
2. Average sentence length in words and characters
3. Statement/question/exclamation/fragment cadence
4. Connectives and adverbs
5. Passive voice and nominalization tendencies
6. Opening, closing, paragraph, heading, list, and emoji habits
7. Lexical habits and signature expressions
8. Expressions the author never uses
9. Six axes: involvement, narrativity, persuasion overtness, abstractness, addressee focus, humor

Where the request is silent, choose the most natural default that does not contradict it; write "not observed" only when nothing supports a choice. Never translate English behavior into Korean ending categories. Return English plain text.`

func seedPromptForLanguage(language Language) string {
	if language == LanguageEnglish {
		return englishSeedPrompt
	}
	return seedPrompt
}
