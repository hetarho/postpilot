import i18next from 'i18next'
import type { ClipProject } from './types'

/** The project's name wherever a history row or the chrome names it: the title is optional and an
 *  empty one saves, so the clip is called `제목 없는 영상` until it has one (CLIP-130). */
export function clipDisplayTitle(project: Pick<ClipProject, 'title'>): string {
  return project.title.trim() || i18next.t('untitled', { ns: 'clips' })
}
