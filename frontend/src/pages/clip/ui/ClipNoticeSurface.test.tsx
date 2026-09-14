import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { ClipProjectSchema, contentLanguageToProto } from '@/shared/api'
import { toClipProject } from '@/entities/clip-project'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ClipResult, ClipDownloadAction } from '@/features/generate-clip'
import { FinalizeClipAction } from '@/features/finalize-clip'

afterEach(cleanup)
it.each(['ko', 'en'] as const)(
  'shows notices with the delivered result and before confirmation (%s)',
  (language) => {
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
    const action = {
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
    expect(confirmation.getAllByRole('listitem')).toHaveLength(3)
    expect(screen.getAllByRole('listitem')).toHaveLength(6)
    expect(screen.getByRole('link')).toHaveAttribute('href', project.result!.downloadUrl)
    const button = confirmation.getByRole('button')
    expect(button).toBeEnabled()
    fireEvent.click(button)
    expect(action.confirm).toHaveBeenCalledOnce()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  },
)
