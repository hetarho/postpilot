export type ExportFormat = 'naver' | 'tistory' | 'site' | 'markdown'

/** Shaped as `{ value, label }` so it is passed straight to `SegmentedControl` as its options. */
export const EXPORT_FORMATS: readonly ExportFormat[] = ['naver', 'tistory', 'site', 'markdown']
