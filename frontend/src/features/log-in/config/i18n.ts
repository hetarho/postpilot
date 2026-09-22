import type { I18nFragment } from '@/shared/lib'

export const i18n = {
  namespace: 'auth',
  ko: { loginPreferences: { rememberMe: '자동 로그인', rememberId: '아이디 저장' } },
  en: { loginPreferences: { rememberMe: 'Keep me signed in', rememberId: 'Remember login ID' } },
} as const satisfies I18nFragment
