import i18next from 'i18next'
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { AppFailureMessage } from './AppFailureMessage'

afterEach(async () => i18next.changeLanguage('ko'))

describe('AppFailureMessage', () => {
  it.each([
    ['ko', '“메뉴”의 부족한 항목을 추가한 뒤 클립을 생성해 주세요.'],
    ['en', 'Add the missing items in “메뉴” before generating the clip.'],
  ])('names an incomplete group and the next action in %s', async (locale, message) => {
    await i18next.changeLanguage(locale)
    render(
      <AppFailureMessage
        failure={{
          reason: 'CLIP_COMPOSITION_INVALID',
          params: { element_id: '메뉴', line: '2', reason: 'items_required' },
        }}
      />,
    )
    expect(screen.getByText(message)).toBeInTheDocument()
  })

  it.each([
    ['ko', /최소 1개 필요한데 지금 0개/],
    ['en', /at least 1 item\(s\) but has 0/],
  ])('says how many items the group admits and how many were given in %s', async (locale, text) => {
    await i18next.changeLanguage(locale)
    render(
      <AppFailureMessage
        failure={{
          reason: 'CLIP_COMPOSITION_INVALID',
          params: {
            element_id: '메뉴',
            label: '메뉴',
            line: '2',
            reason: 'items_required',
            min: '1',
            actual: '0',
          },
        }}
      />,
    )
    expect(screen.getByText(text)).toHaveTextContent('메뉴')
  })

  it.each(['ko', 'en'])('explains invalid group bounds in %s', async (locale) => {
    await i18next.changeLanguage(locale)
    render(
      <AppFailureMessage
        failure={{
          reason: 'CLIP_COMPOSITION_INVALID',
          params: { element_id: 'menu', line: '2', reason: 'invalid_item_bounds' },
        }}
      />,
    )
    expect(
      screen.getByText(
        locale === 'ko' ? /최소 개수는 최대 개수 이하/ : /minimum no greater than the maximum/,
      ),
    ).toHaveTextContent('menu')
  })

  it('retains the generic location message for an unknown composition reason', () => {
    render(
      <AppFailureMessage
        failure={{
          reason: 'CLIP_COMPOSITION_INVALID',
          params: { element_id: 'menu', line: '2', reason: 'future_reason' },
        }}
      />,
    )
    expect(screen.getByText('영상 구성의 2번째 줄(menu)을 확인해 주세요.')).toBeInTheDocument()
  })

  it.each([
    ['ko', '샘플은 최소 200자가 필요해요. 현재 20자예요.'],
    ['en', 'A sample must contain at least 200 characters. It currently contains 20.'],
  ])('renders the stable reason in %s', async (locale, message) => {
    await i18next.changeLanguage(locale)
    render(
      <AppFailureMessage
        failure={{
          reason: 'VOICE_SAMPLE_TOO_SHORT',
          params: { actual: '20', min: '200' },
        }}
      />,
    )
    expect(screen.getByText(message)).toBeInTheDocument()
  })

  it('shows escaped technical diagnostics only behind the generic label', () => {
    const detail = '<script>private provider prose</script>'
    const { container } = render(
      <AppFailureMessage
        failure={{ reason: 'MODEL_UNAVAILABLE', params: {}, technicalDetail: detail }}
      />,
    )
    expect(screen.getByText('기술 세부 정보')).toBeInTheDocument()
    expect(screen.getByText(detail)).toBeInTheDocument()
    expect(container.querySelector('script')).toBeNull()
  })
})
