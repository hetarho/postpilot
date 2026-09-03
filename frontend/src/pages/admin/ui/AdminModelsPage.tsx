import { ModelCatalogManager } from '@/features/manage-model-catalog'

/** The 모델 관리 tab of `/admin` (plan 18). Composition only: the whole surface is one feature,
 *  and the page title and tab row belong to `AdminLayout`. */
export function AdminModelsPage() {
  return <ModelCatalogManager />
}
