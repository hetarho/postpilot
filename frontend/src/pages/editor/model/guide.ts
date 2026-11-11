import i18next from 'i18next'
import { isTerminal } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'

/** Campaign requirements set the context; photos still precede all writing inputs. */
export type GuideStep =
  'kind' | 'requirements' | 'photos' | 'template' | 'details' | 'story' | 'review'

export const GUIDE_STEPS: readonly GuideStep[] = [
  'kind',
  'requirements',
  'photos',
  'template',
  'details',
  'story',
  'review',
]

export function activeGuideSteps(sponsored = true): readonly GuideStep[] {
  return sponsored ? GUIDE_STEPS : GUIDE_STEPS.filter((step) => step !== 'requirements')
}

export function guideSteps(sponsored = true): ReadonlyArray<{ value: GuideStep; label: string }> {
  return activeGuideSteps(sponsored).map((value) => ({
    value,
    label: i18next.t(`guide.steps.${value}`, { ns: 'posts' }),
  }))
}

export function guideIndex(step: GuideStep, sponsored = true): number {
  return activeGuideSteps(sponsored).indexOf(step)
}

export function nextGuideStep(step: GuideStep, sponsored = true): GuideStep | undefined {
  return activeGuideSteps(sponsored)[guideIndex(step, sponsored) + 1]
}

export function previousGuideStep(step: GuideStep, sponsored = true): GuideStep | undefined {
  const index = guideIndex(step, sponsored)
  return index > 0 ? activeGuideSteps(sponsored)[index - 1] : undefined
}

/** Returning drafts with material or a retained job open at the summary. A newly minted draft
 *  instead consumes its stashed step, so entering a guide never skips the photo/template flow. */
export function initialGuideStep(
  post:
    Pick<PostDraft, 'memo' | 'images' | 'videos' | 'activeJob' | 'pendingExperimentId'> | undefined,
): GuideStep {
  if (!post) return 'kind'
  if (post.activeJob && !isTerminal(post.activeJob)) return 'review'
  if (post.pendingExperimentId) return 'review'
  if (post.memo.trim() || post.images.length > 0 || post.videos.length > 0) return 'review'
  return 'kind'
}

/** The step a draft was on when its first save minted it. `/posts/new` and `/posts/$slug` are two
 *  editor mounts, so the step the owner is on has to cross the remount the way the caret does —
 *  otherwise the first keystroke of a story would throw them back to the summary. */
const minted = new Map<string, GuideStep>()

export function stashGuideStep(slug: string, step: GuideStep): void {
  minted.set(slug, step)
}

export function takeGuideStep(slug: string | undefined): GuideStep | undefined {
  if (!slug) return undefined
  const step = minted.get(slug)
  minted.delete(slug)
  return step
}
