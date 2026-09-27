// What the guideline entity may import from the clip-template entity: a 영상 지침's scope names
// video templates, so the scope control needs the account's video templates to offer and label
// them (GUIDE-5). It never reads a composition.
export type { ClipTemplate } from '../model/types'
export { useClipTemplates } from '../api/clip-template'
