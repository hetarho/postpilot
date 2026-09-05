// Query keys and the proto→domain mappers for the guideline entity.
import type { Transport } from '@connectrpc/connect'
import {
  ProtoGuidelineScope,
  type ProtoGuideline,
  type ProtoGuidelineCandidate,
  type ProtoGuidelineTemplateRef,
} from '@/shared/api'
import type {
  Guideline,
  GuidelineCandidate,
  GuidelineScope,
  GuidelineScopeKind,
} from '../model/types'

function toScopeKind(scope: ProtoGuidelineScope): GuidelineScopeKind {
  // An unset scope reads as `templates` with no links rather than `global`: the one shape that
  // must never be guessed is the one that would apply a rule to every post of the account.
  return scope === ProtoGuidelineScope.GLOBAL ? 'global' : 'templates'
}

export function fromScopeKind(kind: GuidelineScopeKind): ProtoGuidelineScope {
  return kind === 'global' ? ProtoGuidelineScope.GLOBAL : ProtoGuidelineScope.TEMPLATES
}

function toTemplateRef(ref: ProtoGuidelineTemplateRef) {
  return { id: ref.id, name: ref.name }
}

export function toGuideline(guideline: ProtoGuideline): Guideline {
  return {
    id: guideline.id,
    text: guideline.text,
    scope: toScopeKind(guideline.scope),
    templates: guideline.templates.map(toTemplateRef),
    createdAt: guideline.createdAt,
    updatedAt: guideline.updatedAt,
  }
}

export function toGuidelineCandidate(candidate: ProtoGuidelineCandidate): GuidelineCandidate {
  return {
    id: candidate.id,
    text: candidate.text,
    postSlug: candidate.postSlug,
    occurrences: candidate.occurrences,
    firstSeenAt: candidate.firstSeenAt,
    lastSeenAt: candidate.lastSeenAt,
  }
}

/** The wire form of a whole scope, used by both the create request and the update patch. */
export function toScopePatch(scope: GuidelineScope) {
  return { scope: fromScopeKind(scope.kind), templateIds: scope.templateIds }
}

/** Per account, like the template directory: an account switch on the same device must never read
 *  the previous account's rules. */
export function guidelinesQueryKey(transport: Transport, ownerId: string) {
  return ['guidelines', transport, ownerId] as const
}

/** Per account for the same reason, and a root of its own rather than a child of the saved list's
 *  key: the two lists are invalidated together on purpose (an approval moves a row from one to the
 *  other), and a shared prefix would make that coupling implicit instead. */
export function guidelineCandidatesQueryKey(transport: Transport, ownerId: string) {
  return ['guideline-candidates', transport, ownerId] as const
}
