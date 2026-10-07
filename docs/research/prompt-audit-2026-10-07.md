# 프롬프트 구조·길이·모순·언어 심층 검토

확인일: 2026-10-07, 코드 기준 `main@95da3ad4`. [현재 프롬프트 지도](../design/prompt-map-2026-10-07.md)의 후속 조사다. 제품 코드·SSOT·배포는 변경하지 않았다.

## 먼저 판단할 내용

**현재 프롬프트는 섹션·입력 타입·조립 함수·응답 스키마·golden이 이미 있다. 더 큰 문제는 단계별 계약, 자료와 지시의 구분, 우선순위, 언어 설정이 조립 경로마다 다르다는 점이다.** 문장을 줄이거나 영어로 옮기는 작업만으로 이 문제를 해결할 수 없다.

언어에 대한 잠정 권고는 **한국어 지시를 비교 기준으로 유지하고, 한국어 사실·말투·예시·출력을 그대로 둔 영어 공통 지시 또는 혼합 지시를 별도로 시험하는 것**이다. 공개 연구는 모델·과업·입력 번역 여부에 따라 결과가 달랐다. 한국어 블로그 작성에서 영어 지시가 항상 우월하다는 근거는 확보하지 못했다.

태그 선택 규칙은 사용자가 정리한 대로 기본 지침에 속한다. 현재 코드도 이미 `tags` 기본 지침이며 이동할 필요가 없다. 사용자가 선택한 **근거 있는 구체적 태그만 최대 N개**는 현재 고정 요청의 **정확히 N개**와 맞출 별도 변경 재료다. 이번 조사에서는 구현하지 않았다.

## 조사 방법과 증거 수준

- 실제 builder·모델 호출·파서·최종 저장 소비자를 추적했다. 현재 문구가 존재하는 것, 잘못된 출력이 처리되는 방식, 실제 모델의 응답 품질을 구분했다.
- 저장소 파일을 바꾸지 않는 임시 Go overlay로 템플릿 문자열 충돌·자료 경계·VIDEO 수정 계약·블록 삭제·순서 지시를 확인했다.
- 영상의 공개 순수 builder에 합성 입력을 넣어 메시지를 생성하고, 언어만 바꾼 비교와 크기를 측정했다. provider 호출은0회다.
- 실제 코드 문자열과 golden을 `tiktoken 0.14.0`의 `cl100k_base`·`o200k_base`로 오프라인 측정했다. 설치는 `/tmp`의 독립 가상환경에만 했다.
- 언어 연구는 원논문의 방법·표·부록을 읽었다. 공급자 안내는 공식 문서를 확인했다. 모델·자료·지시·예시·추론·출력 언어가 각각 무엇이었는지 구분했다.
- T591–T604와 T622–T631의 기존 계획을 함께 확인했다. 미통합 작업을 이미 구현된 기능으로 다루거나, 같은 일을 새 태스크로 중복 생성하지 않았다.

여기서 **결정적 재현**은 같은 입력을 넣었을 때 확인되는 코드 상태다. **문구·계약 불일치**는 서로 다른 요구가 실제로 함께 전달된다는 증거다. **품질 가설**은 모델 출력 비교가 필요한 예상이다. **현재 정책 검토**는 구현이 잘못됐다는 판정이 아니라 선택된 정책의 효과를 다시 볼 제안이다.

측정값과 합성 재현 원문은 [측정 JSON](prompt-audit-measurements-2026-10-07.json), [재현 자료 JSON](prompt-audit-reproductions-2026-10-07.json)에 보존했다. 실제 사용자 작업·운영자 모델 설정·비밀값은 읽지 않았다.

## 글 작성·수정·템플릿에서 확인한 항목

