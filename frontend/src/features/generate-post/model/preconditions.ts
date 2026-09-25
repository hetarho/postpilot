import i18next from 'i18next'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostImage } from '@/entities/image'
import type { ModelRef } from '@/entities/model-catalog'
import { deletedVoiceAIReason, type VoiceRef } from '@/entities/voice'

export interface GenerationModelSelection {
  ref: ModelRef
  vision: boolean
  /** The model takes VIDEO input. Checked only when the post actually carries a clip: watching
   *  is not a purpose, it is a per-run requirement (VIDEO-11). */
  videoInput?: boolean
  signedVideoUrl?: boolean
}

/** Why an action cannot run, as a value the UI can branch on. String-matching a translated
 *  sentence is not a branch, and the editor now has to tell "the models are not set up yet" —
 *  which it answers with a way to go and set them up — from "a job is already running", which it
 *  answers by waiting. */
export type GenerationBlocker =
  | 'published'
  | 'voiceDeleted'
  | 'activeJob'
  | 'observe'
  | 'vision'
  | 'videoModel'
  | 'videoUrl'
  | 'write'
  | 'pair'
  | 'different'

/** The blockers a route out of this screen can fix. `observe` · `vision` · `write` are the active
 *  selections and `pair` · `different` are the A/B candidates; all five are set in the writing
 *  brief. Everything else resolves on its own or belongs to another surface. */
const SETUP_BLOCKERS = new Set<GenerationBlocker>([
  'observe',
  'vision',
  'videoModel',
  'videoUrl',
  'write',
  'pair',
  'different',
])

export function isSetupBlocker(blocker: GenerationBlocker | undefined): boolean {
  return blocker !== undefined && SETUP_BLOCKERS.has(blocker)
}

/** Where the user has to go to clear a setup blocker. One destination since the A/B candidates
 *  joined the brief: sending someone off to the AI 모델 page for a pair they can now set two taps
 *  away, without leaving the draft, is a longer road to the same two dropdowns. */
export function setupBlockerTarget(blocker: GenerationBlocker | undefined): 'brief' | undefined {
  return isSetupBlocker(blocker) ? 'brief' : undefined
}

export type GenerationPreconditions =
  | { ok: true; reason: ''; blocker?: undefined }
  | { ok: false; reason: string; blocker: GenerationBlocker }

/** What every generation gate reads. Each member is required, and a value that may be absent is
 *  `| undefined` rather than optional: a caller that leaves one out does not compile, instead of
 *  passing the blocker it feeds (review F23). */
export interface GenerationGateInput {
  images: readonly Pick<PostImage, 'id'>[]
  videos: readonly unknown[]
  published: boolean
  activeJob: Pick<GenerationJob, 'status'> | undefined
  voice: Pick<VoiceRef, 'deleted'> | undefined
  observe: GenerationModelSelection | undefined
}

/** Mirrors the server gate so an impossible generation never looks clickable. A published post
 *  comes first: it takes no write at all (POST-86), so nothing else about the run matters. Then
 *  the voice: a deleted voice refuses every machine result before any model is even asked about
 *  (spec/legacy/policy/generation.md). */
function sharedPreconditions({
  images,
  videos,
  published,
  activeJob,
  voice,
  observe,
}: GenerationGateInput): GenerationPreconditions {
  if (published)
    return {
      ok: false,
      reason: i18next.t('published.locked', { ns: 'posts' }),
      blocker: 'published',
    }
  if (voice?.deleted) return { ok: false, reason: deletedVoiceAIReason(), blocker: 'voiceDeleted' }
  if (activeJob && activeJob.status !== 'done' && activeJob.status !== 'failed') {
    return {
      ok: false,
      reason: i18next.t('generation.blocked.active', { ns: 'posts' }),
      blocker: 'activeJob',
    }
  }
  return observePreconditions(images.length, videos.length, observe)
}

