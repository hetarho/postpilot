// What the voice entity may import from the post entity: every post projects its voice's name and
// its deletion state, so a voice write stales the post list and every post detail. The dependency
// is declared once in the entity that DEPENDS on the voice (ARCH-14) and the voice verbs call it.
export { invalidatePostsDependingOn } from '../api/post-dependencies'
// A voice habit phrase is located in a post field the way an origin span is (VOICE-77).
export type { OriginFieldLocator } from '../model/semantic-origin'
