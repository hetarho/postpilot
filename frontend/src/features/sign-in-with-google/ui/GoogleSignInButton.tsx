import { useTranslation } from 'react-i18next'
import { GOOGLE_CLIENT_ID } from '@/shared/config'
import { Button } from '@/shared/ui'
import { startGoogleSignIn } from '../lib/google-sign-in'

export function GoogleSignInButton({ redirect }: { redirect?: string }) {
  const { t } = useTranslation('auth')
  if (!GOOGLE_CLIENT_ID) return null

  return (
    <Button
      variant="secondary"
      className="mt-3 w-full"
      onClick={() => void startGoogleSignIn(redirect)}
    >
      <svg
        aria-hidden="true"
        viewBox="0 0 24 24"
        className="text-content-primary size-5 shrink-0"
        fill="currentColor"
      >
        <path d="M21.6 12.2c0-.7-.1-1.5-.2-2.2H12v4h5.4a4.7 4.7 0 0 1-2 3v2.6h3.3c1.9-1.8 2.9-4.4 2.9-7.4Z" />
        <path d="M12 22c2.7 0 5-.9 6.7-2.4L15.4 17c-.9.6-2.1 1-3.4 1-2.6 0-4.8-1.8-5.6-4.1H3v2.7A10 10 0 0 0 12 22Z" />
        <path d="M6.4 13.9A6 6 0 0 1 6.1 12c0-.7.1-1.3.3-1.9V7.4H3A10 10 0 0 0 2 12c0 1.7.4 3.2 1 4.6l3.4-2.7Z" />
        <path d="M12 6c1.5 0 2.8.5 3.9 1.5l2.9-2.9A9.8 9.8 0 0 0 12 2a10 10 0 0 0-9 5.4l3.4 2.7A6 6 0 0 1 12 6Z" />
      </svg>
      {t('google.continue')}
    </Button>
  )
}
