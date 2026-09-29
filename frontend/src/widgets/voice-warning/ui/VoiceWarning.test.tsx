import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { renderAppAt } from '@/test/app'

const UNMADE_REASON = '아직 만들지 않은 말투예요. 말투를 만들거나 다른 말투로 바꿔 주세요.'
const UNMADE = { id: 'voice-cafe', name: '가게 소개', made: false }

// POST-25: the editor's voice warning is for a voice that refuses AI work — deleted, or not yet
// made — and for nothing else. An empty profile is no longer a caveat of its own: a voice is
// either made or listed as 만드는 중.
describe('the editor voice warning', () => {
  it('warns about a voice not yet made and links to that voice', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: { id: 'alice' },
      posts: { posts: [{ slug: '20260820-jeju', voice: UNMADE }] },
      voice: { voices: [{ id: 'voice-default', name: '기본 말투', isDefault: true }, UNMADE] },
    })

    expect(await screen.findByText(UNMADE_REASON)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '말투 학습하기' })).toHaveAttribute(
      'href',
      '/voices/voice-cafe',
    )
  })

  it('says nothing for a made voice, even one whose profile is empty', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: { id: 'alice' },
      posts: { posts: [{ slug: '20260820-jeju' }] },
    })

    await screen.findByLabelText('제목')
    expect(screen.queryByText(UNMADE_REASON)).not.toBeInTheDocument()
    expect(screen.queryByText(/문체 프로필이 비어 있어요/)).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '말투 학습하기' })).not.toBeInTheDocument()
  })

  it('says nothing for a post with 말투 없음', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: { id: 'alice' },
      posts: { posts: [{ slug: '20260820-jeju', voice: null }] },
    })

    await screen.findByLabelText('제목')
    expect(screen.queryByText(UNMADE_REASON)).not.toBeInTheDocument()
    expect(screen.queryByText(/삭제된 말투/)).not.toBeInTheDocument()
  })

  it('says nothing on a new draft', async () => {
    renderAppAt('/posts/new', { user: { id: 'alice' } })

    await screen.findByLabelText('제목')
    expect(screen.queryByText(/문체 프로필이 비어 있어요/)).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '말투 학습하기' })).not.toBeInTheDocument()
  })
})
