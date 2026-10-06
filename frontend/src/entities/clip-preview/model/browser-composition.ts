import {
  CLIP_BROWSER_COMPOSITION,
  CLIP_BROWSER_FONTS,
  CLIP_BROWSER_STATIC_CAPTIONS,
  CLIP_BROWSER_RENDER,
  CLIP_COMPOSITION_LIMITS,
  CLIP_DESIGN,
  type ClipRatioId,
} from '@/entities/clip-design/@x/clip-preview'
import {
  copyClipPlan,
  cutOutputMs,
  cutRate,
  sourceAudioEnabled,
  spokenState,
  textInterval,
  type ClipEditPlan,
  type ClipEditableText,
  type ClipSpeechRef,
} from '@/entities/clip-plan/@x/clip-preview'
import { frameLayers, frameTimeline, previewCrop, previewTimeline } from './draft-preview'
import { speechRenderFingerprint } from './speech-fingerprint'

export type BrowserFrozen<T> = T extends readonly (infer U)[]
  ? readonly BrowserFrozen<U>[]
  : T extends object
    ? { readonly [K in keyof T]: BrowserFrozen<T[K]> }
    : T

/** Authorized manifest data, with no storage key, fetch target or decoded resource. */
export interface BrowserSourceIdentity {
  sourceId: string
  fingerprint: string
  durationMs: number
  width: number
  height: number
  hasAudio: boolean
  allowedRatePermille: readonly number[]
}
export interface BrowserCompositionVersions {
  renderer: string
  components: string
  fonts: string
  assets: string
}
export interface BrowserCompositionDesign {
  captionStyles: readonly string[]
  captionPace: string
  accent: string
  introPreset: string
  outroPreset: string
  disclosure: string
  hideDisclosure: boolean
}
export interface BrowserCompositionComponent {
  instanceId: string
  componentId: string
  componentVersion: string
  element: ClipEditableText
  startMs: number
  endMs: number
  firstFrame: number
  endFrame: number
  phraseIndex?: number
  sequence: boolean
}
export interface BrowserCompositionInput {
  ownerId: string
  projectId: string
  projectRevision: number
  planRevision: number
  plan: ClipEditPlan
  ratio: ClipRatioId
  sources: readonly BrowserSourceIdentity[]
  design?: Partial<BrowserCompositionDesign>
  versions?: BrowserCompositionVersions
  /** Opaque server-authored admission binding; the local hash never authorizes publication. */
  authoritativeFingerprint?: string
}
export type BrowserCompositionSnapshot = BrowserFrozen<{
  schemaVersion: 1
  ownerId: string
  projectId: string
  projectRevision: number
  planRevision: number
  plan: ClipEditPlan
  ratio: ClipRatioId
  versions: BrowserCompositionVersions
  design: BrowserCompositionDesign
  fonts: readonly { id: string; sha256: string }[]
  sources: readonly BrowserSourceIdentity[]
  components: readonly BrowserCompositionComponent[]
  speechFingerprint: string
  snapshotFingerprint: string
  authoritativeFingerprint?: string
  frameCount: number
}>

export class BrowserCompositionError extends Error {
  constructor(
    public readonly code: string,
    public readonly subject = '',
  ) {
    super(subject ? `${code}:${subject}` : code)
  }
}
function refuse(code: string, subject = ''): never {
  throw new BrowserCompositionError(code, subject)
}
const positiveInteger = (n: number) => Number.isSafeInteger(n) && n > 0
const identity = (s: string) => typeof s === 'string' && s.length > 0 && s.length <= 256
const versions = (): BrowserCompositionVersions => ({
  renderer: CLIP_BROWSER_COMPOSITION.renderer,
  components: CLIP_BROWSER_COMPOSITION.components,
  fonts: CLIP_BROWSER_COMPOSITION.fonts,
  assets: CLIP_BROWSER_COMPOSITION.assets,
})
const domainKeys = new Set(
  `ownerId projectId projectRevision planRevision plan ratio sources design versions schemaVersion
  fonts components speechFingerprint snapshotFingerprint authoritativeFingerprint frameCount renderer assets captionStyles captionPace introPreset outroPreset disclosure hideDisclosure
  sourceId fingerprint durationMs width height hasAudio allowedRatePermille id sha256 componentId componentVersion element
  instanceId firstFrame endFrame phraseIndex sequence nativeComposition sourceAudio sourceVolumePermille associations elements cuts narration
  focal x y startMs endMs transitionMs copies volumePermille playbackRatePermille pace text anchor align keyword style accent
  retainOriginalAudio groupId itemId derivedCaption segmentId textRevision textEdited timingEdited ownerEdited
  effectiveStartMs effectiveEndMs phrases staleEvidence evidenceReviewed evidence fallbackReason elementId cutId kind role rows
  position basis resolvedStartMs resolvedEndMs ownerPosition ownerSizePx ownerStyle enabled confirmedVoiceId bindingDigest segments
  inputHash speech assetId voiceId settingsHash audioHash profileId profileRevision samples sampleRate channels timing`.split(
    /\s+/u,
  ),
)

