import { createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook, act } from '@testing-library/react'
import { expect, it } from 'vitest'
import i18next from 'i18next'
import { ClipGenerationService, contentLanguageToProto } from '@/shared/api'
import { withProviders } from '@/test/session'
import { useClipProjectMutations } from './clip-project'

it('sends the resolved UI language explicitly when creating a project', async () => {
  const seen: number[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipGenerationService.method.createClipProject, (request) => {
      seen.push(request.language)
      return {
        project: {
          id: 'project',
          title: request.title,
          ratio: request.ratio,
          language: request.language,
        },
      }
    })
  })
  const previous = i18next.language
  try {
    const view = renderHook(() => useClipProjectMutations('alice'), {
      wrapper: withProviders(transport, new QueryClient()),
    })
    for (const locale of ['ko', 'en']) {
      await i18next.changeLanguage(locale)
      await act(() =>
        view.result.current.save.mutateAsync({
          draft: {
            title: 'new',
            videoTemplateId: 'template',
            ratio: 'vertical',
            targetDurationMs: 15000,
            answers: [],
            disclosure: '',
            cta: '',
            hideDisclosure: false,
          },
        }),
      )
    }
    expect(seen).toEqual([contentLanguageToProto('ko'), contentLanguageToProto('en')])
  } finally {
    await i18next.changeLanguage(previous)
  }
})
