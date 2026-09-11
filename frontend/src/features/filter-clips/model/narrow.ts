import { clipState, type ClipProject, type ClipState } from '@/entities/clip-project'

/** The narrowing the clip directory holds, straight out of the URL (CLIP-41). */
export interface ClipNarrowing {
  q?: string
  status?: ClipState
}

/** Trimmed, case-folded and whitespace-collapsed. No `#` rule here: a clip project has no tags,
 *  so a leading hash is just a character someone typed. */
function normalize(value: string): string {
  return value.trim().replace(/\s+/g, ' ').toLocaleLowerCase()
}

/** Narrows the one list answer in the browser: the server returns every project the account owns
 *  (CLIP-2), and neither control is a query.
 *
 *  The search reads the title as stored. The status reads the project's OWN state (`clipState`),
 *  not the badge: 생성 중 and 실패 are things the badge says about the latest job, so a draft
 *  mid-generation stays under 초안 — the same rule the post list follows for its own badge. */
export function narrowClips(projects: ClipProject[], narrowing: ClipNarrowing): ClipProject[] {
  const query = normalize(narrowing.q ?? '')
  return projects.filter((project) => {
    if (narrowing.status && clipState(project) !== narrowing.status) return false
    return query === '' || normalize(project.title).includes(query)
  })
}
