// What the guideline entity may import from the blog-field entity: a guideline can be scoped to a
// set of 분야, so the scope control lists the catalogue and the mappers carry the set on the wire.
export type { BlogFieldId } from '../model/blog-field'
export { BLOG_FIELD_IDS, blogFieldLabelKey } from '../model/blog-field'
export { blogFieldToProto, requireBlogFieldId } from '../api/blog-field-mappers'
