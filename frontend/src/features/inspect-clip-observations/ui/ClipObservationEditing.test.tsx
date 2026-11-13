import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

/** Whether nothing up the tree hides the element (the collapsed ranges stay mounted). */
function isVisible(element: HTMLElement): boolean {
  return !element.closest('[hidden]')
}
import { observedClipFixture } from '@/test/clip-observations'
import type { ClipObservedSegment } from '@/entities/clip-observation'
import { ClipObservationViewer } from './ClipObservationViewer'
import { observationEditOf, type ObservationField } from '../model/observation-edits'
import type { ClipObservationEditing } from '../model/useClipObservationEditing'

/** A local stand-in for the workspace's editing handle: the same refusal rule, with saves
 *  recorded instead of queued. */
function Harness({
  editable = true,
  saved,
  project = observedClipFixture(),
}: {
  editable?: boolean
  saved: (edit: unknown) => void
  project?: ReturnType<typeof observedClipFixture>
}) {
  const [typed, setTyped] = useState<Record<string, string>>({})
  const [held, setHeld] = useState<Record<string, ObservationField>>({})
  const editing: ClipObservationEditing = {
    editable,
    failure: undefined,
    text: (id, field, segment: ClipObservedSegment) =>
      typed[`${id}:${field}`] ??
      (field === 'subjects' ? segment.subjects.join('\n') : segment[field]),
    change(id, field, segment, value) {
      setTyped((current) => ({ ...current, [`${id}:${field}`]: value }))
      const edit = observationEditOf(id, field, segment, value)
      setHeld((current) => {
        const rest = Object.fromEntries(Object.entries(current).filter(([key]) => key !== id))
        return edit ? rest : { ...rest, [id]: field }
      })
      if (edit) saved(edit)
    },
    refused: (id) => held[id],
  }
  return <ClipObservationViewer project={project} localSources={[]} editing={editing} />
}

describe('correcting observed scenes in place (CLIP-235)', () => {
  it('edits event, action, subjects one per line and speech, saving each change', async () => {
    const user = userEvent.setup()
    const saved = vi.fn()
    render(<Harness saved={saved} />)
    await user.click(screen.getByRole('button', { name: /자세히 보기/ }))
    const events = screen.getAllByRole('textbox', { name: '장면' })
    expect(events).toHaveLength(2)
    await user.clear(events[0]!)
    await user.type(events[0]!, '국수')
    expect(saved).toHaveBeenLastCalledWith({ observationId: 'a/0', event: '국수' })
    const subjects = screen.getAllByRole('textbox', { name: '대상 (한 줄에 하나씩)' })[0]!
    expect(subjects).toHaveValue('음식\n접시')
    await user.type(subjects, '\n젓가락')
    expect(saved).toHaveBeenLastCalledWith({
      observationId: 'a/0',
      subjects: ['음식', '접시', '젓가락'],
    })
    // Motion, quality and the classifications stay as observed, read only.
    expect(screen.queryByRole('textbox', { name: '카메라·화면 움직임' })).not.toBeInTheDocument()
    expect(screen.getByText('카메라가 천천히 다가간다')).toBeVisible()
  })

  it('refuses clearing observed speech with a field error and offers no speech on a silent scene', async () => {
    const user = userEvent.setup()
    const saved = vi.fn()
    render(<Harness saved={saved} />)
    await user.click(screen.getByRole('button', { name: /자세히 보기/ }))
    const speech = screen.getAllByRole('textbox', { name: '들리는 말' })
    // The silent second scene has no speech to correct.
    expect(speech).toHaveLength(1)
    await user.clear(speech[0]!)
    expect(speech[0]).toHaveAttribute('aria-invalid', 'true')
    expect(speech[0]).toHaveAccessibleDescription(
      '들리는 말은 비울 수 없어요. 고칠 내용을 입력해 주세요.',
    )
    expect(saved).not.toHaveBeenCalled()
    await user.type(speech[0]!, '맛있다')
    expect(speech[0]).not.toHaveAttribute('aria-invalid')
    expect(saved).toHaveBeenLastCalledWith({ observationId: 'a/0', speech: '맛있다' })
  })

  it('marks a scene and a source the owner corrected', async () => {
    const user = userEvent.setup()
    const project = observedClipFixture()
    project.observations!.sources[0]!.ownerEdited = true
    project.observations!.sources[0]!.segments[0]!.ownerEdited = true
    render(<Harness saved={vi.fn()} project={project} editable={false} />)
    const visible = () => screen.getAllByText('직접 고침').filter((mark) => isVisible(mark))
    expect(visible()).toHaveLength(1)
    await user.click(screen.getByRole('button', { name: /자세히 보기/ }))
    expect(visible()).toHaveLength(2)
  })

  it('stays read-only while a job runs or the project is finalized', async () => {
    const user = userEvent.setup()
    render(<Harness saved={vi.fn()} editable={false} />)
    await user.click(screen.getByRole('button', { name: /자세히 보기/ }))
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.getByText('맛있어요')).toBeVisible()
    // The saved edit plan's usage stays visible beside the corrections (CLIP-49).
    expect(screen.getByText('1번 컷에 사용 · 1× · 원본 0:03.500–0:10')).toBeVisible()
  })
})
