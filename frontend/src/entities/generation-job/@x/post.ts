// What the post entity may import from the generation-job entity: a post read model
// carries the active durable job snapshot, and its row state reads whether that job still runs.
export type { GenerationJob } from '../model/types'
export { toGenerationJob } from '../api/job-mappers'
export { isTerminal } from '../model/types'
