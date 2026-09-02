// What the post entity may import from the model-experiment entity: deleting a post
// detaches its experiments (model_experiments.post_slug → NULL), so the experiment list
// the post delete invalidates is addressed by this key.
export { experimentListQueriesKey } from '../api/experiment-mappers'
