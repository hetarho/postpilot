// What the voice entity may import from the post entity: every post projects its voice's name and
// its deletion state, so a voice write stales the post list and every post detail. The dependency
// is declared here — in the entity that DEPENDS on the voice — rather than restated in each voice
// verb (ARCH-14). T267 replaces the two keys with one invalidation entry.
export { listPostsQueryKey, postDetailQueriesKey } from '../api/post-queries'
