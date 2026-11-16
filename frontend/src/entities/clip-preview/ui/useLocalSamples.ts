import { useEffect, useLayoutEffect, useMemo, useRef, useState, useCallback } from 'react'
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
/**
 * Every input field is classified: `key` fields enter the preview key, `revision` fields stay out
 * (the effect reads them from the latest input). A new field fails to compile until it is listed.
 */
export const CAPTION_PREVIEW_KEY_FIELDS = {
  ownerId: 'key',
  projectId: 'key',
  plan: 'key',
  ratio: 'key',
  sources: 'key',
  layoutObservations: 'key',
  versions: 'key',
  authoritativeFingerprint: 'key',
  design: 'key',
  projectRevision: 'revision',
  planRevision: 'revision',
} as const satisfies Record<keyof BrowserProjectCompositionInput, 'key' | 'revision'>

/** Serializes the whole composition only when one of its inputs changes identity (or value, for design). */
function useCaptionPreviewKey(
  input: BrowserProjectCompositionInput | undefined,
  enabled: boolean,
  selectedId: string | undefined,
  retry: number,
) {
  const present = !!input
  const ownerId = input?.ownerId
  const projectId = input?.projectId
  const plan = input?.plan
  const ratio = input?.ratio
  const sources = input?.sources
  const layoutObservations = input?.layoutObservations
  const versions = input?.versions
  const authoritativeFingerprint = input?.authoritativeFingerprint
  // Callers pass a fresh design literal each render, so it is compared by value.
  const designKey = JSON.stringify(input?.design ?? null)
  return useMemo(
    () =>
      enabled && present
        ? JSON.stringify([
            {
              ownerId,
              projectId,
              plan,
              ratio,
              sources,
              layoutObservations,
              versions,
              authoritativeFingerprint,
              design:
                (JSON.parse(designKey) as BrowserProjectCompositionInput['design'] | null) ??
                undefined,
            },
            selectedId,
            retry,
          ])
        : '',
    [
      enabled,
      present,
      ownerId,
      projectId,
      plan,
      ratio,
      sources,
      layoutObservations,
      versions,
      authoritativeFingerprint,
      designKey,
      selectedId,
      retry,
    ],
  )
}
/** Metadata is native whole-layout; only the selected caption gets one still bitmap. */
export function useClipLocalCaptionPreview(
  input: BrowserProjectCompositionInput | undefined,
  enabled: boolean,
  selectedId?: string,
) {
  const [retry, setRetry] = useState(0)
  const key = useCaptionPreviewKey(input, enabled, selectedId, retry)
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
