import { EstimatorCombos } from '@/features/assign-estimator-combo'

/** The 편수 기준 조합 tab of `/admin`. Composition only: the surface is one feature, and the page
 *  title and tab row belong to `AdminLayout`.
 *
 *  It used to be a section under the model catalog (QUOTA-39 calls assigning a price tier
 *  curation of the same registrations), and it still edits those registrations — but the catalog
 *  is several hundred rows long, so the section sat a screen and a half below the tab row and an
 *  operator looking for it had to know it was there. Its own address puts it one tap away and
 *  makes it bookmarkable like the other two. */
export function AdminEstimatorPage() {
  return <EstimatorCombos />
}
