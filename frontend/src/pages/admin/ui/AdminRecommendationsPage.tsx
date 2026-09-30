import { RecommendationSetManager } from '@/features/manage-recommendation-set'

/** The 추천 조합 tab of `/admin` (MODEL-69). Composition only: the surface is one feature, and the
 *  page title and tab row belong to `AdminLayout`. */
export function AdminRecommendationsPage() {
  return <RecommendationSetManager />
}
