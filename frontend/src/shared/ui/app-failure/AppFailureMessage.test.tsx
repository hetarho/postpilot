import i18next from 'i18next'
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { AppFailureMessage } from './AppFailureMessage'

afterEach(async () => i18next.changeLanguage('ko'))

describe('AppFailureMessage', () => {
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
