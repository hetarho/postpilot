import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  BlockList,
  copyPostContent,
  newBlock,
  originFieldText,
  postContentWith,
  utf16RangeToScalar,
  type OriginFieldLocator,
  type OriginReview,
  type OriginSpan,
  type PostDraft,
  type SemanticOriginCategory,
} from '@/entities/post'
import { BlockType, type PostContent } from '@/shared/api'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import type { VoiceHabitPhrase } from '@/entities/voice'
import { habitSegments } from '../lib/habit-segments'
import { WritingOriginReview, type WritingOriginVoice } from './WritingOriginReview'

afterEach(() => {
  cleanup()
  window.getSelection()?.removeAllRanges()
})

const content = postContentWith(POST_CONTENT_FIXTURE, {
  title: '제주 여행',
  summary: '제주🌊 제주🌊',
  tags: ['제주'],
  blocks: [
    newBlock({ type: BlockType.TEXT, content: '주인 말 · 사진 관찰 · AI 제안' }),
    newBlock({ type: BlockType.HEADING, level: 2, content: '작은 항구' }),
    newBlock({ type: BlockType.QUOTE, content: '서두르지 않았다.' }),
    newBlock({ type: BlockType.LIST, items: ['우산', '따뜻한 차'] }),
    newBlock({ type: BlockType.IMAGE, file: 'IMG_1.jpg', alt: '빨간 지붕', caption: '제주에서' }),
  ],
})

const SOURCE_OF: Record<SemanticOriginCategory, string> = {
  owner_input: 'owner',
  photo_interpretation: 'visual',
  advertiser_request: 'advertiser',
  ai_added: 'ai',
}

function span(
  field: OriginFieldLocator,
  quote: string,
  category: SemanticOriginCategory = 'owner_input',
  occurrence = 0,
): OriginSpan {
  const text = originFieldText(content, field)!
  let start = -1
  for (let index = 0; index <= occurrence; index += 1) start = text.indexOf(quote, start + 1)
  return {
    field,
    ...utf16RangeToScalar(text, { start, end: start + quote.length })!,
    quote,
    category,
    sourceRefs: [SOURCE_OF[category]],
    reviewState: 'confirmed',
  }
}

function evidence(): OriginReview {
  return {
    version: 1,
    result: { contentRevision: 7n, contentHash: 'current-content' },
    sources: [
      {
        id: 'owner',
        kind: 'memo',
        text: '그날 제주에 가서 천천히 걸었다.',
        attachmentFilename: '',
        available: true,
      },
      {
        id: 'visual',
        kind: 'visual_observation',
        text: '이 결과를 만들 때 관찰한 빨간 지붕과 작은 항구.',
        attachmentId: 'image-1',
        attachmentFilename: 'IMG_1.jpg',
        available: true,
      },
      {
        id: 'advertiser',
        kind: 'advertiser_guide',
        text: '따뜻한 차 메뉴를 꼭 언급해 주세요.',
        attachmentFilename: '',
        available: true,
      },
      {
        id: 'ai',
        kind: 'ai_proposal',
        text: '문단의 연결을 위해 덧붙인 해석.',
        attachmentFilename: '',
        available: true,
      },
    ],
    spans: [
      span({ kind: 'title' }, '제주 여행'),
      span({ kind: 'summary' }, '제주🌊'),
      span({ kind: 'summary' }, '제주🌊', 'photo_interpretation', 1),
      span({ kind: 'tag', tagIndex: 0 }, '제주', 'ai_added'),
      span({ kind: 'block_content', blockIndex: 0 }, '주인 말'),
      span({ kind: 'block_content', blockIndex: 0 }, '사진 관찰', 'photo_interpretation'),
      span({ kind: 'block_content', blockIndex: 0 }, 'AI 제안', 'ai_added'),
      span({ kind: 'block_content', blockIndex: 1 }, '작은 항구', 'photo_interpretation'),
      span({ kind: 'block_content', blockIndex: 2 }, '서두르지 않았다.'),
      span({ kind: 'block_item', blockIndex: 3, itemIndex: 0 }, '우산'),
      span({ kind: 'block_item', blockIndex: 3, itemIndex: 1 }, '따뜻한 차', 'advertiser_request'),
      span({ kind: 'block_alt', blockIndex: 4 }, '빨간 지붕', 'photo_interpretation'),
      span({ kind: 'block_caption', blockIndex: 4 }, '제주에서'),
    ],
  }
}

