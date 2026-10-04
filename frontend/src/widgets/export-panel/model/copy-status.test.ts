import { describe, expect, it } from 'vitest'
import type { PostContent } from '@/shared/api'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import {
  captionCopyStatus,
  captionFellBack,
  exportCopyStatus,
  type CopyFeedbackState,
  type ExportTexts,
} from './copy-status'

const CONTENT = POST_CONTENT_FIXTURE
const NONE: CopyFeedbackState = { copied: undefined, manualCopy: undefined }

/** The texts on screen for `CONTENT`, on the Naver tab unless a test says otherwise. */
function texts(overrides: Partial<ExportTexts> = {}): ExportTexts {
  return {
    content: CONTENT,
    format: 'naver',
    output: 'body',
    outputWithTags: 'body\n\n#제주 #산책 #여행',
    hashtags: '#제주 #산책 #여행',
    ...overrides,
  }
}

describe('a confirmation', () => {
  it('holds only while the content still holds the value it confirmed', () => {
    const copiedTags: CopyFeedbackState = {
      copied: { target: 'tags', value: '#제주 #산책 #여행' },
      manualCopy: undefined,
    }
    expect(exportCopyStatus(copiedTags, texts()).tags).toBe('copied')
    // The tag list changed under it (EXPORT-20): the confirmation is for a list no longer shown.
    expect(exportCopyStatus(copiedTags, texts({ hashtags: '#제주 #산책' })).tags).toBeUndefined()

    const copiedTitle: CopyFeedbackState = {
      copied: { target: 'title', value: CONTENT.title },
      manualCopy: undefined,
    }
    expect(exportCopyStatus(copiedTitle, texts()).title).toBe('copied')
    const retitled: PostContent = { ...CONTENT, title: '다른 제목' }
    expect(exportCopyStatus(copiedTitle, texts({ content: retitled })).title).toBeUndefined()
  })

  it('is reported on its own line and no other', () => {
    const status = exportCopyStatus(
      { copied: { target: 'tags', value: '#제주 #산책 #여행' }, manualCopy: undefined },
      texts(),
    )
    expect(status).toMatchObject({
      tags: 'copied',
      title: undefined,
      output: undefined,
      outputWithTags: undefined,
    })
  })

  it('confirms the combined body only on the Naver tab', () => {
    const copied: CopyFeedbackState = {
      copied: { target: 'outputWithTags', value: 'body\n\n#제주 #산책 #여행' },
      manualCopy: undefined,
    }
    expect(exportCopyStatus(copied, texts()).outputWithTags).toBe('copied')
    expect(exportCopyStatus(copied, texts({ format: 'tistory' })).outputWithTags).toBeUndefined()
  })
})

describe('a manual fallback', () => {
  it('keeps the Naver raw field hidden until the body copy falls back', () => {
    expect(exportCopyStatus(NONE, texts())).toMatchObject({
      rawFieldVisible: false,
      outputFellBack: false,
    })
    // The other formats are markup read as source: their field is always there.
    expect(exportCopyStatus(NONE, texts({ format: 'markdown' })).rawFieldVisible).toBe(true)

    const fellBack = exportCopyStatus(
      { copied: undefined, manualCopy: { target: 'output', value: 'body', source: CONTENT } },
      texts(),
    )
    expect(fellBack).toMatchObject({
      rawFieldVisible: true,
      fallbackOutput: 'body',
      output: 'manual',
    })
  })

  it('reveals the combined body when that is the copy that fell back', () => {
    const status = exportCopyStatus(
      {
        copied: undefined,
        manualCopy: {
          target: 'outputWithTags',
          value: 'body\n\n#제주 #산책 #여행',
          source: CONTENT,
        },
      },
      texts(),
    )
    expect(status).toMatchObject({
      rawFieldVisible: true,
      fallbackOutput: 'body\n\n#제주 #산책 #여행',
      outputWithTags: 'manual',
      output: undefined,
    })
  })

  it('dissolves when the content changes, and stays gone when the edit is undone', () => {
    const manual: CopyFeedbackState = {
      copied: undefined,
      manualCopy: { target: 'output', value: 'body', source: CONTENT },
    }
    expect(exportCopyStatus(manual, texts({ output: 'edited' })).rawFieldVisible).toBe(false)
    // The same string again, from a different content: the dismissed fallback must not return.
    const undone: PostContent = { ...CONTENT }
    expect(exportCopyStatus(manual, texts({ content: undone })).rawFieldVisible).toBe(false)

    const tags: CopyFeedbackState = {
      copied: undefined,
      manualCopy: { target: 'tags', value: '#제주 #산책 #여행', source: CONTENT },
    }
    expect(exportCopyStatus(tags, texts()).tags).toBe('manual')
    expect(exportCopyStatus(tags, texts({ content: undone })).tags).toBeUndefined()
  })

  // On the Naver tab only the BODY reveals the raw field; a tags fallback selects its own field.
  it('reveals nothing for a tags fallback', () => {
    const status = exportCopyStatus(
      {
        copied: undefined,
        manualCopy: { target: 'tags', value: '#제주 #산책 #여행', source: CONTENT },
      },
      texts(),
    )
    expect(status.rawFieldVisible).toBe(false)
    expect(status.tags).toBe('manual')
  })
})

describe("a caption's line (EXPORT-24)", () => {
  it('confirms its own marker only, and only for the caption it copied', () => {
    const copied: CopyFeedbackState = {
      copied: { target: 'caption:0', value: '비 뒤의 바다' },
      manualCopy: undefined,
    }
    expect(captionCopyStatus(copied, CONTENT, 0, '비 뒤의 바다')).toBe('copied')
    expect(captionCopyStatus(copied, CONTENT, 1, '비 뒤의 바다')).toBeUndefined()
    expect(captionCopyStatus(copied, CONTENT, 0, '고친 캡션')).toBeUndefined()
  })

  it('falls back on its own marker, and dissolves with the content it fell back for', () => {
    const manual: CopyFeedbackState = {
      copied: undefined,
      manualCopy: { target: 'caption:0', value: '비 뒤의 바다', source: CONTENT },
    }
    expect(captionFellBack(manual, CONTENT, 0, '비 뒤의 바다')).toBe(true)
    expect(captionCopyStatus(manual, CONTENT, 0, '비 뒤의 바다')).toBe('manual')
    expect(captionFellBack(manual, CONTENT, 1, '비 뒤의 바다')).toBe(false)
    expect(captionFellBack(manual, { ...CONTENT }, 0, '비 뒤의 바다')).toBe(false)
  })
})
