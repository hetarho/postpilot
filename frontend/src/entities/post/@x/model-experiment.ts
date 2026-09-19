// What the model-experiment entity may import from the post entity: applying a winner writes the
// post the experiment ran on, so that post and the list showing its status are stale.
export { invalidateWrittenPost } from '../api/post-dependencies'
