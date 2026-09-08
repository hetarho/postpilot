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
    mode: {
      aria: 'How to edit',
      builder: 'Blocks',
      source: 'Source',
    },
    sourceHelp:
      'See and edit the template in its stored format. Hand the format guide to an AI and paste what it writes here.',
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
    fixInSource: 'Fix in source',
    add: 'Add block',
    empty: 'Add blocks above to lay out the post.',
    insertHere: 'Adds here',
    repeatEmpty: 'Nothing inside this repeat yet.',
    repeatHelp:
      'The blocks inside repeat once per attached photo. Each repetition uses {{count}} photos.',
    unreadable:
      "This template's composition can't be read. Fix it in the source, or clear it and start over.",
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
  // The one surface where this app's grammar is visible (TEMPLATE-26).
  source: {
    label: 'Source',
    copy: 'Copy source',
    copyGuide: 'Copy format guide',
    copied: 'Copied',
    manualCopy: 'Copying is blocked. Press and hold the selected text to copy it.',
    showGuide: 'Show format guide',
    error: 'Line {{line}}: {{reason}}',
    guide: `Write the body of one blog post template in the format below.

[What a template does]
A template is the skeleton of a post. It decides the order and what goes where; it never decides tone or word choice.

[The six things you can write]
- Plain text: appears in the post exactly as written.
- <write>what to write</write>: the AI writes here as instructed. The instruction itself never appears in the post.
- <slot kind="photo"/>: one attached photo goes here. With a count, as in <slot kind="photo" count="2"/>, that many stand side by side in one row.
- <repeat each="photo">…</repeat>: what is inside repeats once per group of photos.
- <note>note to the AI</note>: only the AI reads it; it never appears in the post.
- <ask label="field title"/>: what the author types on the write screen goes here exactly as typed.
- <ask label="field title">what to write</ask>: the AI writes here using only what the author typed as its facts.

[Rules that must hold]
- write, note and repeat must be closed.
- slot closes itself, as <slot …/>, and kind may only be photo.
- count is a whole number from 1 to {{photoRowMax}}. Without it, one photo.
- repeat takes only each="photo", and a repeat may not contain a repeat.
- write and note may never be empty.
- ask must carry a label, no two may share one in the same body, none may sit inside a repeat, and one body holds at most {{askMax}}.
- Use ask only for what the author has to supply each time — a rating, a visit date, a price: facts the AI cannot know.
- Any tag other than those six is refused.
- To write a literal < in a sentence, write &lt; instead.
- The whole body may not exceed {{bodyMax}} characters.

[Example]
{{example}}

[How to answer]
Send the body only — no explanation and no code fence. Write it in the language I am writing in.`,
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
      asksForData: 'Ask for data',
      asksForDataHelp:
        'On, the write screen asks you for what goes here instead of the AI inventing it.',
      askTitle: 'Field title',
      askInRepeat:
        'A row inside Repeat per photos cannot ask for data: how many fields there are must not depend on the photo count.',
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
      duplicate_ask_label: 'another field already asks under that title',
      ask_in_repeat: 'a field inside 사진마다 반복 cannot ask for data',
      too_many_asks: 'at most {{askMax}} fields may ask for data',
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
