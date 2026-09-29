import { describe, expect, it } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { VoiceSample } from '@/entities/voice'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { SampleList } from './SampleList'

const SAMPLES: VoiceSample[] = [
  {
    id: 'answer-1',
    kind: 'answer',
    label: '',
    promptKey: 'photo_food',
    hasPhoto: true,
    chars: 12,
    createdAt: '2026-08-29T12:00:00Z',
  },
  {
    id: 'post-1',
    kind: 'post',
    label: '제주',
    promptKey: '',
    hasPhoto: false,
    chars: 240,
    createdAt: '2026-08-28T12:00:00Z',
  },
]

function renderList(calls: string[]) {
  const transport = createFakeAuthTransport({
    calls,
    voice: {
      samples: SAMPLES.map((sample) => ({
        ...sample,
        body: sample.kind === 'answer' ? '짜장면이 맛있었어요.' : '제주에 다녀왔어요.',
      })),
    },
  })
  return render(<SampleList ownerId="alice" voiceId="voice-default" samples={SAMPLES} />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
}

// VOICE-64: a post by its label, an answer by its prompt, each opening to its full text and
// photo with 삭제.
describe('the 학습 글 list', () => {
  it('names a post by its label and an answer by its prompt', async () => {
    renderList([])
    const rows = screen.getAllByRole('listitem')
    expect(rows[1]).toHaveTextContent('제주')
    expect(rows[1]).toHaveTextContent('붙여 넣은 글')
    expect(await within(rows[0]!).findByText(/음식이나 음료 사진 한 장을 골라/)).toBeInTheDocument()
    expect(rows[0]).toHaveTextContent('문항 답')
    expect(rows[0]).toHaveTextContent('사진')
  })

  it('opens an answer to its text and photo and deletes it after confirmation', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderList(calls)

    await user.click((await screen.findAllByRole('button'))[0]!)
    const sheet = await screen.findByRole('dialog')
    expect(await within(sheet).findByText('짜장면이 맛있었어요.')).toBeInTheDocument()
    expect(within(sheet).getByRole('img', { name: '답에 쓴 사진' })).toHaveAttribute(
      'src',
      'https://storage.test/voices/answer-1.jpg',
    )
    await user.click(within(sheet).getByRole('button', { name: '삭제' }))
    const confirm = await screen.findByRole('dialog', { name: '학습 글을 삭제할까요?' })
    await user.click(within(confirm).getByRole('button', { name: '삭제' }))

    await waitFor(() => expect(calls).toContain('DeleteVoiceSample'))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(calls.filter((call) => call.startsWith('Analyze') || call.startsWith('Start'))).toEqual(
      [],
    )
  })
})
