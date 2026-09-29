import { describe, expect, it } from 'vitest'
import { parseClipComposition } from './composition-parse'
import { sampleClipComposition } from './composition-sample'

const intro =
  '<text id="opening" kind="fixed" role="hook" basis="output-start"><row>첫 장면</row></text>'
const outro =
  '<text id="closing" kind="fixed" role="ending" basis="output-end"><row>끝</row></text>'
const badge = '<text id="ad" kind="fixed" role="badge" basis="whole">광고</text>'
const caption = (id: string, text: string) =>
  `<text id="${id}" kind="fixed" role="caption" basis="whole">${text}</text>`
const sample = (body: string, durationMs = 15000, styles?: string[]) =>
  sampleClipComposition(
    parseClipComposition(`<clip version="1">${body}</clip>`),
    durationMs,
    (label, n) => `${label} ${n}`,
    { caption: (n) => `샘플 자막 ${n}`, styles },
  )
const spans = (timeline: ReturnType<typeof sample>, role: string) =>
  timeline.elements
    .filter((e) => e.element.role === role)
    .map((e) => [e.text || e.rows.map((r) => r.text).join('/'), e.startMs, e.endMs])

// CLIP-170: the preview's own illustration of a clip.
describe('the template preview’s sample clip', () => {
  it('gives the intro the first 2.5 s, the outro the last 3 s and the badge the whole clip', () => {
    const timeline = sample(intro + outro + badge)
    expect(spans(timeline, 'hook')).toEqual([['첫 장면', 0, 2500]])
    expect(spans(timeline, 'ending')).toEqual([['끝', 12000, 15000]])
    expect(spans(timeline, 'badge')).toEqual([['광고', 0, 15000]])
  })

  it('fills the span between with captions of 3–4 s back to back, the outline’s own first', () => {
    for (const duration of [15000, 30000, 47300, 60000]) {
      const captions = sample(
        intro + caption('own', '가게 소개') + outro,
        duration,
      ).elements.filter((e) => e.element.role === 'caption')
      expect(captions[0].text).toBe('가게 소개')
      expect(captions.slice(1).map((c) => c.text)).toEqual(
        captions.slice(1).map((_, i) => `샘플 자막 ${i + 1}`),
      )
      expect(captions[0].startMs).toBe(2500)
      expect(captions.at(-1)!.endMs).toBe(duration - 3000)
      captions.forEach((c, i) => {
        if (i) expect(c.startMs).toBe(captions[i - 1].endMs)
        expect(c.endMs - c.startMs).toBeGreaterThanOrEqual(3000)
        expect(c.endMs - c.startMs).toBeLessThanOrEqual(4000)
      })
    }
  })

  it('keeps the outline’s caption entries in outline order', () => {
    const captions = spans(sample(caption('b', '둘째') + caption('a', '첫째') + intro), 'caption')
    expect(captions.slice(0, 2).map(([text]) => text)).toEqual(['둘째', '첫째'])
  })

  it('gives each caption the next allowed style in turn', () => {
    const styles = sample(intro + outro, 30000, ['neon', 'word-pop', 'bold'])
      .elements.filter((e) => e.element.role === 'caption')
      .map((e) => e.element.style)
    expect(styles).toEqual(['neon', 'word-pop', 'bold', 'neon', 'word-pop', 'bold', 'neon'])
  })

  it('leaves out a caption entry the span cannot hold', () => {
    const body = ['하나', '둘', '셋', '넷'].map((t, i) => caption(`c${i}`, t)).join('')
    const captions = spans(sample(intro + body + outro), 'caption')
    expect(captions.map(([text]) => text)).toEqual(['하나', '둘', '셋'])
  })

  it('leaves a region’s span to the captions when the outline has no entry for it', () => {
    const noIntro = spans(sample(outro), 'caption')
    expect(noIntro[0][1]).toBe(0)
    expect(noIntro.at(-1)![2]).toBe(12000)
    const noOutro = spans(sample(intro), 'caption')
    expect(noOutro[0][1]).toBe(2500)
    expect(noOutro.at(-1)![2]).toBe(15000)
    const neither = spans(sample(''), 'caption')
    expect([neither[0][1], neither.at(-1)![2]]).toEqual([0, 15000])
  })

  it('draws no other text beside the intro or the outro', () => {
    const info =
      '<text id="menu" kind="fixed" role="info" basis="whole"><row role="label">메뉴</row><row role="caption">찌개</row></text>'
    expect(spans(sample(intro + info + outro), 'info')).toEqual([['메뉴/찌개', 2500, 12000]])
  })
})
