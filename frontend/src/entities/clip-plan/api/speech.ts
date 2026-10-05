import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { ClipSpeechService } from '@/shared/api'
import { API_URL } from '@/shared/config'
import { narrationFromProto, narrationToProto } from './spoken'
import type { ClipNarration } from '../model/spoken'
import type { ClipSpeechRef } from '../model/spoken'
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
    const calls = {
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
    return {
      ...calls,
      async load(projectId: string, speech: ClipSpeechRef, signal: AbortSignal) {
        const access = await calls.access(projectId, speech.assetId, signal)
        if (access.audioHash !== speech.audioHash) throw new Error('Speech provenance changed')
        let response = await fetch(access.url, {
          credentials: 'include',
          cache: 'no-store',
          signal,
        })
        // A ticket may expire during tab suspension. Retry authorization once, never synthesis.
        if (response.status === 404) {
          const fresh = await calls.access(projectId, speech.assetId, signal)
          if (fresh.audioHash !== speech.audioHash || fresh.bytes !== access.bytes)
            throw new Error('Speech provenance changed')
          response = await fetch(fresh.url, { credentials: 'include', cache: 'no-store', signal })
        }
        if (!response.ok || !response.headers.get('content-type')?.startsWith('audio/mpeg'))
          throw new Error('Speech unavailable')
        const reader = response.body?.getReader()
        if (!reader) throw new Error('Speech unavailable')
        const bytes = new Uint8Array(access.bytes)
        let offset = 0
        try {
          while (true) {
            signal.throwIfAborted()
            const chunk = await reader.read()
            if (chunk.done) break
            if (offset + chunk.value.length > bytes.length) throw new Error('Invalid speech size')
            bytes.set(chunk.value, offset)
            offset += chunk.value.length
          }
        } finally {
          await reader.cancel()
        }
        if (offset !== bytes.length) throw new Error('Invalid speech size')
        const hash = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), (n) =>
          n.toString(16).padStart(2, '0'),
        ).join('')
        if (hash !== speech.audioHash) throw new Error('Speech provenance changed')
        signal.throwIfAborted()
        return bytes.buffer
      },
    }
  }, [transport])
}
