import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { openConversationDirect, saveConversationDraft } from '@/test/authoring-ui'
import type { ClipRecipe } from '@/entities/clip-template'
import type { FakeClipsOptions } from '@/test/clips'
import type { FakeAuthoringOptions } from '@/test/authoring'
const body =
  '<clip version=\'1\'>\n<group id="menu" max="3"><field id="name" label="메뉴 이름"/></group>\n<stage name="외관">  keep &amp; spacing  </stage>\n<text id="intro" kind="fixed" role="hook"/><text id="outro" kind="fixed" role="ending"/></clip>'
const template = { id: 'owned', name: '장면 템플릿', compositionBody: body }
async function mount(
  user: ReturnType<typeof userEvent.setup>,
  clips: FakeClipsOptions = {},
  authoring: FakeAuthoringOptions = {},
) {
  const view = renderAppAt('/video-templates/owned', {
    user: { id: 'alice' },
    clips: { templates: [template], ...clips },
    authoring,
  })
  await openConversationDirect(user)
  await screen.findByLabelText('템플릿 이름')
  return view
}
describe('durable clip template composition editing', () => {
  it('edits the composition as text only: no block builder, nothing to drag or reorder', async () => {
    const user = userEvent.setup()
    await mount(user)
    expect(screen.getByRole('textbox', { name: '원문' })).toHaveValue(body)
    expect(screen.queryByRole('tab', { name: '구성 편집' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '고급' })).not.toBeInTheDocument()
    expect(screen.queryAllByRole('button', { name: /위로|아래로|끌어서|옮기기/ })).toEqual([])
  })
  it('publishes the typed text exactly, keeping untouched source spans', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    await mount(user, { writes })
    const changed = body.replace('메뉴 이름', '메뉴와 음료')
    fireEvent.change(screen.getByRole('textbox', { name: '원문' }), {
      target: { value: changed },
    })
    await saveConversationDraft(user)
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].compositionBody).toBe(changed)
    expect(writes[0].compositionBody).toContain('<stage name="외관">  keep &amp; spacing  </stage>')
  })
  it('keeps exact raw bytes when only the name changes and performs no provider work', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    const calls: string[] = []
    await mount(user, { writes, calls })
    await user.type(screen.getByLabelText('템플릿 이름'), ' 새 이름')
    await saveConversationDraft(user)
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].compositionBody).toBe(body)
    expect(calls).not.toContain('StartClipGeneration')
  })
  it('keeps an unfinished composition as typed, with saving unavailable', async () => {
    const user = userEvent.setup()
    const patches: NonNullable<FakeAuthoringOptions['patches']> = []
    await mount(user, {}, { patches })
    fireEvent.change(screen.getByRole('textbox', { name: '원문' }), {
      target: { value: '<clip version="1"><stage' },
    })
    await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
    await waitFor(() => expect(patches[0]?.body).toBe('<clip version="1"><stage'))
    expect(screen.getByRole('button', { name: '변경 저장' })).toBeDisabled()
  })
})