/** Reject executable/object resources rather than losing them silently in JSON. Text is data. */
function plainData(value: unknown, depth = 0): void {
  if (depth > 16) refuse('CLIP_SNAPSHOT_INVALID')
  if (value === undefined || value === null || typeof value === 'boolean') return
  if (typeof value === 'string') {
    if (value.length > CLIP_COMPOSITION_LIMITS.sourceChars) refuse('CLIP_SNAPSHOT_INVALID')
    return
  }
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) refuse('CLIP_SNAPSHOT_INVALID')
    return
  }
  if (Array.isArray(value)) {
    if (value.length > CLIP_COMPOSITION_LIMITS.cues) refuse('CLIP_SNAPSHOT_INVALID')
    value.forEach((v) => plainData(v, depth + 1))
    return
  }
  if (typeof value !== 'object' || Object.getPrototypeOf(value) !== Object.prototype)
    refuse('CLIP_SNAPSHOT_INVALID')
  for (const [key, descriptor] of Object.entries(Object.getOwnPropertyDescriptors(value))) {
    if (!('value' in descriptor)) refuse('CLIP_SNAPSHOT_NON_DOMAIN_DATA', key)
    const item: unknown = descriptor.value
    if (
      /^(?:url|.*Url|.*URL|html|css|shader|javascript|code|png|bitmap|bytes|data|creation|refreshDerivedCaptions|__proto__|constructor|prototype)$/u.test(
        key,
      )
    )
      refuse('CLIP_SNAPSHOT_NON_DOMAIN_DATA', key)
    if (!domainKeys.has(key)) refuse('CLIP_SNAPSHOT_NON_DOMAIN_DATA', key)
    plainData(item, depth + 1)
  }
}
function freeze<T>(value: T): BrowserFrozen<T> {
  if (value && typeof value === 'object') {
    Object.values(value).forEach((child) => freeze(child))
    Object.freeze(value)
  }
  return value as BrowserFrozen<T>
}
function canonical(data: unknown): string {
  return JSON.stringify(data, (_key, item: unknown) =>
    item && typeof item === 'object' && !Array.isArray(item)
      ? Object.fromEntries(Object.entries(item).sort(([a], [b]) => a.localeCompare(b)))
      : item,
  )
}
function compatible(v: BrowserCompositionVersions): void {
  const known = versions()
  for (const key of Object.keys(known) as (keyof BrowserCompositionVersions)[])
    if (v[key] !== known[key]) refuse('CLIP_SNAPSHOT_INCOMPATIBLE_VERSION', key)
}
function designOf(input?: Partial<BrowserCompositionDesign>): BrowserCompositionDesign {
  const design = {
    captionStyles: [...(input?.captionStyles?.length ? input.captionStyles : ['bold'])],
    captionPace: input?.captionPace || 'steady',
    accent: input?.accent ?? '',
    // Native UnchosenDesign: absent legacy selections retain intro B/outro E.
    introPreset: input?.introPreset || 'b',
    outroPreset: input?.outroPreset || 'e',
    disclosure: input?.disclosure ?? 'self',
    hideDisclosure: input?.hideDisclosure ?? false,
  }
  if (!design.hideDisclosure && !input?.disclosure) refuse('CLIP_SNAPSHOT_INVALID', 'disclosure')
  if (
    !['steady', 'rapid'].includes(design.captionPace) ||
    (design.accent !== '' && !Object.hasOwn(CLIP_DESIGN.accent, design.accent))
  )
    refuse('CLIP_SNAPSHOT_INVALID', 'design')
  if (
    !design.captionStyles.length ||
    design.captionStyles.some((id) => !Object.hasOwn(CLIP_DESIGN.regions.caption, id))
  )
    refuse('CLIP_SNAPSHOT_UNKNOWN_COMPONENT', 'caption')
  if (!Object.hasOwn(CLIP_DESIGN.regions.intro, design.introPreset))
    refuse('CLIP_SNAPSHOT_UNKNOWN_COMPONENT', 'intro')
  if (!Object.hasOwn(CLIP_DESIGN.regions.outro, design.outroPreset))
    refuse('CLIP_SNAPSHOT_UNKNOWN_COMPONENT', 'outro')
  if (!Object.hasOwn(CLIP_DESIGN.disclosure, design.disclosure))
    refuse('CLIP_SNAPSHOT_UNKNOWN_COMPONENT', 'disclosure')
  return design
}
function componentID(element: ClipEditableText, design: BrowserCompositionDesign): string {
  switch (element.role) {
    case 'caption': {
      const style =
        element.ownerStyle ||
        (element.style && element.style !== 'auto' ? element.style : design.captionStyles[0]!)
      if (!Object.hasOwn(CLIP_DESIGN.regions.caption, style))
        refuse('CLIP_SNAPSHOT_UNKNOWN_COMPONENT', style)
      return `caption/${style}`
    }
    case 'hook':
      return `intro/${design.introPreset}`
    case 'ending':
      return `outro/${design.outroPreset}`
    case 'info':
      return 'region/info'
    case 'badge':
      return 'region/badge'
    case 'disclosure':
      return `disclosure/${design.disclosure}`
    default:
      return refuse('CLIP_SNAPSHOT_UNKNOWN_COMPONENT', element.role)
  }
}
function componentsOf(
  plan: ClipEditPlan,
  design: BrowserCompositionDesign,
): BrowserCompositionComponent[] {
  const fps = CLIP_BROWSER_RENDER.frameRate
  const elements: ClipEditableText[] = [...(plan.elements ?? [])]
  // Old, ordinary plans remain readable without inventing a second editable timeline.
  if (!plan.nativeComposition && !elements.length) {
    for (const item of previewTimeline(plan))
      item.cut.copies.forEach((copy, index) => {
        const defaultWindow = copy.startMs === 0 && copy.endMs === 0
        elements.push({
          instanceId: `${item.cut.id}/copy/${index}`,
          elementId: 'caption',
          cutId: item.cut.id,
          kind: 'fixed',
          role: 'caption',
          text: copy.text,
          rows: [],
          style: copy.style,
          position: copy.anchor,
          align: copy.align,
          basis: 'cut',
          startMs: defaultWindow ? CLIP_DESIGN.timing.copy_lead_ms : copy.startMs,
          endMs: defaultWindow
            ? cutOutputMs(item.cut) - CLIP_DESIGN.timing.copy_lead_ms
            : copy.endMs,
          pace: copy.pace ?? 'steady',
          accent: copy.accent,
          keyword: copy.keyword,
          resolvedStartMs: 0,
          resolvedEndMs: 0,
          groupId: '',
          itemId: '',
        })
      })
  }
  if (!design.hideDisclosure)
    elements.push({
      instanceId: 'product/disclosure',
      elementId: 'disclosure',
      cutId: '',
      kind: 'fixed',
      role: 'disclosure',
      text: CLIP_DESIGN.disclosure[design.disclosure as keyof typeof CLIP_DESIGN.disclosure],
      rows: [],
      style: '',
      position: 'header',
      align: 'right',
      basis: 'whole',
      pace: 'steady',
      accent: '',
      keyword: '',
      resolvedStartMs: 0,
      resolvedEndMs: plan.durationMs,
      groupId: '',
      itemId: '',
    })
  const ids = new Set<string>()
  return elements.flatMap((element) => {
    if (!identity(element.instanceId) || ids.has(element.instanceId))
      refuse('CLIP_SNAPSHOT_INVALID', 'component identity')
    ids.add(element.instanceId)
    const window = textInterval(plan, element)
    if (!window.valid) refuse('CLIP_SNAPSHOT_INVALID', element.instanceId)
    const componentId = componentID(element, design)
    if (
      element.ownerPosition &&
      (!Number.isFinite(element.ownerPosition.x) || !Number.isFinite(element.ownerPosition.y))
    )
      refuse('CLIP_SNAPSHOT_INVALID', element.instanceId)
    if (element.ownerSizePx !== undefined && !(element.ownerSizePx > 0))
      refuse('CLIP_SNAPSHOT_INVALID', element.instanceId)
    const phrases = element.pace === 'rapid' ? element.phrases : undefined
    const windows = phrases?.length
      ? phrases.map((p, phraseIndex) => ({
          startMs: window.cutOffsetMs + p.startMs,
          endMs: window.cutOffsetMs + p.endMs,
          phraseIndex,
        }))
      : [{ startMs: window.startMs, endMs: window.endMs }]
    let previous = window.startMs
    return windows.map((interval) => {
      if (
        !Number.isSafeInteger(interval.startMs) ||
        !Number.isSafeInteger(interval.endMs) ||
        interval.startMs < previous ||
        interval.endMs <= interval.startMs ||
        interval.endMs > window.endMs
      )
        refuse('CLIP_SNAPSHOT_INVALID', element.instanceId)
      previous = interval.endMs
      return {
        instanceId: element.instanceId,
        componentId,
        componentVersion: CLIP_BROWSER_COMPOSITION.components,
        sequence:
          element.role === 'caption' &&
          element.pace !== 'rapid' &&
          !(CLIP_BROWSER_STATIC_CAPTIONS as readonly string[]).includes(
            componentId.slice('caption/'.length),
          ),
        element,
        ...interval,
        firstFrame: Math.floor((interval.startMs * fps) / 1000),
        endFrame: Math.ceil((interval.endMs * fps) / 1000),
      }
    })
  })
}

