import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { create } from '@bufbuild/protobuf'
import { describe, expect, it, vi } from 'vitest'
import type { PostImage } from '@/entities/image'
import { ObservationSchema } from '@/shared/api'
import { ReobservePicker } from './ReobservePicker'

const image = (filename: string, viewUrl = `https://r2.test/${filename}?sig=1`): PostImage => ({
  id: filename,
  filename,
  width: 800,
  height: 600,
  bytes: 1000,
  viewUrl,
})

const observation = (filename: string, fields: { scene?: string; model?: string } = {}) =>
  create(ObservationSchema, {
    file: filename,
    scene: fields.scene ?? `${filename}의 장면`,
    model: fields.model ?? 'openrouter/observer-1',
  })

const IMAGES = [image('IMG_1.jpg'), image('IMG_2.jpg')]
const OBSERVATIONS = [observation('IMG_1.jpg'), observation('IMG_2.jpg')]

function renderPicker(
  props: Partial<Parameters<typeof ReobservePicker>[0]> = {},
  onConfirm = vi.fn(),
) {
  const view = render(
    <ReobservePicker
      open
      images={IMAGES}
      observations={OBSERVATIONS}
      onConfirm={onConfirm}
      onCancel={vi.fn()}
      {...props}
    />,
  )
  return { ...view, onConfirm }
}

