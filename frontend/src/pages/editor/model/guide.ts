import i18next from 'i18next'
import { isTerminal } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'

/** Guide: 글 생성 as four screens, one question each — the story first, then the photos, then the
 *  template's answers and the title, then a short summary that ends on 생성. */
export type GuideStep = 'story' | 'photos' | 'details' | 'review'

export const GUIDE_STEPS: readonly GuideStep[] = ['story', 'photos', 'details', 'review']

export function guideSteps(): ReadonlyArray<{ value: GuideStep; label: string }> {
  return GUIDE_STEPS.map((value) => ({
    value,
    label: i18next.t(`guide.steps.${value}`, { ns: 'posts' }),
  }))
}

export function guideIndex(step: GuideStep): number {
  return GUIDE_STEPS.indexOf(step)
}

export function nextGuideStep(step: GuideStep): GuideStep | undefined {
  return GUIDE_STEPS[guideIndex(step) + 1]
}

export function previousGuideStep(step: GuideStep): GuideStep | undefined {
  const index = guideIndex(step)
  return index > 0 ? GUIDE_STEPS[index - 1] : undefined
}

/** Where a draft opens. A new draft starts on its story. A draft that already holds material —
 *  a story or a photo — has a run going, or has a paid comparison's result waiting opens on the
 *  summary, where 생성 and that result are: the owner coming back to it has already answered the
 *  questions. */
export function initialGuideStep(
  post:
    Pick<PostDraft, 'memo' | 'images' | 'videos' | 'activeJob' | 'pendingExperimentId'> | undefined,
): GuideStep {
  if (!post) return 'story'
  if (post.activeJob && !isTerminal(post.activeJob)) return 'review'
  if (post.pendingExperimentId) return 'review'
  if (post.memo.trim() || post.images.length > 0 || post.videos.length > 0) return 'review'
  return 'story'
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