| 항목 | 문제와 영향을 받는 상황 | 확인 수준·위치 | 검토 방향 |
|---|---|---|---|
| 고정 문구와 작성 지시의 구별 소실 | `&lt;write&gt;메뉴&lt;/write&gt;`라는 고정 문구와 `<write>메뉴</write>` 작성 지시가 둘 다 동일한 `<write>메뉴</write>`로 Render된다. 모델이 두 의미를 구별할 원자료가 사라짐 | 결정적 재현. [expand.go:133](/home/hrlee/project/postpilot/backend/internal/template/expand.go:133), [문법 안내:136](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:136) | 문구와 문법의 출처 구별을 유지하는 계약을 검토. 실제 모델의 오출력은 미측정 |
| 사실 입력이 구획을 닫을 수 있음 | 답변에 `</facts>`가 들어가면 선언한 사실 구획이 문자열상 닫힌다. 이후 `<write>` 문구와 사실 값이 섞일 수 있음 | 결정적 재현. [expand.go:181](/home/hrlee/project/postpilot/backend/internal/template/expand.go:181), [사실 안내:149](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:149) | 사실 값과 템플릿 지시의 경계를 유지할 방법 검토. LLM이 XML parser처럼 해석한다는 가정은 하지 않음 |
| 수정의 VIDEO 계약 누락 | 현재 글·응답 스키마에는 VIDEO가 있지만 수정 System의 허용 type은6종뿐이다. 영상 있는 글에서 요청 밖 블록 보존과 type 목록이 어긋남 | 계약 불일치 재현. [revise.go:45](/home/hrlee/project/postpilot/backend/internal/generation/revise.go:45), [schema:16](/home/hrlee/project/postpilot/backend/internal/generation/schemas/post_content.schema.json:16) | 작성·수정의 실제 허용 블록 계약을 맞추는 후보. 실제 영상 유실은 미측정 |
| 블록별 필드 설명 부족 | LIST 내용을 `content`에 쓴 응답은 items가 없어 해당 블록이 삭제된다. 공통 문구는 type·필드를 나열하지만 각각의 역할 설명은 약함 | 후처리 재현. [prompts.go:78](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:78), [blocks.go:92](/home/hrlee/project/postpilot/backend/internal/generation/blocks.go:92) | 잘못된 응답을 그대로 허용하기보다 field/type 계약을 더 분명히 설명하는 후보 |
| 스토리·템플릿·지침 순서 우선권 | 확정 스토리의 순서, 템플릿을 그대로 따르기, 지침이 템플릿보다 우선이라는 세 요구가 함께 들어감. GEN-70의 확정 스토리 우선 관계는 더 명시할 여지가 있음 | 문구 동시 주입 재현. [스토리 규칙:94](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:94), [템플릿:221](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:221), [지침:236](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:236), [GEN-70](/home/hrlee/project/postpilot/spec/ssot/GEN.md:117) | 실제 순서가 충돌하는 입력으로 우선순위 표현 비교. 원문만으로 모델의 필연적 실패를 단정하지 않음 |
| 태그 개수와 채우기 금지 | 고정 요청은 정확히 N개, 기본 지침은 범용 태그로 개수를 채우지 말라고 함. 파서는 적은 개수도 받음 | 코드·문구 확인. [요청:408](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:408), [기본 지침:150](/home/hrlee/project/postpilot/backend/internal/guideline/defaults.go:150), [파서:209](/home/hrlee/project/postpilot/backend/internal/generation/parse.go:209) | 채택된 최대 N개 방향을 계약에 반영할 재료. 태그 선택 기준의 기본 지침 소유권은 유지 |
| 태그 없는 단계에 태그 규칙 | 스토리라인 생성은 storyline만 반환하지만 글 기본 지침 전체를 받으므로 tags 선택 지침도 들어갈 수 있음 | 코드 확인. [storyline_prompts.go:90](/home/hrlee/project/postpilot/backend/internal/generation/storyline_prompts.go:90) | 지침별 적용 단계를 검토. 관련된 내용 지침까지 모두 빼자는 결론은 아님 |
| 수정의 근거 자료 차이 | 태그 지침은 재료로 확인하라고 하지만 수정 User에는 현재 글·첨부 이름/방향·요청만 있음. 원래 메모·관찰·기억은 다시 전달하지 않음 | 코드 확인. [revise.go:156](/home/hrlee/project/postpilot/backend/internal/generation/revise.go:156) | 수정할 범위에 필요한 근거가 무엇인지 정할 후보. 최초 작성 자료 전체를 무조건 재주입하지 않음 |
| 요청 밖 태그 유지의 한계 | 유지하라는 지시는 있으나 코드가 기존 태그를 다시 대입하거나 변경을 검사하지 않음. 기존 태그가 N보다 많으면 공통 파서가 자를 수 있음 | 코드 확인. [revise_handler.go:49](/home/hrlee/project/postpilot/backend/internal/generation/revise_handler.go:49), [parse.go:209](/home/hrlee/project/postpilot/backend/internal/generation/parse.go:209) | 요청 범위의 보존 책임 검토. 실제 사용자 실패 사례는 확인하지 않음 |
| 출력 언어와 지시 언어의 결합 | 현재 `LanguageEnglish`는 지시 언어뿐 아니라 최종 출력·기본 지침 사본·한국어 전용 규칙·말투 투영을 바꿈 | 코드 확인. [prompts.go:405](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:405), [Default.Text:46](/home/hrlee/project/postpilot/backend/internal/guideline/defaults.go:46), [말투 투영:39](/home/hrlee/project/postpilot/backend/internal/voice/projection.go:39) | 영어 지시+한국어 출력 비교에서 target을 바꾸지 않기. 지시 언어만 다른 조건을 정의할 재료 |
| 사용자 지침의 언어 충돌 | 출력 언어 우선 대상은 말투·템플릿·메모·가제라고 명시하지만 사용자 지침이 목록에서 빠짐. ‘영어로만 작성’ 지침과 Korean target의 관계가 덜 명시적 | 표현 검토 가설. [prompts.go:417](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:417), [GUIDE-15](/home/hrlee/project/postpilot/spec/ssot/GUIDE.md:44) | 기존 정책의 언어 우선 관계를 더 명확히 표현할지 비교. Korean 출력 문장 자체로 충분한 모델도 있을 것 |
| 한국어 자연스러움 규칙의 밀도 | 640자의 규칙에 대조·문단 끝·전망·쉼표·문장 길이·동사·비유·감상·말투 우선 등을 함께 넣음 | 실제 크기·문구 확인, 품질은 가설. [defaults.go:95](/home/hrlee/project/postpilot/backend/internal/guideline/defaults.go:95) | 결과를 악화하는 제약이 무엇인지 개별 제거/긍정 예시 비교. 현재 말투 우선·켜기/끄기 정책 유지 |
| 구성 규칙의 수치와 적합성 | 최소3종 블록과 내용에 맞지 않는 블록 금지가 함께 있음. 아주 짧은 소재에서 불필요한 형식을 유도할 가능성 | 현재 정책 검토. [quality/rules.go:23](/home/hrlee/project/postpilot/backend/internal/quality/rules.go:23) | 소재가 적을 때의 품질 평가. 이는 현재 측정 밴드에 연결된 의도적 규칙이며 자동 삭제할 코드 버그가 아님 |
| 전면 수정 후 품질 측정 자료 | 수정 뒤 nouns는 이전 목록을 유지하며 반복도 측정은 그 목록을 사용함. 새 핵심 명사가 측정 대상에서 빠질 수 있음 | 현재 정책 검토. [revise_handler.go:89](/home/hrlee/project/postpilot/backend/internal/generation/revise_handler.go:89), [metrics.go:256](/home/hrlee/project/postpilot/backend/internal/quality/metrics.go:256) | 측정의 신선도 표시·갱신 정책 후보. 추가 모델 호출을 피하는 장점을 함께 비교 |
| 영어 스토리라인 후 작성 표현 | 제공된 스토리를 따르는 경로의 일부 영상 설명은 direct-path와 같은 `its storyline paragraph` 표현을 유지 | 낮은 우선순위 표현 후보. [prompts.go:438](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:438) | 새 스토리 출력과 제공된 스토리 참조의 표현을 분명히 할 후보. 전용 문구가 이미 있어 직접 계약 오류보다 우선도가 낮음 |

