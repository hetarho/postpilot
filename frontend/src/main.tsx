import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from '@/app'
import { initializeI18n } from '@/app/providers/i18n'
import { bootstrapTheme } from '@/app/providers/theme'
import '@/app/styles/index.css'

const themeSnapshot = bootstrapTheme()
initializeI18n()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App themeSnapshot={themeSnapshot} />
  </StrictMode>,
)
