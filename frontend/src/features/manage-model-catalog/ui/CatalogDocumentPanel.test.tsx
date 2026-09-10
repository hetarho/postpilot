import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'

const MASTER = { id: 'root', plan: ProtoPlan.MASTER }

/** Two writing models registered, one vision model that is not — enough for a document to add
 *  one, drop one, and fail a gate. */
const CATALOG = [
  {
    modelId: 'anthropic/claude-x',
    label: 'Claude X',
    vision: true,
    curated: true,
    purposes: ['writing'],
  },
  { modelId: 'x-ai/grok-x', label: 'Grok X', vision: true, curated: true, purposes: ['writing'] },
  { modelId: 'z-ai/glm-x', label: 'GLM X', vision: false },
]

async function openPanel(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByRole('heading', { name: '모델 관리' })
  await user.click(screen.getByRole('button', { name: '일괄 편집' }))
  return screen.findByRole('heading', { name: '일괄 편집' })
}

function paste() {
  return screen.getByLabelText('붙여넣기')
}

describe('the 일괄 편집 document panel', () => {
  // MODEL-56: one entry for all five tabs, because one document names any purpose.
  it('opens from one entry shared by the tabs and shows the current document', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', { user: MASTER, modelCatalog: { entries: CATALOG } })

    await openPanel(user)
    // The export is a complete statement: every purpose, including the empty ones.
    const current = await screen.findByText(/# postpilot models v1/)
    expect(current.textContent).toContain('[writing]')
    expect(current.textContent).toContain('anthropic/claude-x')
    expect(current.textContent).toContain('[image-generation]')
  })

  // MODEL-52 + MODEL-54: the diff separates what would be added from what the omission drops,
  // and 확정 stays unavailable until a preview says the document is clean.
  it('previews adds and removes, and only then allows 확정', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/models', { user: MASTER, calls, modelCatalog: { entries: CATALOG } })

    await openPanel(user)
    expect(screen.getByRole('button', { name: '확정' })).toBeDisabled()

    await user.click(paste())
    await user.paste('# postpilot models v1\n[writing]\nanthropic/claude-x\nz-ai/glm-x\n')
    // Pasting alone commits nothing: the preview is a separate press.
    expect(calls).not.toContain('PreviewCatalogDocument')

    await user.click(screen.getByRole('button', { name: '미리보기' }))
    await screen.findByText('적용하면 이렇게 바뀌어요')
    expect(screen.getByText('등록 1개')).toBeInTheDocument()
    expect(screen.getByText('해제 1개')).toBeInTheDocument()
    // The omitted registration is named in the diff, not merely counted — scoped to the
    // deregister block, since the catalog behind the sheet lists the same id.
    const dropped = screen.getByText('해제 1개').closest('div')
    expect(within(dropped!).getByText('x-ai/grok-x')).toBeInTheDocument()
    // A purpose the document never named is called out as untouched.
    expect(screen.getByText(/문서에 없는 용도는 그대로예요/)).toBeInTheDocument()

    const confirm = screen.getByRole('button', { name: '확정' })
    await waitFor(() => expect(confirm).toBeEnabled())
    await user.click(confirm)
    await screen.findByText('반영했어요. 등록 1개, 해제 1개.')
    expect(calls.some((call) => call.startsWith('ApplyCatalogDocument'))).toBe(true)
  })

  // MODEL-53: any refused line refuses the whole document, and 확정 must not be reachable.
  it('renders every rejected line with its number and keeps 확정 unavailable', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', { user: MASTER, modelCatalog: { entries: CATALOG } })

    await openPanel(user)
    await user.click(paste())
    await user.paste(
      '# postpilot models v1\n[photo-analysis]\nz-ai/glm-x\n[writing]\n| pasted | row |\n',
    )
    await user.click(screen.getByRole('button', { name: '미리보기' }))

    const rejected = await screen.findByRole('alert')
    expect(rejected).toHaveTextContent('2줄을 읽지 못해서')
    const rows = screen.getAllByRole('listitem')
    const gate = rows.find((row) => within(row).queryByText('3번째 줄'))
    expect(gate).toBeDefined()
    expect(gate!).toHaveTextContent('이 용도에 필요한 기능이 없는 모델이에요.')
    const malformed = rows.find((row) => within(row).queryByText('5번째 줄'))
    expect(malformed!).toHaveTextContent('한 줄에 모델 아이디 하나')
    expect(screen.getByRole('button', { name: '확정' })).toBeDisabled()
    // No diff is offered beside a refusal — there is nothing that would be applied.
    expect(screen.queryByText('적용하면 이렇게 바뀌어요')).not.toBeInTheDocument()
  })

  // MODEL-59: a re-grade is a change the operator has to be able to see and commit — a
  // curator's list pasted with no grades clears every one of them.
  it('renders level changes as their own group and lets a relevel-only document apply', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/models', {
      user: MASTER,
      calls,
      modelCatalog: {
        entries: [
          {
            modelId: 'anthropic/claude-x',
            label: 'Claude X',
            vision: true,
            curated: true,
            purposes: ['writing'],
            level: { writing: 'top' },
          },
          {
            modelId: 'x-ai/grok-x',
            label: 'Grok X',
            vision: true,
            curated: true,
            purposes: ['writing'],
          },
        ],
      },
    })

    await openPanel(user)
    await user.click(paste())
    // Neither membership changes: claude loses its grade, grok gains one.
    await user.paste('# postpilot models v1\n[writing]\nanthropic/claude-x\nx-ai/grok-x value\n')
    await user.click(screen.getByRole('button', { name: '미리보기' }))

    await screen.findByText('적용하면 이렇게 바뀌어요')
    // The id and its two grades are one row; the id sits in its own span, so the assertion
    // is over the row that holds both.
    const regraded = screen.getByText('등급 변경 2개').closest('div')
    const rows = within(regraded!).getAllByRole('listitem')
    expect(rows.map((row) => row.textContent)).toEqual([
      'anthropic/claude-x · 최고 → 미지정',
      'x-ai/grok-x · 미지정 → 가성비',
    ])
    // Nothing is registered or deregistered, and it is still committable.
    expect(screen.queryByText(/^해제 /)).not.toBeInTheDocument()
    expect(screen.queryByText(/^등록 /)).not.toBeInTheDocument()

    const confirm = screen.getByRole('button', { name: '확정' })
    await waitFor(() => expect(confirm).toBeEnabled())
    await user.click(confirm)
    expect(calls.some((call) => call.startsWith('ApplyCatalogDocument'))).toBe(true)
  })

  // A bad grade is its own cause, so the operator is told which half of the line to fix.
  it('rejects an unknown level with its own copy and keeps 확정 unavailable', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', { user: MASTER, modelCatalog: { entries: CATALOG } })

    await openPanel(user)
    await user.click(paste())
    await user.paste('# postpilot models v1\n[writing]\nanthropic/claude-x legendary\n')
    await user.click(screen.getByRole('button', { name: '미리보기' }))

    const rows = await screen.findAllByRole('listitem')
    const bad = rows.find((row) => within(row).queryByText('3번째 줄'))
    expect(bad!).toHaveTextContent('등급 값이 잘못됐어요')
    // Never another cause's copy.
    expect(bad!).not.toHaveTextContent('한 줄에 모델 아이디 하나')
    expect(screen.getByRole('button', { name: '확정' })).toBeDisabled()
  })

  // Editing after a preview invalidates it: 확정 must never commit a diff computed for text
  // the operator has since changed.
  it('keeps the pasted text but drops the diff when the document is edited', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', { user: MASTER, modelCatalog: { entries: CATALOG } })

    await openPanel(user)
    await user.click(paste())
    await user.paste('# postpilot models v1\n[writing]\nanthropic/claude-x\nz-ai/glm-x\n')
    await user.click(screen.getByRole('button', { name: '미리보기' }))
    await screen.findByText('적용하면 이렇게 바뀌어요')

    await user.type(paste(), 'x')
    expect(screen.queryByText('적용하면 이렇게 바뀌어요')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '확정' })).toBeDisabled()
    // The text survives so one bad line can be fixed without pasting the whole list again.
    expect(paste()).toHaveValue(
      '# postpilot models v1\n[writing]\nanthropic/claude-x\nz-ai/glm-x\nx',
    )
  })

  // The document path needs the live snapshot to create a row for an uncurated id, so an
  // unreadable catalog refuses instead of degrading.
  it('refuses when the provider catalog cannot be read', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: { entries: CATALOG, fetchFails: true },
    })

    await screen.findByRole('heading', { name: '모델 관리' })
    await user.click(screen.getByRole('button', { name: '일괄 편집' }))
    await screen.findByRole('heading', { name: '일괄 편집' })
    await user.click(paste())
    await user.paste('# postpilot models v1\n[writing]\nanthropic/claude-x\n')
    await user.click(screen.getByRole('button', { name: '미리보기' }))

    await waitFor(() =>
      expect(
        screen.getByText(/제공사의 모델 목록을 읽지 못해서 아무것도 반영하지 않았어요/),
      ).toBeInTheDocument(),
    )
    expect(screen.getByRole('button', { name: '확정' })).toBeDisabled()
  })
})
