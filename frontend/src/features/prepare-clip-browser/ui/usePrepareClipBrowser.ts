import { useEffect, useRef, useState } from 'react'
import {
  useClipAnalysisPreparationCalls,
  useClipLifecycleApi,
  type ClipAnalysisOriginalMeasurement,
} from '@/entities/clip-project'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import type { BrowserMediaSourceAccess } from '@/shared/lib'
import { ANALYSIS_PREPARATION_LIMITS as limits } from '../config/limits'
import { createAnalysisEncoder } from '../lib/encoder'
import { prepareBrowserAnalysis } from '../model/prepare'
import type { AnalysisPreparationProgress, ClipBrowserPreparation } from '../model/types'

export function usePrepareClipBrowser({
  ownerId,
  projectId,
  selectionKey,
  job,
  resolveAccess,
}: {
  ownerId: string
  projectId: string
  selectionKey: string
  job?: GenerationJob
  resolveAccess(
    sourceId: string,
    fingerprint: string,
    signal: AbortSignal,
  ): Promise<BrowserMediaSourceAccess>
}) {
  const calls = useClipAnalysisPreparationCalls()
  const lifecycle = useClipLifecycleApi(ownerId, projectId)
  const current = useRef<AbortController | undefined>(undefined)
  const parentId = useRef<string | undefined>(undefined)
  const measurements = useRef(new Map<string, ClipAnalysisOriginalMeasurement>())
  const [progress, setProgress] = useState<AnalysisPreparationProgress>()
  const [busy, setBusy] = useState(false)
  const [jobId, setJobId] = useState<string>()
  const [refusal, setRefusal] = useState<'codec' | 'memory' | 'color' | undefined>()
  const mounted = useRef(false)
  useEffect(() => {
    mounted.current = true
    const abort = () => current.current?.abort()
    window.addEventListener('pagehide', abort)
    window.addEventListener('beforeunload', abort)
    return () => {
      mounted.current = false
      abort()
      window.removeEventListener('pagehide', abort)
      window.removeEventListener('beforeunload', abort)
    }
  }, [])
  useEffect(() => {
    current.current?.abort()
  }, [selectionKey, ownerId, projectId])
  useEffect(() => {
    measurements.current.clear()
  }, [ownerId, projectId])
  useEffect(() => {
    if (job && job.id === parentId.current && (job.cancelRequestedAt || isTerminal(job)))
      current.current?.abort()
  }, [job])
  const preparation: ClipBrowserPreparation = {
    async run(input, startParent) {
      if (current.current) throw new Error('CLIP_BUSY')
      const controller = new AbortController()
      current.current = controller
      setBusy(true)
      setRefusal(undefined)
      setJobId(undefined)
      const deadline = setTimeout(
        () => controller.abort(new Error('CLIP_ANALYSIS_TIMEOUT')),
        limits.operationTimeoutMs,
      )
      try {
        return await prepareBrowserAnalysis(
          input,
          {
            ...calls,
            access: resolveAccess,
            encoder: (signal, progress) => {
              const encoder = createAnalysisEncoder(signal, progress)
              const valid = new Set(
                input.batch.sources.map((source) => `${source.id}/${source.metadata.fingerprint}`),
              )
              for (const key of measurements.current.keys())
                if (!valid.has(key)) measurements.current.delete(key)
              return {
                ...encoder,
                measure: async (source) => {
                  const key = `${source.sourceId}/${source.fingerprint}`
                  const cached = measurements.current.get(key)
                  if (cached) return { ...cached }
                  const measured = await encoder.measure(source)
                  signal.throwIfAborted()
                  measurements.current.set(key, measured)
                  return { ...measured }
                },
              }
            },
            cancelParent: lifecycle.cancel,
          },
          controller.signal,
          async (id) => {
            const parent = await startParent(id)
            parentId.current = parent.jobId
            if (mounted.current) setJobId(parent.jobId)
            return parent
          },
          (value) => {
            if (mounted.current && !controller.signal.aborted) setProgress(value)
          },
        )
      } catch (error) {
        if (mounted.current && !controller.signal.aborted && error instanceof Error) {
          if (error.message.includes('MEMORY_LIMIT')) setRefusal('memory')
          else if (error.message.includes('COLOR_UNSUPPORTED')) setRefusal('color')
          else if (error.message.includes('UNSUPPORTED')) setRefusal('codec')
        }
        throw error
      } finally {
        clearTimeout(deadline)
        if (current.current === controller) {
          current.current = undefined
          parentId.current = undefined
        }
        if (mounted.current) {
          setBusy(false)
          setProgress(undefined)
        }
      }
    },
  }
  return { preparation, progress, busy, refusal, jobId, cancel: () => current.current?.abort() }
}