/** A page-owned run freezes a saved plan; URLs and resources are resolved separately. */
export async function freezeBrowserComposition(
  input: BrowserCompositionInput,
): Promise<BrowserCompositionSnapshot> {
  plainData(input)
  if (!input || !input.plan || !Array.isArray(input.plan.cuts) || !Array.isArray(input.sources))
    refuse('CLIP_SNAPSHOT_INVALID')
  if (
    input.plan.cuts.some((cut) => !cut || !Array.isArray(cut.copies)) ||
    input.sources.some((source) => !source || !Array.isArray(source.allowedRatePermille)) ||
    (input.plan.elements !== undefined &&
      (!Array.isArray(input.plan.elements) ||
        input.plan.elements.some(
          (element) => !element || !Array.isArray(element.rows) || typeof element.text !== 'string',
        ))) ||
    (input.plan.narration &&
      (!Array.isArray(input.plan.narration.segments) ||
        input.plan.narration.segments.some(
          (segment) => !segment || (segment.speech && !Array.isArray(segment.speech.timing)),
        )))
  )
    refuse('CLIP_SNAPSHOT_INVALID')
  const { ownerId, projectId, projectRevision, planRevision, ratio, authoritativeFingerprint } =
    input
  if (authoritativeFingerprint !== undefined && !/^[a-f0-9]{64}$/u.test(authoritativeFingerprint))
    refuse('CLIP_SNAPSHOT_INVALID', 'admission')
  if (
    !identity(input.ownerId) ||
    !identity(input.projectId) ||
    !positiveInteger(input.projectRevision) ||
    !positiveInteger(input.planRevision)
  )
    refuse('CLIP_SNAPSHOT_INVALID', 'revision')
  if (!Object.hasOwn(CLIP_DESIGN.ratios, input.ratio)) refuse('CLIP_SNAPSHOT_INVALID', 'ratio')
  const selectedVersions = { ...(input.versions ?? versions()) }
  compatible(selectedVersions)
  const plan = copyClipPlan(JSON.parse(JSON.stringify(input.plan)) as ClipEditPlan)
  const sources = input.sources.map((source) => ({
    ...source,
    allowedRatePermille: [...source.allowedRatePermille],
  }))
  if (
    !plan.cuts.length ||
    plan.cuts.length > CLIP_COMPOSITION_LIMITS.cuts ||
    !positiveInteger(plan.durationMs) ||
    plan.durationMs > CLIP_COMPOSITION_LIMITS.maxDurationMs
  )
    refuse('CLIP_SNAPSHOT_INVALID', 'timeline')
  const ids = new Set<string>()
  for (const source of sources) {
    if (
      !identity(source.sourceId) ||
      !/^[a-f0-9]{64}$/u.test(source.fingerprint) ||
      !positiveInteger(source.durationMs) ||
      !positiveInteger(source.width) ||
      !positiveInteger(source.height) ||
      source.width > 16384 ||
      source.height > 16384 ||
      typeof source.hasAudio !== 'boolean' ||
      sources.filter((s) => s.sourceId === source.sourceId).length !== 1
    )
      refuse('CLIP_SNAPSHOT_INVALID', 'source')
  }
  for (const cut of plan.cuts) {
    const source = sources.find(
      (s) => s.sourceId === cut.sourceId && s.fingerprint === cut.fingerprint,
    )
    if (
      !identity(cut.id) ||
      ids.has(cut.id) ||
      !source ||
      cut.endMs > source.durationMs ||
      !source.allowedRatePermille.includes(cutRate(cut))
    )
      refuse('CLIP_SNAPSHOT_INVALID', cut.id)
    ids.add(cut.id)
    if (
      !Number.isSafeInteger(cut.volumePermille) ||
      cut.volumePermille < 0 ||
      cut.volumePermille > 1000 ||
      (cut.focal &&
        (typeof cut.focal.x !== 'number' ||
          typeof cut.focal.y !== 'number' ||
          cut.focal.x < 0 ||
          cut.focal.x > 1 ||
          cut.focal.y < 0 ||
          cut.focal.y > 1))
    )
      refuse('CLIP_SNAPSHOT_INVALID', cut.id)
  }
  if (
    plan.cuts[0]!.transitionMs !== 0 ||
    !previewTimeline(plan).length ||
    previewTimeline(plan).at(-1)!.endMs !== plan.durationMs
  )
    refuse('CLIP_SNAPSHOT_INVALID', 'timeline')
  if (
    plan.sourceAudio?.some(
      (setting) =>
        !sources.some(
          (s) =>
            s.sourceId === setting.sourceId &&
            s.fingerprint === setting.fingerprint &&
            typeof setting.retainOriginalAudio === 'boolean',
        ),
    )
  )
    refuse('CLIP_SNAPSHOT_INVALID', 'source audio')
  const gain = plan.sourceVolumePermille ?? 1000
  if (!Number.isSafeInteger(gain) || gain < 0 || gain > 1000)
    refuse('CLIP_SNAPSHOT_INVALID', 'volume')
  if (plan.narration?.enabled) {
    const narration = plan.narration
    if (
      !Number.isSafeInteger(narration.volumePermille) ||
      narration.volumePermille < 0 ||
      narration.volumePermille > 1000 ||
      !narration.segments.length
    )
      refuse('CLIP_SNAPSHOT_INVALID', 'speech')
    for (const segment of narration.segments) {
      const speech = segment.speech
      if (
        spokenState(narration, segment, plan.durationMs) !== 'ready' ||
        !speech ||
        !identity(speech.assetId) ||
        !identity(speech.settingsHash) ||
        !identity(speech.audioHash) ||
        !identity(speech.voiceId) ||
        !identity(speech.bindingDigest) ||
        !identity(speech.inputHash) ||
        !identity(speech.profileId) ||
        !positiveInteger(speech.profileRevision) ||
        !positiveInteger(speech.samples) ||
        !positiveInteger(speech.sampleRate) ||
        ![1, 2].includes(speech.channels)
      )
        refuse('CLIP_SNAPSHOT_INVALID', segment.id)
      if (
        narration.segments.filter((s) => s.id === segment.id).length !== 1 ||
        speech.timing.some(
          (t) =>
            !Number.isSafeInteger(t.startMs) ||
            !Number.isSafeInteger(t.endMs) ||
            t.startMs < 0 ||
            t.endMs <= t.startMs ||
            t.endMs > Math.ceil((speech.samples * 1000) / speech.sampleRate),
        )
      )
        refuse('CLIP_SNAPSHOT_INVALID', segment.id)
    }
  }
  const design = designOf(input.design)
  const components = componentsOf(plan, design)
  const speechFingerprint = await speechRenderFingerprint(plan)
  const snapshot = {
    schemaVersion: 1 as const,
    ownerId,
    projectId,
    projectRevision,
    planRevision,
    plan,
    ratio,
    versions: selectedVersions,
    design,
    fonts: CLIP_BROWSER_FONTS.map((f) => ({ ...f })),
    sources,
    components,
    speechFingerprint,
    frameCount: frameTimeline(plan, CLIP_BROWSER_RENDER.frameRate).total,
    ...(authoritativeFingerprint ? { authoritativeFingerprint } : {}),
  }
  const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(canonical(snapshot)))
  const snapshotFingerprint = Array.from(new Uint8Array(hash), (v) =>
    v.toString(16).padStart(2, '0'),
  ).join('')
  return freeze({ ...snapshot, snapshotFingerprint })
}

