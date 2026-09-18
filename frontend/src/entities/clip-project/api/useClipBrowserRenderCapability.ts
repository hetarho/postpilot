import { useQuery } from '@tanstack/react-query'
import { probeEncoderSupport } from '@/shared/lib'
import {
  clipBrowserEncoderConfig,
  clipBrowserRenderCapability,
} from '../model/browser-render-capability'
import type { ClipRatio } from '../model/types'

export function useClipBrowserRenderCapability(ratio: ClipRatio, needsAudio: boolean) {
  return useQuery({
    queryKey: ['clip-browser-render-capability', ratio, needsAudio],
    queryFn: async () => {
      const config = clipBrowserEncoderConfig(ratio)
      const support = await probeEncoderSupport(config.video, needsAudio ? config.audio : undefined)
      return clipBrowserRenderCapability(support, needsAudio)
    },
    staleTime: Infinity,
    retry: false,
    // This query is entirely local, including when the page is offline.
    networkMode: 'always',
  })
}
