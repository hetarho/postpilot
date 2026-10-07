import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
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
import { WritingOriginReview } from './WritingOriginReview'

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
    sourceRefs: [
      category === 'owner_input' ? 'owner' : category === 'photo_interpretation' ? 'visual' : 'ai',
    ],
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
      span({ kind: 'block_item', blockIndex: 3, itemIndex: 1 }, '따뜻한 차'),
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
    ...patch,
  }
}

function Review({
  ownerId = 'alice',
  value = post(),
  displayed = value.content!,
  pending = false,
}: {
  ownerId?: string
  value?: PostDraft
  displayed?: PostContent
  pending?: boolean
}) {
  return (
    <WritingOriginReview ownerId={ownerId} post={value} content={displayed} pending={pending}>
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
  it('shows the named toggle by default and exactly three categories with a separate unconfirmed explanation', () => {
    render(<Review />)
    expect(screen.getByRole('checkbox', { name: '출처 보기' })).toBeChecked()
    const legend = screen.getByRole('list', { name: '의미의 출처' })
    expect(
      within(legend)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual(['직접 입력 기반', '사진에서 추론', 'AI가 보탠 내용'])
    expect(screen.getByText(/^출처 미확인:/)).not.toHaveAttribute('role', 'listitem')
    expect(phrase('주인 말', '직접 입력 기반')).toHaveClass('text-origin-owner-foreground')
    expect(phrase('사진 관찰', '사진에서 추론')).toHaveClass('text-origin-visual-foreground')
    expect(phrase('AI 제안', 'AI가 보탠 내용')).toHaveClass('text-origin-ai-foreground')
  })

  it('opens the frozen owner input, actual visual observation and AI basis without using current memo', async () => {
    const user = userEvent.setup()
    render(<Review />)
    for (const [text, category, source] of [
      ['주인 말', '직접 입력 기반', '그날 제주에 가서 천천히 걸었다.'],
      ['사진 관찰', '사진에서 추론', '이 결과를 만들 때 관찰한 빨간 지붕과 작은 항구.'],
      ['AI 제안', 'AI가 보탠 내용', '문단의 연결을 위해 덧붙인 해석.'],
    ]) {
      await user.click(phrase(text!, category!))
      const dialog = screen.getByRole('dialog', { name: '문구의 출처' })
      expect(within(dialog).getByText(source!)).toHaveClass('text-base', 'leading-relaxed')
      expect(within(dialog).queryByText('현재 메모는 변경되었다.')).not.toBeInTheDocument()
      expect(dialog.textContent).not.toContain('signature=')
      if (text === '사진 관찰') expect(within(dialog).getByText('IMG_1.jpg')).toBeInTheDocument()
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
      ['따뜻한 차', '직접 입력 기반'],
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
    expect(screen.getByText(/^제공된 입력이나 시각 관찰 외에/)).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })
})
