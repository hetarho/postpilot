import { render, screen } from '@testing-library/react'
import { useTranslation } from 'react-i18next'
import { expect, it } from 'vitest'
import { initializeI18n } from './index'

function DynamicCopy({ filename }: { filename: string }) {
  const { t } = useTranslation('posts')
  return <p data-testid="copy">{t('upload.deleteDescription', { filename })}</p>
}

it('leaves dynamic values verbatim while React keeps markup inert', () => {
  initializeI18n('en')
  const filename = 'A&B <img src=x onerror="alert(1)">.jpg'
  render(<DynamicCopy filename={filename} />)

  expect(screen.getByTestId('copy').textContent).toContain(filename)
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
})
