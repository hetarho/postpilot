import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import type { PostDraft, PostStorylineParagraph } from '@/entities/post'
import { StorylineSpace } from './StorylineSpace'

afterEach(cleanup)

const PARAGRAPHS: PostStorylineParagraph[] = [
  { text: '가게 앞을 보여줍니다.', files: ['a.jpg', 'clip.mp4'] },
  { text: '커피를 이야기합니다.', files: ['b.jpg'] },
]

function post(overrides: Partial<NonNullable<PostDraft['storyline']>> = {}) {
  return {
    storyline: {
      paragraphs: PARAGRAPHS,
      editedByHand: false,
      addedFiles: [],
      takenOutFiles: ['c.jpg'],
      ...overrides,
    },
    images: ['a.jpg', 'b.jpg', 'c.jpg', 'new.jpg'].map((filename, index) => ({
      id: `img-${index}`,
      filename,
      width: 1024,
      height: 768,
      bytes: 1,
      viewUrl: `blob:${filename}`,
    })),
    videos: [
      {
        id: 'vid',
        filename: 'clip.mp4',
        width: 1280,
        height: 720,
        bytes: 1,
        durationMs: 3000,
        contentType: 'video/mp4',
        viewUrl: '',
      },
    ],
  } as unknown as Pick<PostDraft, 'storyline' | 'images' | 'videos'>
}

/** The space over state it owns, the way the editor's autosave holds it. */
function Harness({
  hasContent = false,
  readOnly = false,
  onChange,
  source = post(),
}: {
  hasContent?: boolean
  readOnly?: boolean
  onChange?: (paragraphs: PostStorylineParagraph[]) => void
  source?: ReturnType<typeof post>
}) {
  const [paragraphs, setParagraphs] = useState(PARAGRAPHS)
  return (
    <StorylineSpace
      post={source}
      paragraphs={paragraphs}
      readOnly={readOnly}
      hasContent={hasContent}
      onChange={(next) => {
        setParagraphs(next)
        onChange?.(next)
      }}
    />
  )
}

const paragraph = (n: number) => within(screen.getByRole('listitem', { name: `${n}번째 문단` }))

describe('StorylineSpace', () => {
  it('opens while the post has no content, and closes once content first shows', () => {
    const { rerender } = render(<Harness />)
    expect(screen.getByRole('button', { name: '스토리라인' })).toHaveAttribute(
      'aria-expanded',
      'true',
    )
    expect(paragraph(1).getByText('가게 앞을 보여줍니다.')).toBeInTheDocument()

    rerender(<Harness hasContent />)
    expect(screen.getByRole('button', { name: '스토리라인' })).toHaveAttribute(
      'aria-expanded',
      'false',
    )
  })

  it('starts closed over a post that already has content, and opens on request', async () => {
    const user = userEvent.setup()
    render(<Harness hasContent />)
    const heading = screen.getByRole('button', { name: '스토리라인' })
    expect(heading).toHaveAttribute('aria-expanded', 'false')
    await user.click(heading)
    expect(paragraph(2).getByText('커피를 이야기합니다.')).toBeInTheDocument()
  })

  it('edits a paragraph’s text in place, handing the whole list back', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)
    await user.click(paragraph(2).getByRole('button', { name: '2번째 문단 고치기' }))
    const field = paragraph(2).getByRole('textbox', { name: '2번째 문단' })
    await user.clear(field)
    await user.type(field, '라떼')
    expect(onChange).toHaveBeenLastCalledWith([PARAGRAPHS[0], { text: '라떼', files: ['b.jpg'] }])
    await user.click(paragraph(2).getByRole('button', { name: '완료' }))
    expect(paragraph(2).getByText('라떼')).toBeInTheDocument()
  })

  it('moves a file to another paragraph and takes one out', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)
    await user.click(paragraph(1).getByRole('button', { name: 'a.jpg 옮기기' }))
    await user.click(screen.getByRole('menuitemradio', { name: '2번째 문단' }))
    expect(onChange).toHaveBeenLastCalledWith([
      { text: '가게 앞을 보여줍니다.', files: ['clip.mp4'] },
      { text: '커피를 이야기합니다.', files: ['b.jpg', 'a.jpg'] },
    ])

    await user.click(paragraph(2).getByRole('button', { name: 'b.jpg 옮기기' }))
    await user.click(screen.getByRole('menuitemradio', { name: '빼기' }))
    expect(onChange).toHaveBeenLastCalledWith([
      { text: '가게 앞을 보여줍니다.', files: ['clip.mp4'] },
      { text: '커피를 이야기합니다.', files: ['a.jpg'] },
    ])
    // Taken out, it waits under 빠진 사진 with the one taken out before.
    const takenOut = within(screen.getByRole('region', { name: '빠진 사진' }))
    expect(takenOut.getByRole('button', { name: 'b.jpg 넣기' })).toBeInTheDocument()
    expect(takenOut.getByRole('button', { name: 'c.jpg 넣기' })).toBeInTheDocument()
  })

  it('puts a taken-out file back and never offers one added after the storyline', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Harness onChange={onChange} source={post({ addedFiles: ['new.jpg'] })} />)
    expect(
      screen.getByText(
        '이 스토리라인을 만든 뒤 사진이 추가됐어요. 다시 만들면 새 사진도 들어가요.',
      ),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'new.jpg 넣기' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'c.jpg 넣기' }))
    await user.click(screen.getByRole('menuitem', { name: '1번째 문단' }))
    expect(onChange).toHaveBeenLastCalledWith([
      { text: '가게 앞을 보여줍니다.', files: ['a.jpg', 'clip.mp4', 'c.jpg'] },
      PARAGRAPHS[1],
    ])
    expect(screen.queryByRole('region', { name: '빠진 사진' })).not.toBeInTheDocument()
  })

  it('shows a clip with no view URL by its name', () => {
    render(<Harness />)
    expect(paragraph(1).getByText('clip.mp4')).toBeInTheDocument()
  })

  it('offers no field and no move control while read-only', () => {
    render(<Harness readOnly />)
    expect(screen.queryByRole('button', { name: /고치기$/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /옮기기$/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /넣기$/ })).not.toBeInTheDocument()
    expect(screen.getByRole('region', { name: '빠진 사진' })).toBeInTheDocument()
  })
})
