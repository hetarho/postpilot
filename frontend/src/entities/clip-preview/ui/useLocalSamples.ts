import { useEffect, useLayoutEffect, useRef, useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ClipRatioId } from '@/entities/clip-design/@x/clip-preview'
import { CLIP_BROWSER_COMPOSITION } from '@/entities/clip-design/@x/clip-preview'
import type { ClipCaptionPreview } from '@/entities/clip-plan/@x/clip-preview'
import { localCaptionFragments, localComponentSamples } from '../model/component-samples'
import { freezeBrowserPreviewComposition } from '../model/browser-composition'
import {
  projectBrowserComposition,
  type BrowserProjectCompositionInput,
} from '../model/project-composition'
import { BrowserLocalComponents } from '../model/local-components'

export function useClipLocalStyleSamples(ratio: ClipRatioId, enabled = true, pace = 'steady') {
  return useQuery({
    queryKey: [
      'clip-local-style-samples',
      CLIP_BROWSER_COMPOSITION.components,
      CLIP_BROWSER_COMPOSITION.assets,
      ratio,
      pace,
    ],
    enabled,
    gcTime: 0,
    staleTime: Infinity,
    retry: false,
    queryFn: ({ signal }) => localComponentSamples.styles(ratio, pace, signal),
  })
}
export function useClipLocalPresetSamples(ratio: ClipRatioId, slotLabel: string, enabled = true) {
  return useQuery({
    queryKey: [
      'clip-local-preset-samples',
      CLIP_BROWSER_COMPOSITION.components,
      CLIP_BROWSER_COMPOSITION.assets,
      ratio,
      slotLabel,
    ],
    enabled,
    gcTime: 0,
    staleTime: Infinity,
    retry: false,
    queryFn: ({ signal }) => localComponentSamples.presets(ratio, slotLabel, signal),
  })
}
/** Metadata is native whole-layout; only the selected caption gets one still bitmap. */
export function useClipLocalCaptionPreview(
  input: BrowserProjectCompositionInput | undefined,
  enabled: boolean,
  selectedId?: string,
) {
  const [retry, setRetry] = useState(0),
    key =
      enabled && input
        ? JSON.stringify([
            { ...input, projectRevision: undefined, planRevision: undefined },
            selectedId,
            retry,
          ])
        : ''
  const latest = useRef(input)
  useLayoutEffect(() => {
    latest.current = input
  }, [input])
  const renderer = useRef<{ owner: string; local: BrowserLocalComponents } | undefined>(undefined)
  const [state, setState] = useState<{ key: string; data?: ClipCaptionPreview; error?: unknown }>({
    key: '',
  })
  useEffect(() => {
    const controller = new AbortController()
    if (!key) return
    const [value, selected] = JSON.parse(key) as [
      BrowserProjectCompositionInput,
      string | undefined,
    ]
    void freezeBrowserPreviewComposition(
      projectBrowserComposition({
        ...value,
        projectRevision: latest.current!.projectRevision,
        planRevision: latest.current!.planRevision,
      }),
    )
      .then(async (snapshot) => {
        controller.signal.throwIfAborted()
        const owner = JSON.stringify([snapshot.ownerId, snapshot.projectId, snapshot.versions])
        if (renderer.current?.owner !== owner) {
          renderer.current?.local.destroy()
          renderer.current = { owner, local: new BrowserLocalComponents(snapshot) }
        } else renderer.current.local.updateSnapshot(snapshot)
        const data = await localCaptionFragments(
          snapshot,
          renderer.current.local,
          controller.signal,
          selected,
        )
        if (!controller.signal.aborted) setState({ key, data })
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) setState({ key, error })
      })
    return () => controller.abort()
  }, [key])
  useEffect(
    () => () => {
      renderer.current?.local.destroy()
      renderer.current = undefined
    },
    [],
  )
  const current = key && state.key === key ? state : undefined
  return {
    data: current?.data,
    error: current?.error,
    isError: !!current?.error,
    isPending: !!key && !current,
    isLoading: !!key && !current,
    refetch: useCallback(() => setRetry((value) => value + 1), []),
  }
}
