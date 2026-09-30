import i18next from 'i18next'
import type { CatalogModel, PostCreditFigure, StageName } from './types'

/** A model's per-post figure for one stage, if the server sent one. */
export function stagePostFigure(
  model: CatalogModel | undefined,
  stage: StageName,
): PostCreditFigure | undefined {
  return model?.postCredits?.[stage]
}

/** `최근 사용량 기준 글 1개당 약 N크레딧` or `예상 글 1개당 약 N크레딧` (QUOTA-64). */
export function postCreditLabel(figure: PostCreditFigure): string {
  return i18next.t(`postCredits.${figure.basis}`, { ns: 'models', credits: figure.credits })
}

/** One post on a pair (QUOTA-64): the write figure, plus the observe figure when the post has
 *  photos. A free stage adds nothing; a paid stage the server has no figure for means there is
 *  no number to show, and so does a pair of free models, which cost no credits at all. The sum
 *  is an estimate when either part is. */
export function pairPostFigure({
  observe,
  write,
  withPhotos,
}: {
  observe: CatalogModel | undefined
  write: CatalogModel | undefined
  withPhotos: boolean
}): PostCreditFigure | undefined {
  const parts: PostCreditFigure[] = []
  const add = (model: CatalogModel | undefined, stage: StageName): boolean => {
    if (!model) return false
    if (model.access?.[stage]?.grade === 'free') return true
    const figure = stagePostFigure(model, stage)
    if (!figure) return false
    parts.push(figure)
    return true
  }
  if (!add(write, 'write')) return undefined
  if (withPhotos && observe && !add(observe, 'observe')) return undefined
  const credits = parts.reduce((sum, part) => sum + part.credits, 0)
  if (credits === 0) return undefined
  return {
    credits,
    basis: parts.some((part) => part.basis === 'estimate') ? 'estimate' : 'recent',
  }
}
