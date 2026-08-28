export type ExportFormat = 'naver' | 'tistory' | 'site' | 'markdown'

export const EXPORT_FORMATS: readonly { key: ExportFormat; label: string }[] = [
  { key: 'naver', label: '네이버 블로그' },
  { key: 'tistory', label: '티스토리' },
  { key: 'site', label: '자체 사이트' },
  { key: 'markdown', label: '마크다운' },
]

export const EXPORT_GUIDANCE: Record<ExportFormat, string> = {
  naver: '붙여넣고 표시된 자리에 사진을 드래그하세요',
  tistory: 'HTML 모드에 붙여넣고 사진 업로드 후 src를 교체하세요',
  site: '그대로 .html로 저장하고 사진 파일을 옆에 두세요',
  markdown: 'Hugo · Jekyll · Obsidian — 사진 파일을 같은 폴더에 두세요',
}