/** Validate durable identity before reusing a serialized snapshot from another run/version. */
export async function readBrowserCompositionSnapshot(
  value: unknown,
): Promise<BrowserCompositionSnapshot> {
  plainData(value)
  const saved = value as BrowserCompositionSnapshot
  if (!saved || saved.schemaVersion !== CLIP_BROWSER_COMPOSITION.schemaVersion || !saved.versions)
    refuse('CLIP_SNAPSHOT_INCOMPATIBLE_VERSION', 'schema')
  compatible(saved.versions)
  const rebuilt = await freezeBrowserComposition(saved as unknown as BrowserCompositionInput)
  if (canonical(saved) !== canonical(rebuilt)) refuse('CLIP_SNAPSHOT_INVALID', 'derived contract')
  return rebuilt
}

/** Every drawing/decoding instant comes from this output frame, never wall time. */
export function evaluateBrowserFrame(snapshot: BrowserCompositionSnapshot, frame: number) {
  if (snapshot.schemaVersion !== CLIP_BROWSER_COMPOSITION.schemaVersion)
    refuse('CLIP_SNAPSHOT_INCOMPATIBLE_VERSION', 'schema')
  if (!Number.isSafeInteger(frame) || frame < 0 || frame >= snapshot.frameCount)
    refuse('CLIP_SNAPSHOT_INVALID', 'frame')
  compatible(snapshot.versions)
  const fps = CLIP_BROWSER_RENDER.frameRate
  const plan = snapshot.plan as ClipEditPlan // Readers only; deeply frozen by the constructor.
  const timeline = frameTimeline(plan, fps)
  const canvas = CLIP_DESIGN.ratios[snapshot.ratio].canvas
  const timestampUs = Math.round((frame * 1_000_000) / fps)
  const footageLayers = frameLayers(timeline, frame).map((layer) => {
    const source = snapshot.sources.find(
      (s) => s.sourceId === layer.cut.sourceId && s.fingerprint === layer.cut.fingerprint,
    )!
    const sourceEndUs = layer.cut.endMs * 1000
    return {
      cutInstanceId: layer.cut.id,
      sourceId: source.sourceId,
      fingerprint: source.fingerprint,
      sourceTimestampUs: Math.max(
        layer.cut.startMs * 1000,
        Math.min(
          sourceEndUs - 1,
          layer.cut.startMs * 1000 +
            Math.round(
              ((frame - timeline.cuts[layer.index]!.startFrame) * cutRate(layer.cut) * 1000) / fps,
            ),
        ),
      ),
      legacySourceMs: layer.sourceMs,
      sourceStartUs: layer.cut.startMs * 1000,
      sourceEndUs,
      cutStartFrame: timeline.cuts[layer.index]!.startFrame,
      cutLocalFrame: frame - timeline.cuts[layer.index]!.startFrame,
      ratePermille: cutRate(layer.cut),
      focal: layer.cut.focal ?? { x: 0.5, y: 0.5 },
      alpha: layer.alpha,
      weight: layer.weight,
      retainOriginalAudio: source.hasAudio && sourceAudioEnabled(plan, layer.cut),
      crop: previewCrop(source.width, source.height, canvas.width, canvas.height, layer.cut.focal),
    }
  })
  if (footageLayers.length < 1 || footageLayers.length > 2)
    refuse('CLIP_SNAPSHOT_INVALID', 'active layers')
  const components = snapshot.components
    .filter((c) =>
      c.sequence
        ? frame >= c.firstFrame && frame < c.endFrame
        : (frame * 1000) / fps >= c.startMs && (frame * 1000) / fps < c.endMs,
    )
    .map((component) => {
      const localFrame = frame - component.firstFrame
      const frames = component.endFrame - component.firstFrame
      const progress = frames > 1 ? localFrame / (frames - 1) : 0
      return {
        component,
        localFrame,
        localTimeMs: (frame * 1000) / fps - component.startMs,
        durationMs: component.endMs - component.startMs,
        progress,
        animationProgress: component.element.pace === 'rapid' ? 0.5 : progress,
        phraseIndex: component.phraseIndex,
        text:
          component.phraseIndex !== undefined
            ? component.element.phrases![component.phraseIndex]!.text
            : component.element.text,
      }
    })
  return {
    frame,
    timeMs: (frame * 1000) / fps,
    timestampUs,
    durationUs: Math.round(((frame + 1) * 1_000_000) / fps) - timestampUs,
    footageLayers,
    components,
  }
}
export type BrowserEvaluatedFrame = ReturnType<typeof evaluateBrowserFrame>

