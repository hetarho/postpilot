import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { ClipProjectSchema, contentLanguageToProto } from '@/shared/api'
import { toClipProject } from '@/entities/clip-project'
import { clipTimelineFixture } from '@/test/clip-editing'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ClipResult, ClipDownloadAction } from '@/features/generate-clip'
import { FinalizeClipAction } from '@/features/finalize-clip'

afterEach(cleanup)
it.each(['ko', 'en'] as const)(
  'shows notices with the delivered result and before confirmation (%s)',
  async (language) => {
    const project = toClipProject(
      create(ClipProjectSchema, {
        id: 'clip',
        ratio: 'vertical',
        language: contentLanguageToProto(language),
        canFinalize: true,
        result: {
          id: 'render',
          contentType: 'video/mp4',
          viewUrl: 'https://example.test/video',
          downloadUrl: 'https://example.test/download',
        },
        notices: [
          { code: 'plan_cut_rate', cutId: 'cut-a', action: 'repair' },
          { code: 'shorter_copy', cutId: 'cut-a', elementId: 'caption', action: 'repair' },
          { code: 'plan_target_duration', action: 'shortfall' },
        ],
      }),
    )
    project.editing = clipTimelineFixture()
    project.editing.plan.cuts[0].id = 'cut-a'
    const action = {
      prepare: vi.fn(async () => project),
      confirm: vi.fn(async () => {}),
      checkAgain: vi.fn(async () => {}),
      pending: false,
      uncertain: false,
      failure: undefined,
      busy: false,
    }
    render(
      <>
        <ClipResult project={project} ownerId="owner" />
        <ClipDownloadAction project={project} />
        <div data-testid="confirmation">
          <FinalizeClipAction action={action} project={project} disabled={false} />
        </div>
      </>,
      {
        wrapper: withProviders(
          createRouterTransport(() => {}),
          createTestQueryClient(),
        ),
      },
    )
    const confirmation = within(screen.getByTestId('confirmation'))
    expect(confirmation.queryByRole('listitem')).not.toBeInTheDocument()
    expect(confirmation.queryByText(/확정하면|Confirmation deletes/)).not.toBeInTheDocument()
    // The delivered result keeps its own list, which is ③'s and not ②'s.
    expect(screen.getAllByRole('listitem')).toHaveLength(3)
    expect(screen.getByRole('link')).toHaveAttribute('href', project.result!.downloadUrl)
    const button = confirmation.getByRole('button')
    expect(button).toBeEnabled()
    fireEvent.click(button)
    const dialog = within(await screen.findByRole('dialog'))
    expect(action.prepare).toHaveBeenCalledOnce()
    expect(action.confirm).not.toHaveBeenCalled()
    expect(dialog.getByText(/확정하면/)).toBeVisible()
    expect(dialog.getAllByRole('listitem')).toHaveLength(3)
    expect(dialog.getByText(language === 'ko' ? /^컷 1 ·/ : /^Cut 1 ·/)).toBeVisible()
    fireEvent.click(dialog.getByRole('button', { name: '확정하기' }))
    expect(action.confirm).toHaveBeenCalledOnce()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  },
)
