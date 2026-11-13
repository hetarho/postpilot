import i18next from 'i18next'
import { beforeAll, describe, expect, it } from 'vitest'
import { LIFECYCLE_STATIONS, lifecycleStationLabel, lifecycleStepsI18n } from './lifecycle-steps'

describe('lifecycle stations (THEME-39)', () => {
  beforeAll(async () => {
    await i18next.init({
      lng: 'ko',
      ns: ['common'],
      resources: { ko: { common: lifecycleStepsI18n.ko }, en: { common: lifecycleStepsI18n.en } },
    })
  })

  it('runs the five stations in order', () => {
    expect(LIFECYCLE_STATIONS).toEqual(['generate', 'observe', 'storyline', 'refine', 'finish'])
  })

  it('labels each station by its short name in both languages', async () => {
    await i18next.changeLanguage('ko')
    expect(LIFECYCLE_STATIONS.map(lifecycleStationLabel)).toEqual([
      '생성',
      '관찰',
      '스토리라인',
      '다듬기',
      '완성',
    ])
    await i18next.changeLanguage('en')
    expect(LIFECYCLE_STATIONS.map(lifecycleStationLabel)).toEqual([
      'Create',
      'Observe',
      'Storyline',
      'Refine',
      'Finish',
    ])
  })

  it('keeps one label per station in each language', () => {
    expect(Object.keys(lifecycleStepsI18n.ko.lifecycle)).toEqual([...LIFECYCLE_STATIONS])
    expect(Object.keys(lifecycleStepsI18n.en.lifecycle)).toEqual([...LIFECYCLE_STATIONS])
  })
})
