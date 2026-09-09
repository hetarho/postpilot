import { MODEL_PURPOSES, type ModelPurpose } from '@/shared/config'
import type { CatalogDocumentPlan, CatalogDocumentPurposePlan } from '@/entities/model-catalog'

/** The causes the server reports. Anything outside this list is rendered through the fallback
 *  copy rather than shown raw: a slug a newer server invented must not reach the operator as
 *  `purpose_ineligible`. */
export const DOCUMENT_ISSUE_CAUSES = [
  'bad_version',
  'unknown_purpose',
  'duplicate_section',
  'id_before_section',
  'malformed_line',
  'duplicate_id',
  'unknown_model',
  'unlisted_model',
  'purpose_ineligible',
] as const

export type DocumentIssueCause = (typeof DOCUMENT_ISSUE_CAUSES)[number]

export function isKnownCause(cause: string): cause is DocumentIssueCause {
  return (DOCUMENT_ISSUE_CAUSES as readonly string[]).includes(cause)
}

/** One purpose's row in the diff. `touched` is what decides whether it is worth drawing at
 *  all: a section that lists exactly what is already registered is a no-op the operator should
 *  see as such, not as an empty pair of lists. */
export interface DocumentDiffRow extends CatalogDocumentPurposePlan {
  touched: boolean
}

/** The five purposes in tab order, so the diff reads in the same order as the tabs above it,
 *  with the ones the document never named marked absent rather than empty. */
export interface DocumentDiff {
  rows: DocumentDiffRow[]
  untouched: ModelPurpose[]
  registerCount: number
  deregisterCount: number
}

export function documentDiff(plan: CatalogDocumentPlan | undefined): DocumentDiff {
  const named = new Map(plan?.purposes.map((purpose) => [purpose.purpose, purpose]) ?? [])
  const rows = MODEL_PURPOSES.flatMap<DocumentDiffRow>((purpose) => {
    const row = named.get(purpose)
    if (!row) return []
    return [{ ...row, touched: row.register.length > 0 || row.deregister.length > 0 }]
  })
  return {
    rows,
    // A purpose with no section is untouched, and saying so is the point: it is the difference
    // between "this document leaves 글쓰기 alone" and "this document empties it".
    untouched: MODEL_PURPOSES.filter((purpose) => !named.has(purpose)),
    registerCount: rows.reduce((total, row) => total + row.register.length, 0),
    deregisterCount: rows.reduce((total, row) => total + row.deregister.length, 0),
  }
}

/** 확정 is available only for a plan that was actually computed, has nothing refused, and
 *  would change something. A refused document must not be committable, and neither must a
 *  document nobody has previewed yet. */
export function canApply(plan: CatalogDocumentPlan | undefined): boolean {
  if (!plan || plan.fetchError !== '' || plan.issues.length > 0) return false
  const diff = documentDiff(plan)
  return diff.registerCount > 0 || diff.deregisterCount > 0
}