/** Transport uses owner-bound access; even a URL returned by it remains runtime-only. */
export interface BrowserMediaResolver {
  source(
    snapshot: BrowserCompositionSnapshot,
    identity: BrowserFrozen<BrowserSourceIdentity>,
    signal: AbortSignal,
  ): Promise<string | Blob>
  speech(
    snapshot: BrowserCompositionSnapshot,
    identity: BrowserFrozen<ClipSpeechRef>,
    signal: AbortSignal,
  ): Promise<string | Blob>
}
export interface BrowserSnapshotToken {
  readonly snapshot: BrowserCompositionSnapshot
  readonly epoch: number
  readonly signal: AbortSignal
}
/** Seeking also supersedes decode callbacks; abort alone cannot fence a provider ignoring it. */
export class BrowserSnapshotEpoch {
  private epoch = 0
  private controller?: AbortController
  private snapshot?: BrowserCompositionSnapshot
  begin(snapshot: BrowserCompositionSnapshot): BrowserSnapshotToken {
    this.cancel()
    this.snapshot = snapshot
    this.controller = new AbortController()
    return { snapshot, epoch: this.epoch, signal: this.controller.signal }
  }
  seek(): BrowserSnapshotToken {
    if (!this.snapshot) return refuse('CLIP_SNAPSHOT_SUPERSEDED')
    return this.begin(this.snapshot)
  }
  isCurrent(token: BrowserSnapshotToken): boolean {
    return !token.signal.aborted && token.epoch === this.epoch && token.snapshot === this.snapshot
  }
  assertCurrent(token: BrowserSnapshotToken): void {
    if (!this.isCurrent(token)) refuse('CLIP_SNAPSHOT_SUPERSEDED')
  }
  publish(token: BrowserSnapshotToken, publish: () => void, release?: () => void): boolean {
    if (!this.isCurrent(token)) {
      release?.()
      return false
    }
    publish()
    return true
  }
  cancel(): void {
    this.controller?.abort()
    this.controller = undefined
    this.snapshot = undefined
    this.epoch++
  }
}
