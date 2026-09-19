// What the template entity may import from the post entity: every post projects the name of the
// template it was written from, and a delete detaches the assignment itself. Declared once in the
// entity that DEPENDS on the template (ARCH-14) and called by the template verbs.
export { invalidatePostsDependingOn } from '../api/post-dependencies'
