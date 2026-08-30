import type { PublishVisibility } from '@/shared/api'

export interface PublishingCategory {
  id: string
  name: string
}

/** Safe server metadata only. Tokens, cookies, CDP endpoints and local paths have no field here. */
export interface PublishingAgent {
  id: string
  label: string
  platformAccountId: string
  platformAccountLabel: string
  browserLabel: string
  categories: PublishingCategory[]
  defaultCategoryId: string
  defaultVisibility: PublishVisibility
  lastSeenAt: string
  revokedAt: string
  ready: boolean
}
