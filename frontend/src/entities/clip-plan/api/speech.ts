import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { ClipSpeechService } from '@/shared/api'
import { API_URL } from '@/shared/config'
import { narrationFromProto, narrationToProto } from './spoken'
import type { ClipNarration } from '../model/spoken'
export interface ClipSpeechQuote {
  id: string
  maximumCredits: number
  expiresAt: string
  segmentIds: string[]
  revision: number
  cancellationPolicyVersion: number
}
export function useClipSpeechCalls() {
  const transport = useTransport()
  return useMemo(() => {
    const client = createClient(ClipSpeechService, transport)
    return {
      async recovery(projectId: string) {
        const r = await client.getClipSpokenDraft({ projectId })
        return { digest: r.digest, narration: narrationFromProto(r.narration) }
      },
      async saveRecovery(projectId: string, digest: string, narration: ClipNarration) {
        const r = await client.saveClipSpokenDraft({
          projectId,
          expectedDigest: digest,
          narration: narrationToProto(narration),
        })
        return { digest: r.digest, narration: narrationFromProto(r.narration) }
      },
      async quote(projectId: string, revision: number): Promise<ClipSpeechQuote> {
        const q = await client.quoteClipSpeech({ projectId, expectedRevision: revision })
        return {
          id: q.quoteId,
          maximumCredits: q.maximumCredits,
          expiresAt: q.expiresAt,
          segmentIds: q.segmentIds,
          revision: q.planRevision,
          cancellationPolicyVersion: q.cancellationPolicyVersion,
        }
      },
      async start(projectId: string, q: ClipSpeechQuote, key: string) {
        return client.startClipSpeech({
          projectId,
          expectedRevision: q.revision,
          quoteId: q.id,
          approvedMaxCredits: q.maximumCredits,
          cancellationPolicyVersion: q.cancellationPolicyVersion,
          idempotencyKey: key,
        })
      },
      async access(projectId: string, assetId: string, signal?: AbortSignal) {
        const a = await client.getClipSpeechAccess({ projectId, assetId }, { signal })
        if (
          !/^\/clip\/speech\/[a-f0-9]{32}$/.test(a.url) ||
          !/^[a-f0-9]{64}$/.test(a.audioHash) ||
          a.bytes <= 0n ||
          a.bytes > 8388608n
        )
          throw new Error('Invalid private speech access')
        return {
          url: `${import.meta.env.DEV ? '/api' : API_URL.replace(/\/$/, '')}${a.url}`,
          audioHash: a.audioHash,
          bytes: Number(a.bytes),
          expiresAt: a.expiresAt,
        }
      },
    }
  }, [transport])
}
