import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { VoiceSample } from '@/entities/voice'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { LearningMaterials } from './LearningMaterials'

// VOICE-78: the setup's collected materials name a thread answer's experience.
describe('the collected learning materials', () => {
  it('names the experience of a thread answer, never the raw token', async () => {
    const sample: VoiceSample = {
      id: 'thread-1',
      kind: 'answer',
      label: '',
      promptKey: 'thread_food_open',
      experienceId: 'experience-1',
      hasPhoto: false,
      chars: 20,
      createdAt: '2026-08-30T12:00:00Z',
    }
    const transport = createFakeAuthTransport({
      voice: {
        prompts: [
          {
            key: 'thread_food_open',
            part: 'opening',
            photo: false,
            text: '{experience}, 어쩌다 가게 됐어요?',
            experienceKind: 'food',
          },
        ],
        experiences: [{ id: 'experience-1', kind: 1, name: '회사 앞 칼국숫집' }],
      },
    })
    render(<LearningMaterials ownerId="alice" voiceId="voice-default" samples={[sample]} />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    expect(screen.queryByText(/\{experience\}/)).not.toBeInTheDocument()
    expect(await screen.findByText('회사 앞 칼국숫집, 어쩌다 가게 됐어요?')).toBeInTheDocument()
  })
})