/** Whether the observe model can watch this post's media. A post with none never observes. */
function observePreconditions(
  photoCount: number,
  videoCount: number,
  observeSelection: GenerationModelSelection | undefined,
): GenerationPreconditions {
  if (photoCount === 0 && videoCount === 0) return { ok: true, reason: '' }
  if (!observeSelection)
    return {
      ok: false,
      reason: i18next.t('generation.blocked.observe', { ns: 'posts' }),
      blocker: 'observe',
    }
  // Vision first: a model that cannot see a photo is the simpler thing to fix, and a post with
  // both kinds needs both capabilities anyway.
  if (photoCount > 0 && !observeSelection.vision) {
    return {
      ok: false,
      reason: i18next.t('generation.blocked.vision', { ns: 'posts' }),
      blocker: 'vision',
    }
  }
  // Only when the post actually carries a clip. The server refuses the same run before
  // enqueue; this is what stops the run starting (VIDEO-11).
  if (videoCount > 0 && !observeSelection.videoInput) {
    return {
      ok: false,
      reason: i18next.t('generation.blocked.videoModel', { ns: 'posts' }),
      blocker: 'videoModel',
    }
  }
  if (videoCount > 0 && !observeSelection.signedVideoUrl) {
    return {
      ok: false,
      reason: i18next.t('generation.blocked.videoUrl', { ns: 'posts' }),
      blocker: 'videoUrl',
    }
  }
  return { ok: true, reason: '' }
}

export function ordinaryGenerationPreconditions(
  input: GenerationGateInput & { write: GenerationModelSelection | undefined },
): GenerationPreconditions {
  const shared = sharedPreconditions(input)
  if (!shared.ok) return shared
  if (!input.write)
    return {
      ok: false,
      reason: i18next.t('generation.blocked.write', { ns: 'posts' }),
      blocker: 'write',
    }
  return { ok: true, reason: '' }
}

export function comparisonGenerationPreconditions(
  input: GenerationGateInput & {
    writeA: GenerationModelSelection | undefined
    writeB: GenerationModelSelection | undefined
  },
): GenerationPreconditions {
  const shared = sharedPreconditions(input)
  if (!shared.ok) return shared
  return pairPreconditions(input.writeA, input.writeB)
}

function pairPreconditions(
  writeSelectionA: GenerationModelSelection | undefined,
  writeSelectionB: GenerationModelSelection | undefined,
): GenerationPreconditions {
  if (!writeSelectionA || !writeSelectionB)
    return {
      ok: false,
      reason: i18next.t('generation.blocked.pair', { ns: 'posts' }),
      blocker: 'pair',
    }
  if (
    writeSelectionA.ref.providerId === writeSelectionB.ref.providerId &&
    writeSelectionA.ref.modelId === writeSelectionB.ref.modelId
  )
    return {
      ok: false,
      reason: i18next.t('generation.blocked.different', { ns: 'posts' }),
      blocker: 'different',
    }
  return { ok: true, reason: '' }
}

/** The two runs 글 생성 starts. */
export type GenerationMode = 'generation' | 'comparison'

/** A field of the writing brief a run can be waiting on. */
export type BriefField = 'observe' | 'write' | 'pair'

/** What each brief field still needs before a run of one mode can start, in that field's own
 *  words; a field with nothing missing is absent. */
export type BriefIssues = Partial<Record<BriefField, string>>

/** EVERY brief field the mode is waiting on, not only the first: a press on 생성 opens the brief
 *  with each of them marked, so the user fixes them in one visit rather than one per press. The
 *  blockers no brief field resolves (a job running, a deleted voice, a published post) are not
 *  here — those keep the action itself disabled. */
export function briefIssues(
  mode: GenerationMode,
  photoCount: number,
  videoCount: number,
  observeSelection: GenerationModelSelection | undefined,
  writeSelection: GenerationModelSelection | undefined,
  writeSelectionA: GenerationModelSelection | undefined,
  writeSelectionB: GenerationModelSelection | undefined,
): BriefIssues {
  const issues: BriefIssues = {}
  const observe = observePreconditions(photoCount, videoCount, observeSelection)
  if (!observe.ok) issues.observe = observe.reason
  if (mode === 'generation') {
    if (!writeSelection) issues.write = i18next.t('generation.blocked.write', { ns: 'posts' })
  } else {
    const pair = pairPreconditions(writeSelectionA, writeSelectionB)
    if (!pair.ok) issues.pair = pair.reason
  }
  return issues
}
