import { useReducer } from 'react'
import userEvent from '@testing-library/user-event'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { clipTimelineReducer, createClipTimeline, type ClipEditPlan } from '@/entities/clip-plan'
import { clipNarrationFixture } from '@/test/clip-editing'
import { ClipSegmentSheet } from './ClipSegmentSheet'

afterEach(cleanup)

const narrated = (): ClipEditPlan => {
  const plan = clipNarrationFixture().plan
  plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 1000,
    segments: [
      {
        id: 'spoken-1',
        text: '처음 대본',
        textRevision: 1,
        inputHash: 'hash-1',
        startMs: 1000,
        endMs: 5000,
        speech: {
          assetId: 'asset',
          voiceId: 'voice',
          bindingDigest: 'binding',
          inputHash: 'hash-1',
          settingsHash: 'settings',
          audioHash: 'audio',
          profileId: 'profile',
          profileRevision: 1,
          samples: 48000,
          sampleRate: 24000,
          channels: 1,
          timing: [],
        },
      },
    ],
  }
  return plan
}

/** The real reducer, with the script editor ② hands the sheet as its body. */
function Harness({ readOnly = false }: { readOnly?: boolean }) {
  const [timeline, dispatch] = useReducer(clipTimelineReducer, narrated(), createClipTimeline)
  const segment = timeline.plan.narration!.segments[0]
  return (
    <>
      <button type="button" onClick={() => dispatch({ type: 'undo' })}>
        undo
      </button>
      <button type="button" onClick={() => dispatch({ type: 'redo' })}>
        redo
      </button>
      <ClipSegmentSheet
        open
        onClose={() => {}}
        narration={timeline.plan.narration!}
        segment={segment}
        durationMs={timeline.plan.durationMs}
      >
        {readOnly ? undefined : (
          <textarea
            aria-label="script"
            value={segment.text}
            onChange={(event) =>
              dispatch({
                type: 'edit',
                edit: { type: 'spokenPatch', id: segment.id, patch: { text: event.target.value } },
                at: Date.now(),
                group: `${segment.id}:words`,
              })
            }
          />
        )}
      </ClipSegmentSheet>
    </>
  )
}

it('opens a segment for its script and marks changed speech stale, which undo restores', async () => {
  render(<Harness />)
  const sheet = screen.getByRole('dialog', { name: '더빙 1' })
  expect(sheet).toHaveTextContent('준비됨')
  fireEvent.change(screen.getByLabelText('script'), { target: { value: '고친 대본' } })
  expect(sheet).toHaveTextContent('재생성 필요')
  // Undo and redo move between the two scripts; neither of them produces speech (DUB-20).
  await userEvent.click(screen.getByRole('button', { name: 'undo' }))
  expect(screen.getByLabelText('script')).toHaveValue('처음 대본')
  expect(sheet).toHaveTextContent('준비됨')
  await userEvent.click(screen.getByRole('button', { name: 'redo' }))
  expect(screen.getByLabelText('script')).toHaveValue('고친 대본')
  expect(sheet).toHaveTextContent('재생성 필요')
})

it('states the script as a value when the plan is only read', () => {
  render(<Harness readOnly />)
  const sheet = screen.getByRole('dialog', { name: '더빙 1' })
  expect(sheet).toHaveTextContent('처음 대본')
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
})
