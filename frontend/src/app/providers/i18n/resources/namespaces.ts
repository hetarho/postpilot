// The namespace list lives apart from the assembly in `index.ts` so that `../configure.ts` can
// name it without evaluating the catalog modules; `index.ts` re-exports both unchanged.

export const defaultNS = 'common' as const

export const RESOURCE_NAMESPACES = [
  'requestInspection',
  'authoring',
  'voicePreparation',
  'channel',
  'creation',
  'spokenVoice',
  'createSpokenVoice',
  'manageSpokenVoice',
  'spokenCreation',
  'spokenVoices',
  'common',
  'auth',
  'nav',
  'writingTests',
  'voiceMaterialEdit',
  'posts',
  'voices',
  'clipWizard',
  'templateTargets',
  'templates',
  'guidelines',
  'memories',
  'models',
  'errors',
  'marketing',
  'plans',
  'billing',
  'clips',
] as const
