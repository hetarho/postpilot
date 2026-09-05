export const guidelines = {
  title: 'Guidelines',
  page: {
    description:
      'A guideline says what a post must avoid or watch out for. Saved guidelines apply to every post of this account, and can be narrowed to specific templates. Tone and sentence endings still follow your voice profile.',
    saved: 'Saved guidelines',
    new: 'New guideline',
    empty: 'No guidelines saved yet',
    emptyHelp:
      'Save the sentence you keep deleting from every draft, once, here. For example, like this.',
    example:
      'In posts about an unmanned store, do not mention staff or owner interactions, or CCTV',
    order: 'Guidelines apply global first, then the ones scoped to the post’s template.',
  },
  loadFailed: 'Could not load your guidelines.',
  scope: {
    label: 'Applies to',
    global: 'Everything',
    templates: 'Specific templates',
    globalHelp: 'Applies to every post of this account.',
    templatesHelp: 'Applies only to posts assigned one of the templates you pick.',
    pick: 'Templates',
    orphaned: 'Applies to nothing',
    orphanedHelp:
      'Every template this was scoped to has been deleted, so it currently reaches no post. Pick a scope again, or delete it.',
    templatesEmpty: 'Create a template first.',
  },
  create: {
    text: 'Guideline',
    textPlaceholder: 'e.g. In unmanned-store posts, do not mention CCTV',
    help: 'One short rule per guideline. A guideline wins over a conflicting template instruction.',
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
      'This removes the guideline. AI work already started finishes with the guidelines it started with, and your posts, templates and voices are untouched.',
  },
  candidate: {
    section: 'Candidates',
    sectionHelp:
      'What your finished AI revisions asked for, recorded word for word. Nothing here reaches a post while it sits in this list — approving one is where you choose what it applies to.',
    occurrences: 'asked {{count}} times',
    occurrences_one: 'asked {{count}} time',
    occurrences_other: 'asked {{count}} times',
    source: 'View the post',
    sourceGone: 'That post was deleted',
    queueFull:
      'The candidate list is full, so new requests are no longer recorded. Approve or dismiss one to start recording again.',
    approve: 'Approve',
    dismiss: 'Dismiss',
    approveTitle: 'Approve as a guideline',
    approveSubmit: 'Save as a guideline',
    approveDescription:
      'Save this request as a guideline. You can reword it, and you choose what it applies to here.',
    approveDuplicate: 'That guideline already exists. Reword it, or dismiss this candidate.',
  },
  capture: {
    action: 'Save as guideline',
    title: 'Save as guideline',
    description:
      'Save this revision instruction as a rule to keep applying to future posts. You can edit it before saving.',
    scopeGlobal: 'Everything',
    scopeTemplate: 'Only this post’s template “{{name}}”',
    submit: 'Save',
    saved: 'Saved as a guideline.',
    duplicate: 'You already have the same guideline.',
  },
} as const
