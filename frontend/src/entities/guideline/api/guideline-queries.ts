// Query keys and the proto→domain mappers for the guideline entity.
import type { Transport } from '@connectrpc/connect'
import {
  ProtoGuidelineScope,
  type ProtoGuideline,
  type ProtoGuidelineTemplateRef,
} from '@/shared/api'
import type { Guideline, GuidelineScope, GuidelineScopeKind } from '../model/types'

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

/** The wire form of a whole scope, used by both the create request and the update patch. */
export function toScopePatch(scope: GuidelineScope) {
  return { scope: fromScopeKind(scope.kind), templateIds: scope.templateIds }
}

/** Per account, like the template directory: an account switch on the same device must never read
 *  the previous account's rules. */
export function guidelinesQueryKey(transport: Transport, ownerId: string) {
  return ['guidelines', transport, ownerId] as const
}