function post(patch: Partial<PostDraft> = {}): PostDraft {
  return {
    slug: 'jeju',
    title: '',
    memo: '현재 메모는 변경되었다.',
    status: 'review',
    createdAt: '',
    updatedAt: '',
    template: { id: '', name: '' },
    field: '',
    templateAnswers: [],
    images: POST_IMAGES_FIXTURE,
    videos: [],
    activeJob: undefined,
    content,
    observations: [],
    pendingExperimentId: '',
    contentRevision: 7n,
    contentHash: 'current-content',
    contentOrigins: evidence(),
    inputRevision: 1n,
    machineBaselineRevision: 7n,
    canFinalize: true,
    tagCount: 3,
    useMemory: false,
    qualityRules: [],
    finalizedRevision: 0n,
    finalizedAt: '',
    publishedUrl: '',
    publishedAt: '',
    targetLanguage: 'ko',
    contentLanguage: 'ko',
    campaign: false,
    advertiserGuide: { text: '', keywords: [], hashtags: [] },
    checklistTicks: [],
    ...patch,
  }
}

function Review({
  ownerId = 'alice',
  value = post(),
  displayed = value.content!,
  pending = false,
  voice,
}: {
  ownerId?: string
  value?: PostDraft
  displayed?: PostContent
  pending?: boolean
  voice?: WritingOriginVoice
}) {
  return (
    <WritingOriginReview
      ownerId={ownerId}
      post={value}
      content={displayed}
      pending={pending}
      voice={voice}
    >
      {(renderers) => (
        <BlockList content={displayed} images={value.images} videos={value.videos} {...renderers} />
      )}
    </WritingOriginReview>
  )
}

function phrase(text: string, category: string) {
  return screen.getByRole('button', { name: `${category} 출처 보기: ${text}` })
}

