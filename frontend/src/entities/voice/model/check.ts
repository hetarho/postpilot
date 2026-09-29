import type { AppFailure } from '@/shared/api'
import type { FingerprintComparisonItem } from './fingerprint'
import type { VoicePrompt } from './types'

/** Where a 검증 is: queued and running while its job holds it, then done or failed. */
export type VoiceCheckStatus = 'queued' | 'running' | 'done' | 'failed'

/** One 검증 as the tab reads it (VOICE-43): the prompt, the owner's answer beside the piece, the
 *  piece measured against the current analysis, and whether an older analysis wrote it. */
export interface VoiceCheck {
  id: string
  prompt?: VoicePrompt
  /** The answer's current text; `answerDeleted` once that 학습 글 is gone. */
  answer: string
  answerDeleted: boolean
  status: VoiceCheckStatus
  piece: string
  failure?: AppFailure
  comparison: FingerprintComparisonItem[]
  /** Written from an analysis the voice has since replaced (이전 분석으로 검증). */
  stale: boolean
  createdAt: string
}
