export const templates = {
  title: 'Templates',
  page: {
    description:
      'A template decides the structure and order of a post. Choose one per post and AI follows that structure, while the voice still controls style and endings.',
    saved: 'Saved templates',
    new: 'New template',
    empty: 'There are no saved templates yet',
    emptyHelp:
      'If you write the same shape of post again and again, build that shape once and pick it per post. Set the order — intro, a description per photo, a verdict — and AI follows it every time.',
    name: 'Name',
    newDockAria: 'Create a new template',
  },
  loadFailed: 'Could not load the template list.',
  noTemplate: 'None',
  create: {
    name: 'Template name',
    namePlaceholder: 'For example: Informational restaurant review',
    description: 'What kind of post is this?',
    descriptionPlaceholder: 'For example: A hosted restaurant visit review',
    body: 'Template structure',
    help: 'A template decides the structure and order. Style and endings still follow the voice profile.',
    submit: 'Create template',
  },
  emptyDescription: 'No description',
  screen: {
    backToList: '← Templates',
    newTitle: 'New template',
    notFound: 'Could not find this template. Pick one from the list again.',
    compositionHelp:
      'Add blocks above to lay out the post. Tap a line to edit it, and drag to reorder.',
    saved: 'Saved.',
    saveDockAria: 'Save template',
    leaveTitle: 'Leave without saving?',
    leaveDescription: 'Your changes have not been saved yet. Leaving now discards them.',
    leaveConfirm: 'Leave without saving',
  },
  composition: {
    add: 'Add block',
    empty: 'Add blocks above to lay out the post.',
    insertHere: 'Adds here',
    repeatEmpty: 'Nothing inside this repeat yet.',
    repeatHelp:
      'The blocks inside repeat once per attached photo. Each repetition uses {{count}} photos.',
    unreadable:
      'The structure of this template cannot be read — it may have been saved in an older format. Clear it and build it again.',
    clearAndRestart: 'Clear and start over',
    summary: {
      // Only ever formatted with a count of one — photoSummaryKey sends anything above it to
      // photoRow — so no plural form is needed on either side.
      photo: '{{count}} photo',
      photoRow: '{{count}} photos side by side',
    },
    placeholder: {
      write: 'Say what to write here',
      text: 'Type the text that goes in',
      note: 'Say something to AI only',
      photo: 'An attached photo goes here',
      repeat: 'Once per photo',
    },
  },
  builder: {
    palette: {
      write: 'AI writes here',
      writeHelp: 'AI writes prose here',
      text: 'Fixed text',
      textHelp: 'Appears in the post exactly as typed',
      photo: 'Photo',
      photoHelp: 'An attached photo goes here',
      repeat: 'Repeat per photos',
      repeatHelp: 'Repeats its contents once per photo',
      note: 'Note to AI',
      noteHelp: 'Only AI reads it; it never appears in the post',
    },
    // What an unlabelled legacy position is called once it is read as fixed text (TEMPLATE-37).
    legacy: {
      place: 'Map',
      link: 'Link',
    },
    block: {
      instruction: 'What to write',
      text: 'Text to include',
      label: 'Name for this position',
      note: 'Note for AI',
      count: 'Photos side by side',
      fewer: 'Fewer',
      more: 'More',
      drag: 'Drag to move',
      moveUp: 'Move up',
      moveDown: 'Move down',
      remove: 'Remove',
    },
    reasons: {
      unknown_tag: 'unknown notation',
      unclosed_tag: 'never closed',
      unexpected_close: 'closed without being opened',
      malformed_tag: 'malformed notation',
      missing_attribute: 'a required part is missing',
      unknown_slot_kind: 'unknown position kind',
      unknown_repeat_each: 'unknown repeat basis',
      nested_repeat: 'a repeat cannot contain a repeat',
      empty_write: 'nothing to write',
      empty_note: 'the note is empty',
      invalid_count: 'photos per row must be between 1 and {{max}}',
    },
  },
  slot: {
    unfilled: 'Position to fill',
    pending: '{{count}} positions to fill',
    pending_one: '{{count}} position to fill',
    pending_other: '{{count}} positions to fill',
    exportHint: 'Fill the bracketed positions in the platform editor after pasting.',
  },
  delete: {
    aria: 'Delete {{name}}',
    title: 'Delete this template?',
    description:
      '‘{{name}}’ will be removed. {{detach}} Posts that were already generated and work in progress are unaffected.',
  },
  assignment: {
    runningJob:
      'A running AI job finishes with the template it started with. A change applies from the next generation.',
    notFound: 'Could not find the selected template. Refresh the list and try again.',
    notFoundDetail:
      'Could not find the selected template. Refresh the list and try again. {{error}}',
    failed: 'Could not change the template. Please try again.',
  },
  postCount: '{{count}} posts',
  postCount_one: '{{count}} post',
  postCount_other: '{{count}} posts',
  detachWarning: {
    used: '{{count}} posts will lose their template. The posts and their content stay.',
    used_one: '{{count}} post will lose its template. The post and its content stay.',
    used_other: '{{count}} posts will lose their template. The posts and their content stay.',
    unused: 'No post uses this template.',
  },
} as const
