import i18next from 'i18next'

/** The five stations every lifecycle surface runs (THEME-39): the post editor and the clip
 *  workspace share the order and the short names, so one product reads as one app and the row fits
 *  a 360 px phone. */
export const LIFECYCLE_STATIONS = ['generate', 'observe', 'storyline', 'refine', 'finish'] as const

export type LifecycleStation = (typeof LIFECYCLE_STATIONS)[number]

/** The stations' short names, registered with the `common` namespace (ARCH-16). */
export const lifecycleStepsI18n = {
  namespace: 'common',
  ko: {
    lifecycle: {
      generate: '생성',
      observe: '관찰',
      storyline: '스토리라인',
      refine: '다듬기',
      finish: '완성',
    },
  },
  en: {
    lifecycle: {
      generate: 'Create',
      observe: 'Observe',
      storyline: 'Storyline',
      refine: 'Refine',
      finish: 'Finish',
    },
  },
} as const

export function lifecycleStationLabel(station: LifecycleStation): string {
  return i18next.t(`lifecycle.${station}`, { ns: 'common' })
}
