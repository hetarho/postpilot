export const guidelines = {
  title: 'Guidelines',
  page: {
    description:
      'A guideline says what a post must avoid or watch out for. Saved guidelines apply to every post of this account, and can be narrowed to specific purposes. Tone and sentence endings still follow your voice profile.',
    saved: 'Saved guidelines',
    new: 'New guideline',
    empty: 'No guidelines saved yet',
    emptyHelp:
      'Save the sentence you keep deleting from every draft, once, here. For example, like this.',
    example:
      'In posts about an unmanned store, do not mention staff or owner interactions, or CCTV',
    order: 'Guidelines apply global first, then the ones scoped to the post’s purpose.',
  },
  loadFailed: 'Could not load your guidelines.',
  scope: {
    label: 'Applies to',
    global: 'Everything',
    purposes: 'Specific purposes',
    globalHelp: 'Applies to every post of this account.',
    purposesHelp: 'Applies only to posts assigned one of the purposes you pick.',
    pick: 'Purposes',
    orphaned: 'Applies to nothing',
    orphanedHelp:
      'Every purpose this was scoped to has been deleted, so it currently reaches no post. Pick a scope again, or delete it.',
    purposesEmpty: 'Create a purpose first.',
  },
  create: {
    text: 'Guideline',
    textPlaceholder: 'e.g. In unmanned-store posts, do not mention CCTV',
    help: 'One short rule per guideline. A guideline wins over a conflicting purpose instruction.',
    submit: 'Create guideline',
  },
  edit: {
    text: 'Guideline',
    scope: 'Applies to',
  },
  delete: {
    aria: 'Delete guideline',
    title: 'Delete this guideline?',
    description:
      'This removes the guideline. AI work already started finishes with the guidelines it started with, and your posts, purposes and voices are untouched.',
  },
  capture: {
    action: 'Save as guideline',
    title: 'Save as guideline',
    description:
      'Save this revision instruction as a rule to keep applying to future posts. You can edit it before saving.',
    scopeGlobal: 'Everything',
    scopePurpose: 'Only this post’s purpose “{{name}}”',
    submit: 'Save',
    saved: 'Saved as a guideline.',
    duplicate: 'You already have the same guideline.',
  },
} as const