이번 정리의 목적은 중복·불필요한 context와 잘못된 구획을 줄이고 프롬프트 생성을 유지보수할 수 있게 하는 것이다. 필요한 근거·규칙과 적용 조건은 추적 가능하게 유지한다. 태그·사실성 같은 선택 가능한 작성 방향을 고정 System의 무조건 규칙으로 옮기면 기존 선택권을 바꾼다. System 전송 역할 안에 기본 지침이 들어가는 것과, 그 지침을 끌 수 있다는 정책은 함께 성립한다.

## 설정 만들기·공통 조립에서 확인한 항목

| 항목 | 현재 상태와 영향 | 근거·검토 방향 |
|---|---|---|
| 모드·종류와 무관한 공통 문구 | 모든 authoring 호출이 추천8개·템플릿8개 장르·가상 말투·수정 규칙을 함께 받음. 수정이나 영상 지침 생성에 직접 필요하지 않은 내용도 있음 | [authoring/run.go:69](/home/hrlee/project/postpilot/backend/internal/authoring/run.go:69). mode/kind별로 필요한 규칙을 드러내는 후보. schema·mode 정보가 이미 구별을 돕기 때문에 실모델 실패로 판정하지 않음 |
| 읽기 어려운 guide 원문 | 지침·말투 guide 문자열은 한국어 띄어쓰기·섹션이 적고 긴 문장에 제한과 역할이 합쳐짐 | [guideline/authoring.go:83](/home/hrlee/project/postpilot/backend/internal/guideline/authoring.go:83), [voice/authoring.go:83](/home/hrlee/project/postpilot/backend/internal/voice/authoring.go:83). 가독성과 조건의 분리를 검토. 공백을 늘리거나 줄이면 토큰·품질이 반드시 개선된다는 주장은 하지 않음 |
| Code 소유와 메시지 역할의 혼합 | authoring guide는 코드가 만들지만 User JSON의 guide에 들어감. 개인 말투·템플릿·기본/사용자 지침은 글 작성 System에 들어감 | [modelMessage:140](/home/hrlee/project/postpilot/backend/internal/authoring/run.go:140), [write builder:445](/home/hrlee/project/postpilot/backend/internal/generation/prompts.go:445). System=모든 제품 정책, User=모든 사실자료라는 단순 분류로 설명할 수 없음. 출처·역할·권한을 따로 표시할 후보 |
| 같은 설정의 별도 생성 경로 | 공통 authoring 외에 기존 템플릿 요청과 가상 말투 후보가 별도 활성 handler로 남음. 입력·예시·언어·보정 방식도 다름 | [jobs.go:112](/home/hrlee/project/postpilot/backend/cmd/api/jobs.go:112), [request.go:256](/home/hrlee/project/postpilot/backend/internal/template/request.go:256), [candidates.go:176](/home/hrlee/project/postpilot/backend/internal/voice/candidates.go:176). 기존 T625/T626/T630과 함께 일관성을 점검할 후보 |
| 문자 수와 토큰 단위 | `PromptTokens` 값은 `RuneCount`로 산출하고 context의 token capacity와 비교함. schema와 메시지 경계도 이 값에 포함되지 않음 | [run.go:88](/home/hrlee/project/postpilot/backend/internal/authoring/run.go:88), [run.go:120](/home/hrlee/project/postpilot/backend/internal/authoring/run.go:120). 언어에 따른 tokenizer 차이 때문에 정확한 token 측정으로 볼 수 없음. 비용 예약에는 별도 입력 floor가 있으므로 실제 과소청구까지 단정하지 않음 |
| 과거 요청 재현과 현재 조립의 차이 | job payload는 일부 자료 확정이며 완성 wire 요청이 아님. 일반 글의 실행 시 말투 조회, 비교 snapshot, provider reasoning override도 다름 | [write.go:14](/home/hrlee/project/postpilot/backend/internal/generation/write.go:14), [snapshot:40](/home/hrlee/project/postpilot/backend/internal/generation/experiment_snapshot.go:40), [registry.go:442](/home/hrlee/project/postpilot/backend/internal/llm/registry.go:442). 코드·fixture·준비 요청·실행 기록을 구분해 보여줄 후보 |

## 영상·음성·기억에서 확인한 항목

