import { auth as enAuth } from './en/auth'
import { common as enCommon } from './en/common'
import { errors as enErrors } from './en/errors'
import { guidelines as enGuidelines } from './en/guidelines'
import { marketing as enMarketing } from './en/marketing'
import { models as enModels } from './en/models'
import { nav as enNav } from './en/nav'
import { plans as enPlans } from './en/plans'
import { posts as enPosts } from './en/posts'
import { publishing as enPublishing } from './en/publishing'
import { templates as enTemplates } from './en/templates'
import { voices as enVoices } from './en/voices'
import { auth as koAuth } from './ko/auth'
import { common as koCommon } from './ko/common'
import { errors as koErrors } from './ko/errors'
import { guidelines as koGuidelines } from './ko/guidelines'
import { marketing as koMarketing } from './ko/marketing'
import { models as koModels } from './ko/models'
import { nav as koNav } from './ko/nav'
import { plans as koPlans } from './ko/plans'
import { posts as koPosts } from './ko/posts'
import { publishing as koPublishing } from './ko/publishing'
import { templates as koTemplates } from './ko/templates'
import { voices as koVoices } from './ko/voices'

export const defaultNS = 'common' as const

export const RESOURCE_NAMESPACES = [
  'common',
  'auth',
  'nav',
  'posts',
  'voices',
  'templates',
  'guidelines',
  'models',
  'publishing',
  'errors',
  'marketing',
  'plans',
] as const

export const resources = {
  ko: {
    common: koCommon,
    auth: koAuth,
    nav: koNav,
    posts: koPosts,
    voices: koVoices,
    templates: koTemplates,
    guidelines: koGuidelines,
    models: koModels,
    publishing: koPublishing,
    errors: koErrors,
    marketing: koMarketing,
    plans: koPlans,
  },
  en: {
    common: enCommon,
    auth: enAuth,
    nav: enNav,
    posts: enPosts,
    voices: enVoices,
    templates: enTemplates,
    guidelines: enGuidelines,
    models: enModels,
    publishing: enPublishing,
    errors: enErrors,
    marketing: enMarketing,
    plans: enPlans,
  },
} as const