describe('WritingOriginReview', () => {
  it('writes both native clipboard flavors from canonical selection without review styles or source data', () => {
    const value = post()
    const before = copyPostContent(value.content!)
    render(<Review value={value} />)
    const first = phrase('주인 말', '직접 입력 기반')
    const last = phrase('AI 제안', 'AI가 보탠 내용')
    const range = document.createRange()
    range.setStart(first.firstChild!, 0)
    range.setEnd(last.firstChild!, last.textContent!.length)
    const selection = window.getSelection()!
    selection.removeAllRanges()
    selection.addRange(range)
    const setData = vi.fn()
    fireEvent.copy(first, { clipboardData: { setData } })
    expect(setData).toHaveBeenCalledWith('text/plain', '주인 말 · 사진 관찰 · AI 제안')
    expect(setData).toHaveBeenCalledWith('text/html', '주인 말 · 사진 관찰 · AI 제안')
    expect(value.content).toEqual(before)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(selection.toString()).toBe('주인 말 · 사진 관찰 · AI 제안')
  })

  // Turning 출처 보기 off dropped the legend and its notes in one frame and pulled the draft up
  // under the owner's thumb; they fold away and back, and are out of reach while folded.
  // The toggle only shows or hides the colours: the legend stays, grey while the draft is not
  // wearing them, and 문구 출처 찾기 stays in the row, disabled — nothing appears or leaves.
  it('keeps the bar steady and greys the legend while the colours are off', async () => {
    const user = userEvent.setup()
    render(<Review />)
    const legend = screen.getByRole('list', { name: '의미의 출처' })
    const find = screen.getByRole('button', { name: '문구 출처 찾기' })
    expect(within(legend).getByText('광고주 요청')).toHaveClass('text-origin-advertiser-foreground')

    await user.click(screen.getByRole('checkbox', { name: '출처 보기' }))
    expect(screen.getByRole('list', { name: '의미의 출처' })).toBe(legend)
    const muted = within(legend).getByText('광고주 요청')
    expect(muted).toHaveClass('text-content-tertiary', 'decoration-double')
    expect(muted).not.toHaveClass('text-origin-advertiser-foreground')
    expect(screen.getByRole('button', { name: '문구 출처 찾기' })).toBe(find)
    expect(find).toBeDisabled()
    expect(screen.queryByRole('button', { name: /주인 말/ })).not.toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', { name: '출처 보기' }))
    expect(within(legend).getByText('광고주 요청')).toHaveClass('text-origin-advertiser-foreground')
    expect(find).toBeEnabled()
  })

  it('shows the named toggle by default, the four categories and the unconfirmed state', () => {
    render(<Review />)
    expect(screen.getByRole('checkbox', { name: '출처 보기' })).toBeChecked()
    const legend = screen.getByRole('list', { name: '의미의 출처' })
    expect(
      within(legend)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual(['직접 입력 기반', '사진에서 추론', '광고주 요청', 'AI가 보탠 내용', '출처 미확인'])
    // The unconfirmed state is named with its own style and its one-line meaning.
    expect(within(legend).getByText('출처 미확인')).toHaveClass(
      'text-origin-unconfirmed-foreground',
      'decoration-wavy',
    )
    expect(within(legend).getByText('출처 미확인')).toHaveAttribute(
      'title',
      '연결된 출처를 확인할 수 없는 문구',
    )
    // Each category is named, never told by colour alone, and the advertiser's has its own role.
    expect(within(legend).getByText('광고주 요청')).toHaveClass(
      'text-origin-advertiser-foreground',
      'bg-origin-advertiser-highlight',
    )
    // The longer notes are read where a phrase is looked up, not printed above the draft.
    expect(screen.queryByText(/^출처 미확인:/)).not.toBeInTheDocument()
    expect(phrase('주인 말', '직접 입력 기반')).toHaveClass('text-origin-owner-foreground')
    expect(phrase('사진 관찰', '사진에서 추론')).toHaveClass('text-origin-visual-foreground')
    expect(phrase('AI 제안', 'AI가 보탠 내용')).toHaveClass('text-origin-ai-foreground')
    expect(phrase('따뜻한 차', '광고주 요청')).toHaveClass('text-origin-advertiser-foreground')
  })

  it('opens the frozen owner input, actual visual observation, advertiser guide and AI basis without using current memo', async () => {
    const user = userEvent.setup()
    render(<Review />)
    for (const [text, category, source] of [
      ['주인 말', '직접 입력 기반', '그날 제주에 가서 천천히 걸었다.'],
      ['사진 관찰', '사진에서 추론', '이 결과를 만들 때 관찰한 빨간 지붕과 작은 항구.'],
      ['AI 제안', 'AI가 보탠 내용', '문단의 연결을 위해 덧붙인 해석.'],
      ['따뜻한 차', '광고주 요청', '따뜻한 차 메뉴를 꼭 언급해 주세요.'],
    ]) {
      await user.click(phrase(text!, category!))
      const dialog = screen.getByRole('dialog', { name: '문구의 출처' })
      expect(within(dialog).getByText(source!)).toHaveClass('text-body', 'leading-relaxed')
      expect(within(dialog).queryByText('현재 메모는 변경되었다.')).not.toBeInTheDocument()
      expect(dialog.textContent).not.toContain('signature=')
      if (text === '사진 관찰') expect(within(dialog).getByText('IMG_1.jpg')).toBeInTheDocument()
      if (text === '따뜻한 차')
        expect(within(dialog).getByText('광고주 가이드')).toBeInTheDocument()
      await user.click(within(dialog).getByRole('button', { name: '닫기' }))
    }
  })

  it('lets keyboard users open with Enter or Space, close with Escape and return focus to the phrase', async () => {
    const user = userEvent.setup()
    render(<Review />)
    const trigger = phrase('주인 말', '직접 입력 기반')
    trigger.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('dialog', { name: '문구의 출처' })).toHaveFocus()
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
    await user.keyboard(' ')
    expect(screen.getByRole('dialog', { name: '문구의 출처' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '닫기' }))
    expect(trigger).toHaveFocus()
  })

  it('offers full touch targets in a named phrase picker, with detail, Back and Escape focus return', async () => {
    const user = userEvent.setup()
    render(<Review />)
    const entry = screen.getByRole('button', { name: '문구 출처 찾기' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await user.click(entry)
    const picker = screen.getByRole('dialog', { name: '문구 출처 찾기' })
    const choice = within(picker).getByRole('button', {
      name: '본문 1 · 사진에서 추론 · 사진 관찰',
    })
    expect(choice).toHaveClass('pointer-coarse:min-h-11', 'w-full')
    await user.click(choice)
    const detail = screen.getByRole('dialog', { name: '문구의 출처' })
    expect(
      within(detail).getByText('이 결과를 만들 때 관찰한 빨간 지붕과 작은 항구.'),
    ).toBeInTheDocument()
    expect(within(detail).getByRole('heading', { name: '문구의 출처' })).toHaveFocus()
    await user.click(within(detail).getByRole('button', { name: '문구 목록으로' }))
    expect(screen.getByRole('dialog', { name: '문구 출처 찾기' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '본문 1 · 사진에서 추론 · 사진 관찰' })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(entry).toHaveFocus()
  })

  it('preserves every canonical field and native alt string when origins are toggled', async () => {
    const user = userEvent.setup()
    render(<Review />)
    for (const [text, category] of [
      ['제주 여행', '직접 입력 기반'],
      ['제주', 'AI가 보탠 내용'],
      ['작은 항구', '사진에서 추론'],
      ['서두르지 않았다.', '직접 입력 기반'],
      ['우산', '직접 입력 기반'],
      ['따뜻한 차', '광고주 요청'],
      ['제주에서', '직접 입력 기반'],
    ])
      expect(phrase(text!, category!)).toBeInTheDocument()
    const summary = screen.getAllByRole('button', { name: /출처 보기: 제주🌊$/ })
    expect(summary).toHaveLength(2)
    expect(summary[0]).toHaveClass('text-origin-owner-foreground')
    expect(summary[1]).toHaveClass('text-origin-visual-foreground')
    expect(screen.getByRole('img', { name: '빨간 지붕' })).toHaveAttribute('alt', '빨간 지붕')
    expect(
      within(screen.getByRole('group', { name: '대체 텍스트 출처' })).getByRole('button', {
        name: /빨간 지붕$/,
      }),
    ).toBeInTheDocument()
    const beforeText = phrase('주인 말', '직접 입력 기반').parentElement!.textContent
    await user.click(screen.getByRole('checkbox', { name: '출처 보기' }))
    expect(screen.queryByRole('button', { name: /출처 보기:/ })).not.toBeInTheDocument()
    expect(screen.getByText('주인 말 · 사진 관찰 · AI 제안')).toHaveTextContent(beforeText!)
    expect(screen.getByRole('img', { name: '빨간 지붕' })).toHaveAttribute('alt', '빨간 지붕')
    expect(screen.queryByRole('group', { name: '대체 텍스트 출처' })).not.toBeInTheDocument()
  })

  it('keeps drag selection and copy text across mixed origins, and ordinary selection after hiding origins', async () => {
    const user = userEvent.setup()
    render(<Review />)
    const first = phrase('주인 말', '직접 입력 기반')
    const last = phrase('AI 제안', 'AI가 보탠 내용')
    const range = document.createRange()
    range.setStart(first.firstChild!, 0)
    range.setEnd(last.firstChild!, 5)
    window.getSelection()!.addRange(range)
    fireEvent.mouseUp(last)
    fireEvent.click(last)
    expect(window.getSelection()!.toString()).toBe(content.blocks[0]!.content)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    window.getSelection()!.removeAllRanges()
    await user.click(screen.getByRole('checkbox', { name: '출처 보기' }))
    const plain = screen.getByText(content.blocks[0]!.content)
    range.selectNodeContents(plain)
    window.getSelection()!.removeAllRanges()
    window.getSelection()!.addRange(range)
    expect(window.getSelection()!.toString()).toBe(content.blocks[0]!.content)
  })

  it('dismisses old details on owner, post and result changes without resurrecting them', async () => {
    const user = userEvent.setup()
    const original = post()
    const { rerender } = render(<Review value={original} />)
    await user.click(phrase('주인 말', '직접 입력 기반'))
    rerender(<Review ownerId="bob" value={post({ slug: 'another', contentOrigins: undefined })} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByText('그날 제주에 가서 천천히 걸었다.')).not.toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: '출처 보기' })).toBeChecked()
    rerender(<Review value={original} />)
    await user.click(phrase('주인 말', '직접 입력 기반'))
    rerender(<Review value={post({ contentRevision: 8n, contentHash: 'new-result' })} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: '직접 입력 기반 출처 보기: 주인 말' }),
    ).not.toBeInTheDocument()
    rerender(<Review value={original} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('unconfirms pending or unsaved changed documents until the aligned authoritative result arrives', async () => {
    const user = userEvent.setup()
    const original = post()
    const { rerender } = render(<Review value={original} />)
    await user.click(phrase('주인 말', '직접 입력 기반'))
    rerender(<Review value={original} pending />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(
      screen.getByRole('article').querySelector('[class*="text-origin-owner"]'),
    ).not.toBeInTheDocument()
    await user.click(phrase(content.blocks[0]!.content, '출처 미확인'))
    expect(screen.getByText(/^수정한 글과 출처가/)).toBeInTheDocument()
    const edited = copyPostContent(content)
    edited.title = '수정한 제목'
    rerender(<Review value={original} displayed={edited} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(
      screen.getByRole('article').querySelector('[class*="text-origin-owner"]'),
    ).not.toBeInTheDocument()
    expect(phrase('수정한 제목', '출처 미확인')).toBeInTheDocument()
    rerender(<Review value={original} />)
    expect(phrase('주인 말', '직접 입력 기반')).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('withdraws deleted visual evidence immediately and never retargets it to another attachment with the same filename', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<Review />)
    await user.click(phrase('사진 관찰', '사진에서 추론'))
    rerender(<Review value={post({ images: [] })} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await user.click(phrase('사진 관찰', '출처 미확인'))
    expect(screen.getByText(/^이 문구에 연결된 원본이/)).toBeInTheDocument()
    expect(
      screen.queryByText('이 결과를 만들 때 관찰한 빨간 지붕과 작은 항구.'),
    ).not.toBeInTheDocument()
    rerender(
      <Review value={post({ images: [{ ...POST_IMAGES_FIXTURE[0]!, id: 'replacement' }] })} />,
    )
    expect(
      screen.queryByRole('button', { name: '사진에서 추론 출처 보기: 사진 관찰' }),
    ).not.toBeInTheDocument()
  })

  it('keeps legacy, malformed, stale and unavailable data readable as unconfirmed', () => {
    const malformed = { ...evidence(), sources: [null] } as unknown as OriginReview
    const unavailable = evidence()
    unavailable.sources[0]!.available = false
    const fixtures = [
      undefined,
      malformed,
      { ...evidence(), result: { contentRevision: 6n, contentHash: 'older-result' } },
      unavailable,
    ]
    const { rerender } = render(<Review value={post({ contentOrigins: undefined })} />)
    for (const contentOrigins of fixtures) {
      rerender(<Review value={post({ contentOrigins })} />)
      expect(screen.getByRole('article')).toHaveTextContent('주인 말 · 사진 관찰 · AI 제안')
      expect(
        screen.getByRole('article').querySelector('[class*="text-origin-owner"]'),
      ).not.toBeInTheDocument()
    }
  })

  it('shows an honest explanation for AI-added meaning without recorded proposal refs, including a published read view', async () => {
    const user = userEvent.setup()
    const review = evidence()
    review.spans.find((entry) => entry.quote === 'AI 제안')!.sourceRefs = []
    render(<Review value={post({ status: 'published', contentOrigins: review })} />)
    await user.click(phrase('AI 제안', 'AI가 보탠 내용'))
    expect(screen.getByText(/^제공된 입력, 광고주 가이드나 시각 관찰 외에/)).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  describe('말투 view', () => {
    function habit(patch: Partial<VoiceHabitPhrase> = {}): VoiceHabitPhrase {
      return {
        field: { kind: 'block_content', blockIndex: 2 },
        start: 5,
        end: 9,
        quote: '않았다.',
        kind: 'ending',
        term: '~았다',
        voicePerHundred: 12.5,
        textCount: 3,
        example: '그날은 조금 늦었다.',
        exampleFictional: false,
        ...patch,
      }
    }
    const reading = <p>6개 중 5개 비슷해요</p>
    const voice = (patch: Partial<WritingOriginVoice> = {}): WritingOriginVoice => ({
      applicable: true,
      revision: 7n,
      habits: [
        habit(),
        habit({
          field: { kind: 'title' },
          start: 0,
          end: 2,
          quote: '제주',
          kind: 'signature',
          term: '제주',
          voicePerHundred: 0,
          textCount: 1,
          example: '제주는 늘 좋아요.',
          exampleFictional: true,
        }),
      ],
      reading,
      ...patch,
    })

    it('shows one view at a time with 의미 출처 first, and the reading with underlined habits in 말투', async () => {
      const user = userEvent.setup()
      render(<Review voice={voice()} />)
      const tabs = screen.getAllByRole('tab')
      expect(tabs.map((tab) => tab.textContent)).toEqual(['의미 출처', '말투'])
      expect(tabs[0]).toHaveAttribute('aria-selected', 'true')
      expect(screen.queryByText('6개 중 5개 비슷해요')).not.toBeInTheDocument()

      await user.click(tabs[1])
      expect(screen.getByText('6개 중 5개 비슷해요')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /출처 보기: 주인 말/ })).not.toBeInTheDocument()
      expect(screen.queryByRole('list', { name: '의미의 출처' })).not.toBeInTheDocument()
      const ending = screen.getByRole('button', { name: '끝맺음 말투 보기: 않았다.' })
      expect(ending).toHaveClass('text-voice-habit-foreground', 'decoration-2', 'decoration-solid')
      expect(ending.parentElement).toHaveTextContent('서두르지 않았다.')

      await user.click(ending)
      const sheet = screen.getByRole('dialog', { name: '말투 습관' })
      expect(within(sheet).getByText('끝맺음 · ~았다')).toBeInTheDocument()
      expect(within(sheet).getByText('내 말투 100문장에 12.5번 · 이 글 3번')).toBeInTheDocument()
      expect(within(sheet).getByText('“그날은 조금 늦었다.”')).toBeInTheDocument()
      expect(within(sheet).queryByText('AI가 만든 가상의 예문')).not.toBeInTheDocument()
      await user.click(within(sheet).getByRole('button', { name: '닫기' }))

      await user.click(screen.getByRole('button', { name: '이 사람만의 표현 말투 보기: 제주' }))
      const fictional = screen.getByRole('dialog', { name: '말투 습관' })
      expect(within(fictional).getByText('이 글 1번')).toBeInTheDocument()
      expect(within(fictional).queryByText(/100문장/)).not.toBeInTheDocument()
      expect(within(fictional).getByText('AI가 만든 가상의 예문')).toBeInTheDocument()
      await user.click(within(fictional).getByRole('button', { name: '닫기' }))

      // Turning 출처 보기 off hides both views.
      await user.click(screen.getByRole('checkbox', { name: '출처 보기' }))
      expect(screen.queryByRole('button', { name: /말투 보기/ })).not.toBeInTheDocument()
      expect(screen.queryByText('6개 중 5개 비슷해요')).not.toBeInTheDocument()
    })

    it.each([
      [{ applicable: false, reason: 'none' as const, habits: [] }, '말투 없음'],
      [{ applicable: false, reason: 'deleted' as const, habits: [] }, '삭제된 말투'],
      [{ applicable: false, reason: 'unmade' as const, habits: [] }, '아직 만들지 않은 말투'],
    ])('disables 말투 with its reason when the reading is not applicable', (value, reason) => {
      render(<Review voice={value} />)
      expect(screen.getByRole('tab', { name: '말투' })).toBeDisabled()
      expect(screen.getByText(reason)).toBeInTheDocument()
    })

    it('names the reason from the post itself when no reading is passed', () => {
      render(<Review />)
      expect(screen.getByRole('tab', { name: '말투' })).toBeDisabled()
      expect(screen.getByText('말투 없음')).toBeInTheDocument()
    })

    it('draws no habit over another revision or over pending local edits', async () => {
      const user = userEvent.setup()
      const { rerender } = render(<Review voice={voice({ revision: 6n })} />)
      await user.click(screen.getByRole('tab', { name: '말투' }))
      expect(screen.queryByRole('button', { name: /말투 보기/ })).not.toBeInTheDocument()
      expect(screen.getByText('서두르지 않았다.')).toBeInTheDocument()

      rerender(<Review voice={voice()} pending />)
      expect(screen.queryByRole('button', { name: /말투 보기/ })).not.toBeInTheDocument()
      rerender(<Review voice={voice()} />)
      expect(screen.getByRole('button', { name: '끝맺음 말투 보기: 않았다.' })).toBeInTheDocument()
    })

    it('splits by rune offsets and leaves a moved or overlapping quote plain', () => {
      const field = { kind: 'summary' } as const
      const text = '제주🌊 진짜 좋아요'
      const found = habitSegments(text, field, [
        habit({ field, start: 4, end: 6, quote: '진짜', kind: 'adverb' }),
        habit({ field, start: 5, end: 7, quote: '짜 ', kind: 'adverb' }),
        habit({ field, start: 7, end: 10, quote: '맛있다', kind: 'ending' }),
      ])
      expect(found.map((segment) => [segment.text, Boolean(segment.habit)])).toEqual([
        ['제주🌊 ', false],
        ['진짜', true],
        [' 좋아요', false],
      ])
    })
  })
})