| 항목 | 현재 상태와 영향 | 확인 수준·근거 |
|---|---|---|
| 영상 작성의 언어 값 누락 | 같은 입력에서 Language만 ko→en으로 바꾸어도 Flow·Storyline·Narrate·SpokenScript의 System/User가 동일. 일부는 관찰 언어를 따르거나 ‘project language’를 말하지만 값은 전달되지 않음 | 순수 builder 재현. [flow_prompt.go:97](/home/hrlee/project/postpilot/backend/internal/clip/ai/flow_prompt.go:97), [storyline_prompt.go:44](/home/hrlee/project/postpilot/backend/internal/clip/ai/storyline_prompt.go:44), [narration_prompt.go:56](/home/hrlee/project/postpilot/backend/internal/clip/ai/narration_prompt.go:56), [spoken_script.go:15](/home/hrlee/project/postpilot/backend/internal/clip/ai/spoken_script.go:15). 실제 오언어 응답률은 미측정 |
| 버릴 스토리 생성·검증 | 확정 FollowStoryline이 있어도 음성 응답에 새 storyline·region_slots를 요구하고 검증한 뒤 원래 스토리로 덮음. 대본이 좋아도 불필요한 스토리 오류로 실패 가능 | 코드 확인. [spoken_script.go:37](/home/hrlee/project/postpilot/backend/internal/clip/ai/spoken_script.go:37), [storyline_parse.go:27](/home/hrlee/project/postpilot/backend/internal/clip/ai/storyline_parse.go:27). 확정 스토리 경로의 결과 책임을 줄일 후보 |
| 사실이 instruction으로 포장됨 | 템플릿의 value 치환 결과가 declared caption의 instruction 문자열로 합쳐짐. 사실 값의 명령형 문장과 템플릿 지시의 출처가 평탄화됨 | 합성 builder 재현. [composition/resolve.go:152](/home/hrlee/project/postpilot/backend/internal/clip/composition/resolve.go:152), [narration_prompt.go:106](/home/hrlee/project/postpilot/backend/internal/clip/ai/narration_prompt.go:106). 모델의 실제 의미적 오반응은 미측정 |
| 음성 수정의 지침 차이 | 음성 생성·일반 영상 작성은 영상 지침을 붙이지만 spoken revision의 대본 분기는 붙이지 않음 | 코드 확인. [spoken_revision.go:50](/home/hrlee/project/postpilot/backend/internal/clip/ai/spoken_revision.go:50), [video_guidelines_prompt.go:11](/home/hrlee/project/postpilot/backend/internal/clip/ai/video_guidelines_prompt.go:11). 적용 범위의 일관성을 검토할 후보 |
| 음성 수정의 schema 차이 | 생성은 제한 일부를 제거한 구조 schema, 수정은 min/max 제한이 남은 원 schema를 전송 | 코드 확인. [spoken_script.go:74](/home/hrlee/project/postpilot/backend/internal/clip/ai/spoken_script.go:74), [schemas.go:63](/home/hrlee/project/postpilot/backend/internal/clip/ai/schemas.go:63), [service.go:264](/home/hrlee/project/postpilot/backend/internal/clip/ai/service.go:264). 실제 provider 거절은 미재현 |
| 발화 가능 시간의 정보 부족 | intro/outro 시간을 비우라고 지시하지만 builder에는 그 실제 길이와 본문 시간 값이 없음. 합성 뒤 전체 길이 초과를 거절할 수 있음 | 코드 확인. [spoken_script.go:15](/home/hrlee/project/postpilot/backend/internal/clip/ai/spoken_script.go:15), [spoken_generation.go:77](/home/hrlee/project/postpilot/backend/internal/clip/app/spoken_generation.go:77). 사전 조건과 사후 측정값을 구분할 후보. TTS 길이의 정확한 텍스트 예측을 요구하지 않음 |
| 서버가 쓰지 않는 필수 응답 값 | caption id는 서버가 다시 만들고 Flow duration_ms는 컷으로 재계산. 모델은 해당 필드를 필수 생성해야 함 | 코드·기존 타임라인 테스트 확인. [narration_parse.go:267](/home/hrlee/project/postpilot/backend/internal/clip/ai/narration_parse.go:267), [flow_parse.go:28](/home/hrlee/project/postpilot/backend/internal/clip/ai/flow_parse.go:28). 생성 계약과 기존 응답 수용 계약을 나눠 검토할 후보 |
| schema 문법의 중복 | Observe·SpokenScript의 일부 structured 호출은 System의 전체 JSON 계약과 별도 구조 schema를 함께 보냄. 다른 Flow/Storyline/Narrate는 이미 더 짧은 안내를 선택함 | 실제 builder 크기 측정. [Observe:38](/home/hrlee/project/postpilot/backend/internal/clip/ai/prompts.go:38), [spoken:26](/home/hrlee/project/postpilot/backend/internal/clip/ai/spoken_script.go:26). 중복 문법을 줄일 후보이며, 숫자·coverage·시간 등 domain 조건은 남겨야 함 |
| 기억 추출의 언어·자료 구획 | 한국어 고정 System에 제목·메모·본문을 heading으로 붙이며 목표 언어와 인용/명령문이 자료라는 설명이 없음 | 코드 확인. [extraction.go:67](/home/hrlee/project/postpilot/backend/internal/memory/extraction.go:67), [extraction.go:232](/home/hrlee/project/postpilot/backend/internal/memory/extraction.go:232). 실제 인용 오인·번역률은 미측정 |
| 설명 문서와 실행 코드 | clip README는 제거된 Plan, 텍스트 없는 Flow, 보정 없음으로 설명하지만 실제 구현과 다름 | [README](/home/hrlee/project/postpilot/backend/internal/clip/ai/README.md), [보정 구현:36](/home/hrlee/project/postpilot/backend/internal/clip/ai/response_correction.go:36). 프롬프트 찾기와 수정 영향 파악의 장애 |

