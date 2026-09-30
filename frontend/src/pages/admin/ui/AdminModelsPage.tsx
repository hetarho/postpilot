import { ModelCatalogManager } from '@/features/manage-model-catalog'
import { RecommendationSetManager } from '@/features/manage-recommendation-set'

/** The 모델 관리 tab of `/admin` (MODEL-28). Composition only: the catalog and the recommendation
 *  sets are two features, and the sets are the catalog's sixth tab (MODEL-69) — passed in as a
 *  slot because one feature may not import another (ARCH-13). The page title and tab row belong
 *  to `AdminLayout`; the estimator combos have their own tab (`AdminEstimatorPage`). */
export function AdminModelsPage() {
  return <ModelCatalogManager recommendations={<RecommendationSetManager />} />
}
