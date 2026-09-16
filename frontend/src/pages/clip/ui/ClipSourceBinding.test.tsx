import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ClipSourceBatchSchema } from '@/shared/api/gen/postpilot/v1/clip_pb'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { ClipProjectDraft } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'

const body =
  '<clip version="1" intro="b" caption="bold" outro="e"><group id="menu" label="고기"><field id="name" label="부위" required="true"/></group><repeat for="menu"><scene id="cut" scope="item"><text id="copy" kind="ai" role="caption" basis="cut">설명</text></scene></repeat><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>'
const inputs = {
  values: {},
  items: {
    menu: [
      { id: 'belly', values: { name: '삼겹살' } },
      { id: 'neck', values: { name: '목살' } },
    ],
  },
  associations: [],
}
const project = {
  id: 'owned',
  title: '고기',
  videoTemplateId: 'meat',
  ratio: 'vertical' as const,
  targetDurationMs: 30000,
  answers: [],
  disclosure: '' as const,
  cta: '' as const,
  composition: { snapshot: { version: 1, body, templateId: 'meat', legacy: false }, inputs },
  compositionInputs: inputs,
}
const template = {
  id: 'meat',
  name: '고기',
  compositionBody: body,
  compositionLegacy: false,
  informationFields: [],
  cutGuidance: '',
  accent: '' as const,
  preset: '' as const,
}
const batch = (
  sources: { id: string; filename: string; durationMs: number; fingerprint: string }[],
) =>
  create(ClipSourceBatchSchema, {
    id: 'retained',
    projectId: 'owned',
    current: true,
    state: 'ready',
    expiresAt: '2099-01-01T00:00:00Z',
    sources: sources.map((s) => ({
      id: s.id,
      state: 'ready',
      availability: 'available',
      retentionExpiresAt: '2099-01-01T00:00:00Z',
      metadata: {
        filename: s.filename,
        durationMs: s.durationMs,
        fingerprint: s.fingerprint,
        contentType: 'video/mp4',
        bytes: 5n,
      },
    })),
  })
afterEach(() => discardClipDraftQueues())

describe('binding a source to an item before generation', () => {
  it('offers the declared items per source and saves the whole-source range', async () => {
    const user = userEvent.setup(),
      writes: ClipProjectDraft[] = []
    renderAppAt('/clips/owned', {
      user: { id: 'alice' },
      clips: {
        templates: [template],
        projects: [project],
        projectWrites: writes,
        retainedBatches: [
          batch([
            { id: 'src-1', filename: '고기1.mp4', durationMs: 20000, fingerprint: 'fp-1' },
            { id: 'src-2', filename: '고기2.mp4', durationMs: 30000, fingerprint: 'fp-2' },
          ]),
        ],
      },
    })
    const first = await screen.findByLabelText('고기1.mp4의 항목')
    expect(screen.getByLabelText('고기2.mp4의 항목')).toBeInTheDocument()
    await user.click(first)
    // The label carries the group and the item's own facts, never a bare id.
    await user.click(await screen.findByRole('option', { name: '고기 · 삼겹살' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].compositionInputs?.associations).toEqual([
      {
        groupId: 'menu',
        itemId: 'belly',
        sourceId: 'src-1',
        fingerprint: 'fp-1',
        startMs: 0,
        endMs: 20000,
      },
    ])
  })

  it('binds a source to at most one item and clears it again', async () => {
    const user = userEvent.setup(),
      writes: ClipProjectDraft[] = []
    renderAppAt('/clips/owned', {
      user: { id: 'alice' },
      clips: {
        templates: [template],
        projects: [project],
        projectWrites: writes,
        retainedBatches: [
          batch([{ id: 'src-1', filename: '고기1.mp4', durationMs: 20000, fingerprint: 'fp-1' }]),
        ],
      },
    })
    const control = await screen.findByLabelText('고기1.mp4의 항목')
    await user.click(control)
    await user.click(await screen.findByRole('option', { name: '고기 · 삼겹살' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    await user.click(control)
    await user.click(await screen.findByRole('option', { name: '고기 · 목살' }))
    await waitFor(() => expect(writes).toHaveLength(2))
    // One item, not two: choosing again replaces rather than accumulates.
    expect(writes[1].compositionInputs?.associations).toHaveLength(1)
    expect(writes[1].compositionInputs?.associations?.[0].itemId).toBe('neck')
    await user.click(control)
    await user.click(await screen.findByRole('option', { name: '자동으로 연결' }))
    await waitFor(() => expect(writes).toHaveLength(3))
    expect(writes[2].compositionInputs?.associations).toEqual([])
  })

  it('offers nothing where the template declares no group', async () => {
    const plain =
      '<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="장소"/><text id="copy" kind="ai" role="caption" basis="whole">설명</text><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>'
    renderAppAt('/clips/owned', {
      user: { id: 'alice' },
      clips: {
        templates: [{ ...template, compositionBody: plain }],
        projects: [
          {
            ...project,
            composition: {
              snapshot: { version: 1, body: plain, templateId: 'meat', legacy: false },
              inputs: { values: {}, items: {}, associations: [] },
            },
            compositionInputs: { values: {}, items: {}, associations: [] },
          },
        ],
        retainedBatches: [
          batch([{ id: 'src-1', filename: '고기1.mp4', durationMs: 20000, fingerprint: 'fp-1' }]),
        ],
      },
    })
    expect(await screen.findByText('고기1.mp4')).toBeInTheDocument()
    expect(screen.queryByLabelText('고기1.mp4의 항목')).not.toBeInTheDocument()
  })
})
