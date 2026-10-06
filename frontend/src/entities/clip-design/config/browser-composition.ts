/** Identity negotiation is independent of component/output qualification (ARCH-68). */
export const CLIP_BROWSER_COMPOSITION = {
  schemaVersion: 1,
  renderer: 'clip-browser-composition-v1',
  components: 'native-cds-r33-v1',
  fonts: 'bundled-clip-fonts-v1',
  assets: 'clip-design-assets-v1',
  qualified: false,
} as const

/** Native CaptionStyle.Static(); sequence captions own frame-quantized intervals. */
export const CLIP_BROWSER_STATIC_CAPTIONS = ['bold', 'keynote', 'film'] as const

/** Same files as media/copy.go; never substitute a system face. */
export const CLIP_BROWSER_FONTS = [
  { id: 'wantedsans', sha256: '9953a7cfc4a3cba4ef1242abaf89779b3cd15fd9729c2d67d9e9d37a0da967f5' },
  { id: 'paperlogy', sha256: 'fb0324f8ac057e50f4f4632331617e347bfe5a04184f7b0db514be682fb6b25c' },
  { id: 'jua', sha256: '769677aef240bfc3b9965f2b50748075bff885e6c6992fc591a3fb268279f898' },
  {
    id: 'nanummyeongjo',
    sha256: '7ed9e8653a8ed04285d51dc343ffea6eb3d9c73afc27383ea8929ee4ffd03205',
  },
  {
    id: 'nanummyeongjo-800',
    sha256: '60c0077fce069ba90ae97c0a3679f6eb3712e0ca637bdd0c15b72d335ec46db7',
  },
] as const
