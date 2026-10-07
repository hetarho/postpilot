import { create } from '@bufbuild/protobuf'
import { BlockType, PostContentSchema } from '@/shared/api'

export const OWNER_CONTROL_TEXT =
  '직접 입력 기반 · 사진에서 추론 · AI가 보탠 내용 · 출처 미확인 · 요청 기술 보기 · <write>원문 & 😀</write> · 사진_7_내_원문_사진 · <span class="text-origin-owner-foreground">출처 보기</span>'
export const OWNER_CONTROL_CAPTION = '출처 미확인 <write>소유자가 쓴 캡션 & 😀</write>'
export const PRIVATE_REVIEW_SENTINEL = 'PRIVATE-ORIGIN-SOURCE-EXPLANATION'
export const PRIVATE_REQUEST_SENTINEL = 'PRIVATE-TECHNICAL-REQUEST-METADATA'

/** Review chrome and private sidecars deliberately resemble genuine canonical owner text.
 * Exports consume the canonical fields and must never strip owner wording by label matching. */
export const OWNER_CONTROL_CONTENT = {
  ...create(PostContentSchema, {
    title: '요청 기술 보기 <write>소유자 제목</write>',
    summary: '출처 미확인 · 소유자 요약',
    tags: ['owner_input', '출처 미확인'],
    blocks: [
      { type: BlockType.TEXT, content: OWNER_CONTROL_TEXT },
      { type: BlockType.HEADING, level: 2, content: '직접 입력 기반' },
      { type: BlockType.QUOTE, content: 'AI가 보탠 내용이라는 소유자 문장' },
      { type: BlockType.LIST, items: ['사진에서 추론이라는 소유자 항목'] },
      {
        type: BlockType.IMAGE,
        file: 'IMG_1.jpg',
        alt: '출처 미확인이라는 소유자 대체 텍스트',
        caption: OWNER_CONTROL_CAPTION,
      },
    ],
  }),
  contentOrigins: {
    sources: [{ text: PRIVATE_REVIEW_SENTINEL }],
    highlightClass: 'text-origin-owner-foreground',
  },
  requestInspection: { prompt: PRIVATE_REQUEST_SENTINEL, callId: 'private-call-identity' },
}
