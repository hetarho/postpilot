import type { ClipProject, ClipUploadTextLimits } from '@/entities/clip-project'

export function uploadDisclosure(project: Pick<ClipProject, 'campaign' | 'language'>) {
  if (!project.campaign) return ''
  return project.language === 'en'
    ? 'disclosure_banner_or_text_disclosure\n\n'
    : '대가표시_배너_또는_문구_대가표시\n\n'
}
export function uploadTags(value: string) {
  const seen = new Set<string>()
  return value
    .split(/\s+/u)
    .filter(Boolean)
    .map((tag) => tag.replace(/^#+/u, ''))
    .filter((tag) => {
      const key = tag.toLocaleLowerCase()
      if (!tag || seen.has(key)) return false
      seen.add(key)
      return true
    })
}
export function uploadTagText(tags: readonly string[]) {
  return tags.map((tag) => `#${tag}`).join(' ')
}
export function uploadCopyText(project: Pick<ClipProject, 'campaign' | 'language'>, body: string) {
  return uploadDisclosure(project) + body
}
export function uploadTextFits(
  body: string,
  tags: readonly string[],
  limits: ClipUploadTextLimits,
) {
  const combined = body + (limits.tagsInCaption && tags.length ? `\n\n${uploadTagText(tags)}` : '')
  return (
    !!body.trim() && Array.from(combined).length <= limits.maxChars && tags.length <= limits.maxTags
  )
}
