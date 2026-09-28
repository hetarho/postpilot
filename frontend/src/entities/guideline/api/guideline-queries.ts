// Query keys and the proto→domain mappers for the guideline entity.
import type { Transport } from '@connectrpc/connect'
import {
  blogFieldToProto,
  requireBlogFieldId,
  type BlogFieldId,
} from '@/entities/blog-field/@x/guideline'
import {
  ProtoGuidelineKind,
  ProtoGuidelineScope,
  type ProtoBlogField,
  type ProtoDefaultGuideline,
  type ProtoGuideline,
  type ProtoGuidelineCandidate,
  type ProtoGuidelineTemplateRef,
} from '@/shared/api'
import type { Locale } from '@/shared/lib'
import type {
  DefaultGuideline,
  DefaultGuidelineEntry,
  Guideline,
  GuidelineCandidate,
  GuidelineKind,
  GuidelineScope,
  GuidelineScopeKind,
} from '../model/types'

const SCOPE_TO_PROTO: Record<GuidelineScopeKind, ProtoGuidelineScope> = {
  global: ProtoGuidelineScope.GLOBAL,
  templates: ProtoGuidelineScope.TEMPLATES,
  fields: ProtoGuidelineScope.FIELDS,
}

/** Undefined for UNSPECIFIED and for a number this build does not know (ARCH-3). No scope is ever
 *  guessed: a guessed one would misstate which posts a rule reaches (GUIDE-14). */
export function toScopeKind(scope: ProtoGuidelineScope): GuidelineScopeKind | undefined {
  for (const [kind, wire] of Object.entries(SCOPE_TO_PROTO) as [
    GuidelineScopeKind,
    ProtoGuidelineScope,
  ][]) {
    if (wire === scope) return kind
  }
  return undefined
}

export function fromScopeKind(kind: GuidelineScopeKind): ProtoGuidelineScope {
  return SCOPE_TO_PROTO[kind]
}

function toTemplateRef(ref: ProtoGuidelineTemplateRef) {
  return { id: ref.id, name: ref.name }
}

/** Every entry names a 분야: 없음 or a number this build does not know fails the read (ARCH-3),
 *  and the page shows its load failure with a retry, as for an unknown scope. Dropping one would
 *  have the next whole-set save erase it on the server. */
function toFieldIds(fields: readonly ProtoBlogField[]): BlogFieldId[] {
  return fields.map(requireBlogFieldId)
}

/** Throws on a scope it cannot read, which fails the list read: the page then says so and offers a
 *  retry rather than listing a rule under a scope it may not have. */
export function toGuideline(guideline: ProtoGuideline): Guideline {
  const scope = toScopeKind(guideline.scope)
  if (!scope) throw new Error(`unsupported guideline scope enum: ${String(guideline.scope)}`)
  return {
    id: guideline.id,
    kind: guideline.kind === ProtoGuidelineKind.CLIP ? 'clip' : 'post',
    text: guideline.text,
    scope,
    templates: guideline.templates.map(toTemplateRef),
    fields: toFieldIds(guideline.fields),
    createdAt: guideline.createdAt,
    updatedAt: guideline.updatedAt,
  }
}

const KIND_TO_PROTO: Record<GuidelineKind, ProtoGuidelineKind> = {
  post: ProtoGuidelineKind.POST,
  clip: ProtoGuidelineKind.CLIP,
}

export function fromGuidelineKind(kind: GuidelineKind): ProtoGuidelineKind {
  return KIND_TO_PROTO[kind]
}

export function toDefaultGuidelineEntry(value: ProtoDefaultGuideline): DefaultGuidelineEntry {
  return {
    key: value.key,
    enabled: value.enabled,
    ko: { name: value.ko?.name ?? '', text: value.ko?.text ?? '' },
    en: { name: value.en?.name ?? '', text: value.en?.text ?? '' },
    koreanTargetOnly: value.koreanTargetOnly,
    memoriesOnly: value.memoriesOnly,
  }
}

/** The copy the UI language reads; the server sends both (GUIDE-19). */
export function localizeDefaultGuideline(
  entry: DefaultGuidelineEntry,
  locale: Locale,
): DefaultGuideline {
  const copy = locale === 'en' ? entry.en : entry.ko
  return {
    key: entry.key,
    enabled: entry.enabled,
    name: copy.name,
    text: copy.text,
    koreanTargetOnly: entry.koreanTargetOnly,
    memoriesOnly: entry.memoriesOnly,
  }
}

export function toGuidelineCandidate(candidate: ProtoGuidelineCandidate): GuidelineCandidate {
  return {
    id: candidate.id,
    kind: candidate.kind === ProtoGuidelineKind.CLIP ? 'clip' : 'post',
    text: candidate.text,
    postSlug: candidate.postSlug,
    clipId: candidate.clipId,
    occurrences: candidate.occurrences,
    firstSeenAt: candidate.firstSeenAt,
    lastSeenAt: candidate.lastSeenAt,
  }
}

/** The wire form of a whole scope, used by both the create request and the update patch. */
export function toScopePatch(scope: GuidelineScope) {
  return {
    scope: fromScopeKind(scope.kind),
    templateIds: scope.templateIds,
    fields: scope.fields.map(blogFieldToProto),
  }
}

/** Per account, like the template directory: an account switch on the same device must never read
 *  the previous account's rules. */
export function guidelinesQueryKey(transport: Transport, ownerId: string) {
  return ['guidelines', transport, ownerId] as const
}

/** The list of one kind: a post's 지침 under the key it always had, a clip's 영상 지침 under a root
 *  of its own, so a post-template rename refreshes the one list whose chips name it (GUIDE-44). */
export function guidelineKindQueryKey(transport: Transport, ownerId: string, kind: GuidelineKind) {
  return kind === 'post'
    ? guidelinesQueryKey(transport, ownerId)
    : (['video-guidelines', transport, ownerId] as const)
}

/** The candidate queue of one kind, keyed like the list. */
export function guidelineCandidatesKindQueryKey(
  transport: Transport,
  ownerId: string,
  kind: GuidelineKind,
) {
  return kind === 'post'
    ? guidelineCandidatesQueryKey(transport, ownerId)
    : (['video-guideline-candidates', transport, ownerId] as const)
}

/** Per account for the same reason, and a root of its own rather than a child of the saved list's
 *  key: the two lists are invalidated together on purpose (an approval moves a row from one to the
 *  other), and a shared prefix would make that coupling implicit instead. */
export function guidelineCandidatesQueryKey(transport: Transport, ownerId: string) {
  return ['guideline-candidates', transport, ownerId] as const
}
