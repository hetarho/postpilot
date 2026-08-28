// What the post entity may import from the generation-job entity: a post read model
// carries the active durable job snapshot.
export type { GenerationJob } from '../model/types'
export { toGenerationJob } from '../api/job-mappers'
