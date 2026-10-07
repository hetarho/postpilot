import { CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import { timelineCuts, type ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import type { BrowserCompositionSnapshot } from './browser-composition'
import { CLIP_COMPOSITION_PLACEMENT } from '../config/composition-placement'
import type { InkBox } from './ink-typography'

const intersects = (a: InkBox, b: InkBox) =>
  a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height
const union = (a: InkBox, b: InkBox): InkBox => ({
  x: Math.min(a.x, b.x),
  y: Math.min(a.y, b.y),
  width: Math.max(a.x + a.width, b.x + b.width) - Math.min(a.x, b.x),
  height: Math.max(a.y + a.height, b.y + b.height) - Math.min(a.y, b.y),
})
const empty = (): InkBox => ({ x: 0, y: 0, width: 0, height: 0 })
export function sharedCaptionBounds(a: readonly InkBox[], b: readonly InkBox[]): InkBox[] {
  return a.flatMap((one) =>
    b.flatMap((other) => {
      const x = Math.max(one.x, other.x),
        y = Math.max(one.y, other.y)
      const width = Math.min(one.x + one.width, other.x + other.width) - x,
        height = Math.min(one.y + one.height, other.y + other.height) - y
      return width > 0 && height > 0 ? [{ x, y, width, height }] : []
    }),
  )
}
/** Native coveredFootage uses nominal output-ms cut windows, including transition overlap. */
export function coveredBrowserFootage(
  snapshot: BrowserCompositionSnapshot,
  component: BrowserCompositionSnapshot['components'][number],
) {
  const canvas = CLIP_DESIGN.ratios[snapshot.ratio].canvas
  let subject = empty(),
    readableText = false,
    captionSafe: InkBox[] = [],
    covered = 0
  for (const item of timelineCuts(snapshot.plan as ClipEditPlan)) {
    const cut = item.cut
    if (
      component.element.cutId
        ? cut.id !== component.element.cutId
        : item.startMs >= component.endMs || item.endMs <= component.startMs
    )
      continue
    for (const observation of snapshot.layoutObservations) {
      if (observation.sourceId !== cut.sourceId) continue
      const segments = observation.segments.filter(
        (s) => s.startMs < cut.endMs && cut.startMs < s.endMs,
      )
      readableText ||= segments.some((s) => s.readableText)
      const scale = Math.max(canvas.width / observation.width, canvas.height / observation.height)
      const width = observation.width * scale,
        height = observation.height * scale
      const cropX = Math.max(
        0,
        Math.min(width - canvas.width, width * (cut.focal?.x ?? 0.5) - canvas.width / 2),
      )
      const cropY = Math.max(
        0,
        Math.min(height - canvas.height, height * (cut.focal?.y ?? 0.5) - canvas.height / 2),
      )
      const project = (box: InkBox): InkBox | undefined => {
        const left = Math.max(0, Math.min(canvas.width, box.x * width - cropX)),
          top = Math.max(0, Math.min(canvas.height, box.y * height - cropY))
        const right = Math.max(0, Math.min(canvas.width, (box.x + box.width) * width - cropX)),
          bottom = Math.max(0, Math.min(canvas.height, (box.y + box.height) * height - cropY))
        return right > left && bottom > top
          ? { x: left, y: top, width: right - left, height: bottom - top }
          : undefined
      }
      let cutSubject = empty()
      for (const segment of segments) {
        if (!(segment.subject.width > 0 && segment.subject.height > 0)) continue
        const box = project(segment.subject)
        if (box)
          cutSubject = cutSubject.width > 0 && cutSubject.height > 0 ? union(cutSubject, box) : box
      }
      if (!covered) subject = cutSubject
      else if (cutSubject.width || cutSubject.height) subject = union(subject, cutSubject)
      let safe: InkBox[] = []
      for (const [index, segment] of segments.entries()) {
        if (!segment.captionSafe.length) {
          safe = []
          break
        }
        safe = index
          ? sharedCaptionBounds(safe, segment.captionSafe).filter(
              (box, index, all) =>
                all.findIndex((other) => JSON.stringify(other) === JSON.stringify(box)) === index,
            )
          : segment.captionSafe.map((box) => ({ ...box }))
        if (!safe.length) break
      }
      const boxes = safe.flatMap((box) => {
        const projected = project(box)
        return projected ? [projected] : []
      })
      captionSafe = covered ? sharedCaptionBounds(captionSafe, boxes) : boxes
      covered++
    }
  }
  return { subject, readableText, captionSafe }
}
export interface BrowserAnchorCandidate {
  anchor: string
  align: string
  plate: InkBox
  fits: boolean
  authoredAlign: boolean
}
/** Port of native SelectAnchor: containment ranks candidates; it is not a new safety veto. */
export function selectBrowserAnchor(
  candidates: readonly BrowserAnchorCandidate[],
  subject: InkBox,
  placed: readonly InkBox[],
  readableText: boolean,
  previous: string,
  captionSafe: readonly InkBox[],
): number {
  const order: readonly string[] = CLIP_COMPOSITION_PLACEMENT.anchors
  const distance = (anchor: string) => {
    const a = order.indexOf(previous),
      b = order.indexOf(anchor)
    return a < 0 || b < 0 ? order.length : Math.abs(a - b)
  }
  let viable = candidates.flatMap((candidate, index) =>
    !candidate.fits ||
    (candidate.align !== '' && candidate.align !== 'center' && !candidate.authoredAlign) ||
    (readableText && !['top', 'bottom'].includes(candidate.anchor)) ||
    placed.some((box) => intersects(candidate.plate, box))
      ? []
      : [index],
  )
  const near = viable.filter(
    (index) => readableText || !previous || distance(candidates[index]!.anchor) <= 1,
  )
  if (near.length) viable = near
  if (!viable.length) return -1
  const inside = (index: number) =>
    captionSafe.some((box) => {
      const p = candidates[index]!.plate
      return (
        box.width > 0 &&
        box.height > 0 &&
        p.x >= box.x &&
        p.y >= box.y &&
        p.x + p.width <= box.x + box.width &&
        p.y + p.height <= box.y + box.height
      )
    })
  if (captionSafe.length) viable.sort((a, b) => Number(inside(b)) - Number(inside(a)))
  if (!(subject.width > 0 && subject.height > 0)) return viable[0]!
  let best = -1,
    least = 2
  for (const index of viable) {
    const p = candidates[index]!.plate
    const width = Math.min(p.x + p.width, subject.x + subject.width) - Math.max(p.x, subject.x),
      height = Math.min(p.y + p.height, subject.y + subject.height) - Math.max(p.y, subject.y)
    const cover = width > 0 && height > 0 ? (width * height) / (subject.width * subject.height) : 0
    if (cover <= CLIP_COMPOSITION_PLACEMENT.subjectCoverMax) return index
    if (cover < least) {
      best = index
      least = cover
    }
  }
  return best
}
