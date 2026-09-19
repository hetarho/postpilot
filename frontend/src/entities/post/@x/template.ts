// What the template entity may import from the post entity: every post projects the name of the
// template it was written from, and a delete detaches the assignment itself. Declared here, in the
// entity that DEPENDS on the template (ARCH-14). T267 replaces the two keys with one entry.
export { listPostsQueryKey, postDetailQueriesKey } from '../api/post-queries'
