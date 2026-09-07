import { EstimatorCombos } from '@/features/assign-estimator-combo'
import { ModelCatalogManager } from '@/features/manage-model-catalog'

/** The 모델 관리 tab of `/admin` (plan 18). Composition only: the surface is two features,
 *  and the page title and tab row belong to `AdminLayout`.
 *
 *  The estimator combos sit under the catalog because assigning a price tier is curation of
 *  the same registrations the list above it edits (QUOTA-39). */
export function AdminModelsPage() {
  return (
    <>
      <ModelCatalogManager />
      <EstimatorCombos />
    </>
  )
}
