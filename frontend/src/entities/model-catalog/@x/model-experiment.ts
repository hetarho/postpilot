export type { ModelRef, StageName } from '../model/types'
export { stageToProto, toModelRef } from '../api/catalog-mappers'
// Adopting a winner model rewrites the account's stage selections.
export { getSelectionsQueryKey } from '../api/catalog-mappers'