반대로 관찰/편집의 분리, 소스 범위, 음성 존재, 좌표·확실성, 컷 속도, 출력 시간, 자막 읽기 시간·겹침·스타일·본문 window는 실제 소비자가 요구하는 계약이다. 길다는 이유로 삭제하면 안 된다. 자막 `short_text`도 실제 fallback에 쓰인다.

최대32줄·줄당500자·전체2,000자는 동시에 지킬 수 있는 **상한**이며 서로 모순되지 않는다. 모든 상한을 채우라는 요청도 아니다. 전체2,000자 합계는 일반 schema만으로 표현하지 못하므로 System과 [로컬 검증](/home/hrlee/project/postpilot/backend/internal/clip/spoken.go:113)이 맡는다. 한국어 heading이 영어 목표 프롬프트에 남는 것은 현재 일부 단계의 의도적 framing이며, 그 사실만으로 출력 오류라고 판정하지 않았다.

## 실제 길이와 토큰 측정

golden은 서로 다른 입력의 시험 예시다. 다음 행을 보고 ‘말투 없음이 더 길다’ 또는 ‘한국어가 더 비싸다’고 비교하면 안 된다. 특히 no-voice fixture는 런타임에서 제외될 수 있는 memories-only 기본 지침도 포함한다.

| 작성 예시의 System | Unicode 문자 | cl100k_base 텍스트 토큰 | o200k_base 텍스트 토큰 |
|---|---:|---:|---:|
| 말투 없음+템플릿+기본/사용자 지침 | 4,849 | 4,046 | 2,692 |
| 같은 조건의 수정 fixture | 4,770 | 4,086 | 2,692 |
| 템플릿 없는 다른 fixture | 1,383 | 954 | 665 |
| 스토리라인 후 작성의 다른 fixture | 1,252 | 891 | 620 |
| 한국어 자연스러움 기본 지침만 | 640 | 574 | 397 |

no-voice 작성 fixture의 작문 지침 영역은2,869자·15개 bullet이다. 전체 System의 약59%를 차지한다. 이는 어떤 영역을 살펴볼지 알려주는 크기 정보이며, 지침이 원고 품질을 낮췄다는 증거는 아니다.

태그 지침만 내용상 같은 영어 후보로 옮기고 한국어 고유명사·태그 예시는 유지한 오프라인 비교는 다음과 같다.

| 태그 지침 텍스트 | 문자 | cl100k_base | o200k_base |
|---|---:|---:|---:|
| 현재 한국어 | 541 | 518 | 349 |
| 진단용 영어 지시+한국어 예시 | 1,020 | 261 | 245 |

이 번역 후보는 실제 모델에서 평가되지 않았다. 문자가 늘어도 토큰은 줄 수 있음을 보여준다. 현재 코드의 영어 사본은 영어 결과용이며 일부 이름과 예시도 영어이므로, 한국어 출력 비교의 동일 조건으로 그대로 사용하지 않았다.

위 번역을 no-voice 작성 System에서 태그 지침 한 곳에만 대입하면 **4,046→3,789(cl100k, 약6.35%)**, **2,692→2,589(o200k, 약3.83%)**다. User·다른 지침·출력 계약은 그대로 두었다. 이는 해당 fixture의 텍스트 차이이며 실제 비용·지연·품질의 측정이 아니다. 출력 토큰, cache hit, 역할/스키마/미디어 overhead까지 포함한 전체 비용의 같은 감소율을 뜻하지 않는다.

영상은 소스1개·관찰1개·15초·선언 자막2개·사용자 지침 없음의 합성 입력을 실제 builder에 넣었다.

| 단계 | System 문자 | User 문자 | 별도 schema UTF-8 bytes |
|---|---:|---:|---:|
| Flow plain | 6,532 | 993 | 0 |
| Flow structured | 5,307 | 993 | 1,249 |
| Storyline plain | 2,581 | 929 | 0 |
| Storyline structured | 2,125 | 929 | 553 |
| Narrate plain | 4,649 | 1,480 | 0 |
| Narrate structured | 3,469 | 1,480 | 1,082 |
| SpokenScript plain/structured | 3,169 | 929 | 0 / 626 |
| Observe structured | 4,822 | 129 | 1,518 |

SpokenScript의 구조 schema626bytes는 checked-in schema의 같은 projection을 별도로 재현한 값이며 실제 wire 캡처가 아니다. 최대 파일명 Observe 기존 fixture는 encoded JSON7,226/7,952bytes를 사용했다. 이 한도 역시 문자/바이트 allowance이며 모델 context token 수가 아니다.