describe('ReobservePicker', () => {
  it('is one modal surface listing every photo with its stored observation', () => {
    renderPicker()
    expect(screen.getByRole('dialog', { name: '다시 관찰할 사진 선택' })).toBeInTheDocument()
    expect(screen.getAllByRole('checkbox')).toHaveLength(2)
    expect(screen.getByText('IMG_1.jpg의 장면')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'IMG_1.jpg 관찰 사진' })).toHaveAttribute(
      'src',
      'https://r2.test/IMG_1.jpg?sig=1',
    )
  })

  // A1: ONE presentation switched in CSS by `Sheet` — a bottom sheet on a phone, a centred
  // dialog from `md:` up. Two mounted overlays would fight over the body scroll lock.
  it('renders the phone sheet shape and the desktop dialog shape from the same mount', () => {
    renderPicker()
    const panel = screen.getByRole('dialog')
    expect(panel).toHaveClass('rounded-t-xl', 'md:rounded-xl', 'max-h-sheet')
    expect(panel.parentElement).toHaveClass('items-end', 'md:items-center')
  })

  // A2: every checkbox clear, and confirming untouched sends an EMPTY set — not `undefined`,
  // which would mean "observe everything".
  it('starts with every checkbox clear and confirms an empty set', async () => {
    const { onConfirm } = renderPicker()
    for (const box of screen.getAllByRole('checkbox')) expect(box).not.toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: '이대로 시작' }))
    expect(onConfirm).toHaveBeenCalledWith([])
  })

  it('confirms the checked filenames in post order', async () => {
    const { onConfirm } = renderPicker()
    await userEvent.click(screen.getByRole('checkbox', { name: 'IMG_2.jpg 다시 관찰' }))
    await userEvent.click(screen.getByRole('checkbox', { name: 'IMG_1.jpg 다시 관찰' }))
    await userEvent.click(screen.getByRole('button', { name: '이대로 시작' }))
    expect(onConfirm).toHaveBeenCalledWith(['IMG_1.jpg', 'IMG_2.jpg'])
  })

  it('selects and clears every photo at once, and clearing keeps the forced ones', async () => {
    const { onConfirm } = renderPicker({
      images: [...IMAGES, image('NEW.jpg')],
      observations: OBSERVATIONS,
    })
    await userEvent.click(screen.getByRole('button', { name: '전체 선택' }))
    await userEvent.click(screen.getByRole('button', { name: '전체 해제' }))
    await userEvent.click(screen.getByRole('button', { name: '이대로 시작' }))
    expect(onConfirm).toHaveBeenCalledWith(['NEW.jpg'])
  })

  // A5: a photo with nothing to reuse is checked, cannot be cleared, and says why — and a
  // picker whose only forced rows are those still confirms.
  it('forces a photo with nothing to reuse and states the reason', async () => {
    const { onConfirm } = renderPicker({
      images: [image('IMG_1.jpg'), image('NEW.jpg')],
      observations: [observation('IMG_1.jpg')],
    })
    const forced = screen.getByRole('checkbox', { name: 'NEW.jpg 다시 관찰' })
    expect(forced).toBeChecked()
    expect(forced).toBeDisabled()
    expect(screen.getByText('관찰 결과가 없어 반드시 관찰합니다')).toBeInTheDocument()
    expect(screen.getByText('저장된 관찰 결과 없음')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '이대로 시작' }))
    expect(onConfirm).toHaveBeenCalledWith(['NEW.jpg'])
  })

  it('forces a photo whose stored entry carries no eyesight', () => {
    renderPicker({
      images: [image('IMG_1.jpg')],
      observations: [observation('IMG_1.jpg', { scene: '' })],
    })
    expect(screen.getByRole('checkbox', { name: 'IMG_1.jpg 다시 관찰' })).toBeDisabled()
  })

  // A photo attached since the last observation still carries its local upload preview. Only
  // the presigned GetPost capability may fetch an R2 thumbnail, so no `blob:` URL is used.
  it('does not fetch a blob preview as a thumbnail', () => {
    renderPicker({
      images: [image('NEW.jpg', 'blob:http://localhost/abc')],
      observations: [],
    })
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('names the model that produced the stored observations', () => {
    renderPicker()
    expect(screen.getByText('저장된 관찰: openrouter/observer-1')).toBeInTheDocument()
  })

  it('reports an entry written before provenance existed as unrecorded', () => {
    renderPicker({ observations: [observation('IMG_1.jpg', { model: '' })], images: [IMAGES[0]] })
    expect(screen.getByText('저장된 관찰: 기록되지 않은 모델')).toBeInTheDocument()
  })

  // A6: the notice names BOTH models and the defaults stay clear — mixing is the user's call.
  it('warns when the selected observation model differs, without checking anything', () => {
    renderPicker({ observeModel: { providerId: 'openrouter', modelId: 'observer-2' } })
    expect(
      screen.getByText(/openrouter\/observer-2.*openrouter\/observer-1/, { exact: false }),
    ).toBeInTheDocument()
    for (const box of screen.getAllByRole('checkbox')) expect(box).not.toBeChecked()
  })

  it('does not warn when the selected model is the one that observed', () => {
    renderPicker({ observeModel: { providerId: 'openrouter', modelId: 'observer-1' } })
    expect(screen.queryByText(/달라요/)).not.toBeInTheDocument()
  })

  // Unknown provenance says nothing about WHICH model observed, so it cannot be a difference.
  // Every observation written before the field existed reads as unknown, so treating it as a
  // mismatch would fire this warning on every pre-existing post.
  it('does not warn when the stored provenance is unknown', () => {
    renderPicker({
      images: [IMAGES[0]],
      observations: [observation('IMG_1.jpg', { model: '' })],
      observeModel: { providerId: 'openrouter', modelId: 'observer-2' },
    })
    expect(screen.queryByText(/달라요/)).not.toBeInTheDocument()
  })

  it('reopens with the defaults rather than the previous answer', async () => {
    function Harness() {
      const [open, setOpen] = useState(true)
      return (
        <>
          <button onClick={() => setOpen((value) => !value)}>toggle</button>
          <ReobservePicker
            open={open}
            images={IMAGES}
            observations={OBSERVATIONS}
            onConfirm={() => setOpen(false)}
            onCancel={() => setOpen(false)}
          />
        </>
      )
    }
    render(<Harness />)
    await userEvent.click(screen.getByRole('checkbox', { name: 'IMG_1.jpg 다시 관찰' }))
    expect(screen.getByRole('checkbox', { name: 'IMG_1.jpg 다시 관찰' })).toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'toggle' }))
    await userEvent.click(screen.getByRole('button', { name: 'toggle' }))
    expect(screen.getByRole('checkbox', { name: 'IMG_1.jpg 다시 관찰' })).not.toBeChecked()
  })

  // The checkboxes, the count and the confirmed set are one answer. A forced row that appears
  // while the picker is open — an upload confirming behind it — must not render checked while the
  // count and the confirm leave it out.
  it('keeps a forced row that appears while it is open inside the count and the confirmed set', async () => {
    const onConfirm = vi.fn()
    const view = render(
      <ReobservePicker
        open
        images={[IMAGES[0]]}
        observations={[OBSERVATIONS[0]]}
        onConfirm={onConfirm}
        onCancel={vi.fn()}
      />,
    )
    expect(screen.getByRole('status')).toHaveTextContent('다시 관찰할 사진 없음')

    view.rerender(
      <ReobservePicker
        open
        images={[IMAGES[0], image('LATE.jpg')]}
        observations={[OBSERVATIONS[0]]}
        onConfirm={onConfirm}
        onCancel={vi.fn()}
      />,
    )
    const late = screen.getByRole('checkbox', { name: 'LATE.jpg 다시 관찰' })
    expect(late).toBeChecked()
    expect(late).toBeDisabled()
    expect(screen.getByRole('status')).toHaveTextContent('1장 다시 관찰')
    await userEvent.click(screen.getByRole('button', { name: '이대로 시작' }))
    expect(onConfirm).toHaveBeenCalledWith(['LATE.jpg'])
  })

  it('reports how many photos will be observed again', async () => {
    renderPicker()
    expect(screen.getByRole('status')).toHaveTextContent('다시 관찰할 사진 없음')
    await userEvent.click(screen.getByRole('checkbox', { name: 'IMG_1.jpg 다시 관찰' }))
    expect(screen.getByRole('status')).toHaveTextContent('1장 다시 관찰')
  })
})
