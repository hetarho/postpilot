import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { GenerationService } from '@/shared/api'
import type { GenerationJob } from '../model/types'
import { toGenerationJob } from './job-mappers'

/** A durable job read on demand, for a caller that waits on it inside its own run instead of
 *  showing it through a query: a browser render waiting on its sampling (CLIP-192). */
export interface GenerationJobCalls {
  get(id: string, signal: AbortSignal): Promise<GenerationJob>
}

export function generationJobCalls(transport: Transport): GenerationJobCalls {
  const client = createClient(GenerationService, transport)
  return {
    async get(id, signal) {
      const response = await client.getGeneration({ id }, { signal })
      if (!response.job) throw new Error('Missing job')
      return toGenerationJob(response.job)
    },
  }
}

export function useGenerationJobCalls(): GenerationJobCalls {
  const transport = useTransport()
  return useMemo(() => generationJobCalls(transport), [transport])
}
