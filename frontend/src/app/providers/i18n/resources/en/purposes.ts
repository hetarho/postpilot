export const purposes = {
  title: 'Purposes',
  page: {
    description:
      'A purpose shapes the kind and structure of a post. Choose one per post and AI follows its guidelines while the voice still controls style and endings.',
    saved: 'Saved purposes',
    new: 'New purpose',
    empty: 'There are no saved purposes yet',
    emptyHelp: 'Save the context you repeatedly type in Notes once here. For example:',
    name: 'Name',
    exampleName: 'Informational restaurant review',
    exampleDescription: 'A hosted restaurant visit review',
    exampleInstructions:
      'Describe every photo.\nDo not write it like a diary.\nPut visit details at the end.',
  },
  loadFailed: 'Could not load the purpose list.',
  noPurpose: 'None',
  create: {
    name: 'Purpose name',
    namePlaceholder: 'For example: Informational restaurant review',
    description: 'What kind of post is this?',
    descriptionPlaceholder: 'For example: A hosted restaurant visit review',
    instructions: 'Writing guidelines',
    instructionsPlaceholder:
      'For example: Describe every photo.\nDo not write it like a diary.\nPut visit details at the end.',
    help: 'A purpose shapes the content and structure. Style and endings still follow the voice profile.',
    submit: 'Create purpose',
  },
  emptyDescription: 'No description',
  delete: {
    aria: 'Delete {{name}}',
    title: 'Delete this purpose?',
    description:
      'This will delete “{{name}}”. {{detach}} Existing results and active jobs stay unchanged.',
  },
  assignment: {
    runningJob:
      'The active AI job will finish with the purpose it started with. Your change applies to the next generation.',
    notFound: 'The selected purpose could not be found. Refresh the list and try again.',
    notFoundDetail:
      'The selected purpose could not be found. Refresh the list and try again. {{error}}',
    failed: 'Could not change the purpose. Try again.',
    manage: 'Manage purposes',
  },
  postCount: '{{count}} posts',
  postCount_one: '{{count}} post',
  postCount_other: '{{count}} posts',
  detachWarning: {
    used: 'The purpose will be removed from {{count}} posts. Their content will remain unchanged.',
    used_one: 'The purpose will be removed from {{count}} post. Its content will remain unchanged.',
    used_other:
      'The purpose will be removed from {{count}} posts. Their content will remain unchanged.',
    unused: 'No posts use this purpose.',
  },
} as const