`cl100k_base`·`o200k_base`는 두 참고 vocabulary다. 실제 사용하는 등록 모델의 tokenizer로 확정한 것이 아니다. OpenAI 공식 안내도 로컬 tokenizer의 텍스트 계산과 역할·schema·이미지·파일을 포함한 요청 계산을 구분한다. [Counting tokens](https://developers.openai.com/api/docs/guides/token-counting)

## 한국어 지시와 영어 지시를 비교할 때의 구분

‘프롬프트를 영어로 한다’는 말은 적어도 여섯 가지 다른 변경을 포함할 수 있다.

| 변경 요소 | 이번 과업에서의 의미 | 단독 비교를 위한 고정 조건 |
|---|---|---|
| 공통 과업 지시 | 무엇을 만들고 어떤 형식을 지킬지 | 의미·순서·제약·output contract를 같게 유지 |
| 작성 정책 | 기본 지침과 사용자 지침의 언어 | 어느 정책을 시험하는지 명시하고 활성 상태 유지 |
| 사용자 사실자료 | 메모·상호·지역·메뉴·가격·감상 | 영어 지시 비교를 위해 원자료까지 번역하지 않기 |
| 말투와 예시 | 한국어 어미·조사·문단·표현 | 선택한 한국어 투영과 발췌 예시를 byte 수준에서 동일하게 유지 |
| 추론 유도 | 설명·단계별 생각·영어로 재표현 요구 | 언어 변경과 함께 추가하지 않기 |
| 최종 출력 | 제목·본문·태그·자막의 언어 | 한국어로 고정. JSON key·enum·파일명·원래 브랜드 표기는 별도 보존 |

지시문만 영어로 바꾸어 한국어 결과를 얻는 것과, 사실자료를 영어로 번역해 영어 결과를 만든 뒤 한국어로 재번역하는 것은 다른 파이프라인이다. 후자는 번역 손실·추가 호출·고유명사 변화까지 섞인다.

## 1차 언어 연구를 대조한 결과

| 자료·연도 | 실제 비교 | 확인 결과 | 적용 한계 |
|---|---|---|---|
| [Beyond English, NAACL2025](https://aclanthology.org/2025.findings-naacl.73/) | 지시·context·예시·출력의24개 조합. GPT-3.5 중심, Mixtral/Gemini1.0/BLOOMZ도 검토.35개 언어 | 한국어 요약에서 선정한 최적 조합의 ROUGE11.84는 전체 한국어 대비36.99%, 전체 영어 대비11.78% 향상 | 여러 조합 중 선택한 최댓값이며 영어 System 단독 효과가 아님. 영어 결과는 평가 전 역번역. overlap 점수는 한국어 블로그 문체 평가가 아님 |
| [Breaking the Language Barrier, NAACL2024](https://aclanthology.org/2024.naacl-short.75/) | PaLM2-S/L,108개 언어. 원자료 직접 처리와 영어 사전번역 비교 | 한국어 결과로 평가한 요약 ROUGE-L은 S에서 직접31.16/번역21.40, L에서 직접9.39/번역24.11로 방향이 반대 | 원자료 번역 파이프라인 연구이며 instruction만 바꾼 것이 아님. 전체94개 언어의 직접 처리 우세를 모든 한국어 과업에 적용 불가 |
| [EnKoQA, EMNLP2025](https://arxiv.org/html/2410.18436v3) | 한국 문화 QA의 영어·한국어 용어 혼합·역번역 한국어·원래 한국어 | GPT-4o의 혼합78.23/영어73.33, Claude3.5의78.65/73.71. 원래 한국어는 부록에서80.14/80.93로 혼합보다 높음 | 주표의 KO는 원래 한국어가 아니라 역번역. 혼합 지시가 한국어보다 우월하다는 결론 불가. 문화 QA와 문체 생성은 다름 |
| [KITE, 2025](https://arxiv.org/html/2510.15558v1) | 일반 지시427개와 한국어 특화100개: 존댓말·조사·숫자·삼행시 | GPT-4o zero-shot 일반89.35/한국어 특화61.42. 예시 효과도 모델마다 다름 | KO/EN instruction 직접 비교가 아님. 한국어 출력 평가 항목을 따로 만들 근거 |
| [Language Confusion, EMNLP2024](https://aclanthology.org/2024.emnlp-main.380/) | 한국어 포함15개 언어, 단일 언어와 영어→비영어 생성. 여러 GPT·Llama·Mistral·Command | 복잡한 cross-lingual 과업에서 언어 혼동이 나타나며 모델별 차이가 있음. 출력 언어를 독립 지시로 명시하는 방식이 유용 | 언어별 합계 지표를 한국어 문체 정확도로 해석 불가. JSON 영어 key와 허용 브랜드명을 출력 언어 오류로 검사하면 안 됨 |
| [Korean Idiom Matching, RANLP2025](https://aclanthology.org/2025.ranlp-1.156.pdf) | 한국어 관용구1,175개 의미 선택, 직접 답과 CoT 지시 | CoT에서 GPT-4o87.32→86.47, HyperCLOVA90.38→83.83, o3-mini86.72→76.68; 일부 mini는 개선 | 수학 reasoning의 성공을 한국어 의미 과업에 일반화할 수 없다는 자료. 지시 언어의 직접 비교도, 블로그 생성 실험도 아님 |
| [XLT, EMNLP2023](https://arxiv.org/html/2305.07004v2) | 영어 expert 지시·영어 재표현·분석·CoT·target 출력 결합 | 여러 과업의 향상을 보고 | baseline도 영어이며 native 지시 비교를 제외. generation subset에 한국어 없음. 영어 지시 하나의 한국어 글쓰기 효과로 인용 불가 |
| [Native vs Non-Native, 2024](https://arxiv.org/html/2409.07054v1) | 아랍어11과업, 영어·아랍어·혼합 지시, GPT-4o/Llama/Jais | 전체 평균은 영어 우세지만 claim detection에서 GPT-4o 영어 우세, Llama 아랍어 우세로 방향이 반대 | 지시 언어 효과의 모델·과업 차이를 보여주는 자료. 한국어 블로그에 직접 적용할 수치는 아님 |

위 자료를 종합한 **추론**은 영어 공통 지시가 유용한 모델·과업이 있을 수 있고, 한국어 고유 표현·문체의 보존이 필요한 조건에서는 전체 번역이 손해일 수 있다는 것이다. 현재 등록된 정확한 모델·provider 경로에서 시험하기 전에는 한 방식의 우월성을 확정하지 않는다.

## 공식 지침과 구조 연구가 뒷받침하는 범위

- OpenAI는 역할, Markdown/XML 구획, typed input, 코드에 둔 feature-local prompt builder, 대표 fixture와 평가를 안내한다. 현재 PostPilot의 조립 함수·자료 타입·golden은 이 방향에 이미 맞는 부분이 있다. 모든 프롬프트를 거대한 한 파일로 모으는 결론은 이 가이드가 뒷받침하지 않는다. [Prompt engineering](https://developers.openai.com/api/docs/guides/prompt-engineering)
- OpenAI reasoning 안내는 단순하고 직접적인 지시, 명확한 경계, 필요할 때만 맞는 예시를 권한다. 일반적인 ‘단계별로 생각’ 추가가 항상 개선을 주지는 않는다. **사용자가 검토·저장하는 스토리라인은 제품의 결과물이므로 내부 추론 지시와 같은 것으로 취급해 삭제하면 안 된다.** [Reasoning best practices](https://developers.openai.com/api/docs/guides/reasoning-best-practices)
- 큰 context를 넣는 방법은 실제 context 크기별 평가가 필요하다. Lost in the Middle은 다문서 QA·key/value 검색에서 위치에 따른 차이를 측정한2024년 연구다. 이것이 몇 천 토큰의 현재 블로그 prompt에서 같은 실패율을 뜻하거나, 모든 정보를 무조건 끝에 두라는 규칙은 아니다. [OpenAI accuracy guidance](https://developers.openai.com/api/docs/guides/optimizing-llm-accuracy), [원논문](https://aclanthology.org/2024.tacl-1.9/)
- FollowBench와 ComplexBench는 제약의 개수뿐 아니라 순서·조건·중첩의 수행을 평가한다. ComplexBench는 중국어 중심의2024년 모델 비교이므로 한국어 prompt 언어 선택의 직접 증거가 아니다. PostPilot에는 규칙별 조건·우선순위·소비단계를 따로 점검할 근거로 사용한다. [FollowBench](https://arxiv.org/abs/2310.20410), [ComplexBench](https://arxiv.org/html/2407.03978v3)
- 2026년 CCR-Bench도 content/format 결합과 workflow를2025년 계열 모델들에서 평가했다. 실제 산업 입력을 쓰지만 한국어 블로그나 모든 최신 모델의 성능 자료는 아니다. 복합 계약을 직접 시험하는 평가 관점으로 참고했다. [CCR-Bench](https://arxiv.org/html/2603.07886v1)
- Anthropic은 목표 출력 언어, 원래 문자체계, 문화 맥락을 명시하도록 안내한다. 공개 한국어 수치는 번역 MMLU의 영어 대비 상대 성능이며 영어 지시 우월성·블로그 문체 점수가 아니다. 실제 과업에 가까운 예시와 자료 경계도 권한다. [Multilingual support](https://platform.claude.com/docs/en/build-with-claude/multilingual-support), [Prompting practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices)
- Gemini는 명확한 과업·제약·구획·예시·출력 형식과 structured output 사용을 안내한다. 공식 문서의 영어 예제는 KO/EN 비교 실험이 아니다. [Prompting strategies](https://ai.google.dev/gemini-api/docs/prompting-strategies)
- 평가에서는 형식·사실·지시 준수와 사람의 원고 선호를 나눠 본다. 자동 judge는 길이·순서 편향이 있으므로 현재 제품의 사람 비교를 대체할 근거로 삼지 않는다. [OpenAI evaluation guidance](https://developers.openai.com/api/docs/guides/evaluation-best-practices)

## 비교할 언어 후보

| 후보 | 유지할 내용 | 기대와 위험 |
|---|---|---|
| 한국어 지시+한국어 자료/예시+한국어 출력 | 현재 코드 기준에 가장 가까운 비교군 | 관리·원문 대조가 쉽고 한국어 규칙의 의미가 직접 보임. 특정 모델에 최적인지는 미정 |
| 영어 공통 지시+한국어 자료/말투/기본 지침+한국어 출력 | facts·고유명사·어미·작성 정책의 byte를 유지 | 공통 task/format 언어만 비교할 수 있음. 출력 언어 혼동·지시 번역 차이를 평가해야 함 |
| 영어 공통 지시와 일반 정책+한국어 표현 규칙/예시+한국어 출력 | 한국어 관련 문법·이름·사실을 그대로 둠 | 토큰 절감과 다국어 유지관리 후보. 바뀌는 정책이 늘어 원인 분리가 어려우므로 후속 조건으로 검토 |
| 전체 자료 영어 번역→영어 작성→한국어 재번역 | 사실·문체도 번역됨 | 별도 파이프라인이며 추가 비용·고유명사·말투 손실이 섞임. 지시 언어만 확인하려는 첫 비교에는 부적합 |

혼합 지시는 임시 채택 결론이 아니다. 한국어 기준군보다 좋아지는지는 현재 사용하는 정확한 모델에서 판단해야 한다.

## 다음 실험을 구체적으로 만드는 방법

1. 먼저 블록·언어·자료 경계·우선순위의 계약을 정리할 후보를 고른다. 현재 입력에서 실질적으로 구별 불가능한 정보를 영어로 번역해도 구별 정보가 복구되지 않는다.
2. 구조 정리와 지시 언어 변경을 같은 비교에 섞지 않는다. 구조 실험은 같은 언어로, 언어 실험은 같은 의미·구조로 한다. 최대 N개 전환도 양쪽에 같은 조건으로 적용한다.
3. 한국어 출력, 동일한 사실·말투·템플릿·기본/사용자 지침·태그 cap, 모델 ID·reasoning·completion cap·관찰 결과를 고정한다. `LanguageEnglish`를 단순 선택하는 방식은 현재 코드에서 단일 요인 비교가 아니다.
4. 식당의 메뉴·가격·지점, 짧은 소재, 자료 없는 가격, 이름이 같은 다른 지점, 말투 없음/선택 말투, 템플릿 없음/있음, 확정 스토리와 순서 충돌, VIDEO 포함 수정, 상호가 Latin 표기인 글을 포함해 대표 사례를 만든다. 구성별로 어떤 규칙이 있어야 하는지 먼저 기록한다.
5. 자동 검사는 JSON·블록·이름/가격 보존·첨부·개수·수정 범위처럼 확인 가능한 조건을 맡는다. 실제 사실을 완전히 확인하거나 문체 자연스러움을 숫자 하나로 판정하는 기능으로 과장하지 않는다.
6. 사용자는 모델/지시 언어를 가린 결과를 같은 소재끼리 비교한다. 사실 오류·빠진 정보·어미·태그 적합성·자연스러움을 따로 기록한다. 여러 사례와 반복을 보고 판단하며, 한 번의 좋았던 출력은 공통 정책 승격의 근거로 삼지 않는다.
7. 입력/출력 token usage·cache·지연·검증 실패를 원고 품질과 함께 기록한다. 그 뒤 사용자가 실제로 발행하고 조회를 관찰한다. 원고 선호의 개선과 조회 변화의 원인은 같은 결과가 아니다.

현재 MODEL의 사람 비교 및 T627–T631 계획을 이용해 실험을 설계할 수 있다. 이 문서는 별도의 비교 시스템이나 유료 평가 호출을 추가하기로 결정한 것이 아니다.

## 기존 작업과 연결할 범위

| 기존 작업 | 이번 조사에서 연결할 내용 |
|---|---|
| T625 authoring | mode/kind별 지시, guide 원문, 문자/token 경계, 별도 설정 생성 경로 |
| T626 voice | 실제 한국어 말투·자료와 accepted projection, 예시의 출처 보존 |
| T627 generation | 블록 계약, 지침 적용 단계·우선순위, 언어 단일요인, 자료·prompt/schema provenance |
| T630 template/guideline | literal/facts 경계, 기본 지침 소유권, 태그 cap을 사용자에게 설명하는 방식 |
| T628/T629/T631 | 기존 사람 비교·기록·통합 검증. 별도 실험 구현을 중복 생성하지 않기 |
| T591–T604 media | 언어·관찰/구성·시간·음성/자막 계약의 영향 확인. 현재 작업의 소유 범위를 먼저 검토 |

이미 계획에 있는 accepted voice 확정·prompt/schema version hash·비교 기록은 새 finding으로 다시 태스크화하지 않는다. 새로운 자료 경계나 블록 설명은 해당 담당 범위의 추가 검토 재료로 남긴다. 어떤 정책을 바꿀지는 사용자 선택 후 SSOT·태스크로 전환한다.

## 검증과 한계

이번 라운드에서 기존 authoring 테스트5개와 영상/음성 테스트7개가 통과했다. 앞 라운드의 태그 관련13개 결과도 같은 제품 코드에 대한 근거로 유지한다. 임시 overlay 진단5개는 예상한 불일치·정보 소실·후처리를 재현했다. **진단이 통과했다는 것은 문제가 해결됐다는 의미가 아니다.**

```sh
cd backend
go test -timeout 2m -v ./internal/authoring -run '^Test(FrozenContextNeverClipsCurrentDraftOrLatestRequest|RecommendationParsingRequiresAllEightDistinctDomainValidArtifacts|RefinementRejectsModelIdentifiersAndKeepsSelectedIdentity|SmallContextFitsCompletionCapAndRejectsImpossibleInput|EscapedRecentHistoryFitsActualSerializedContext)$'
go test -timeout 2m -v ./internal/clip/ai ./internal/clip -run '^(TestObservePromptFitsTheInlineInputAllowance|TestObservePromptUsesTheProjectLanguageAndPreservesSpeech|TestEveryClipWritingCallEndsWithTheVideoGuidelines|TestSpokenWriterPrecedesFlowWithoutCaptionWritingAndKeepsExactWords|TestNarratedAIRevisionChangesSpokenScriptWithoutCaptionOrSynthesisWork|TestSpokenUnicodeAndWholeScriptBounds|TestComposeCorrectsArithmeticButKeepsValidTiming)$'
```

선택 이유: authoring의 입력 확정·모드별 결과 계약·context 조건과, 영상의 언어 설명·지침 적용·음성/자막 호출 분리·Unicode 상한·duration 재계산을 확인하기 위해서다. 제품 동작 변경이 없으므로 전체 CI·빌드·배포 게이트는 이번 검증 대상이 아니다.

로컬 진단의 파일·재현 명령·합성 입력은 재현JSON에 저장했다. 실모델의 유료 호출, 영어/한국어 생성 품질 비교, 실제 발행 후 조회 관찰은 아직 하지 않았다. 현행 모델·provider의 모든 설정을 가져온 것도 아니다. 접근하지 못한 ICLR2026 localization 초고는 중심 근거에서 제외했다.

문서 검증은 `git diff --check`와 `pnpm exec haeram-spec-creator lint`가 통과했다. spec lint에는 기존 경고112건·검토 후보14건이 남는다.31개 검토 항목·20개 서로 다른1차 출처 링크·소스 파일/줄 앵커·재현JSON·아이디에이션 open 상태·STATE20줄 log를 확인했다. 세 독립 검토에서 코드 근거, 측정 단위, 논문의 비교 조건과 한계를 대조했다.
