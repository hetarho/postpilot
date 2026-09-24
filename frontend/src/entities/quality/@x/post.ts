// What the post entity may import from the quality entity: a content save, a URL save and a delete
// each change what a quality reading says, so the post hooks mark the readings stale.
export { invalidateQuality } from '../api/quality-cache'
// A post names the metrics it has ticked (POST-81), so its mapper reads them by these.
export type { QualityMetricId } from '../model/types'
export { qualityMetricFromProto, qualityMetricToProto } from '../api/quality-mappers'
