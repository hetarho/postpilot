import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useSpokenLibrary, type SpokenDraftInput } from '@/entities/spoken-voice'
import { SpokenVoiceCreation } from '@/widgets/spoken-voice-creation'
import { Typography, pageStyles, buttonStyles } from '@/shared/ui'
export function NewSpokenVoicePage() {
  const { t } = useTranslation('spokenVoices'),
    { user } = useSession(),
    ownerId = user?.id ?? '',
    search = useSearch({ from: '/authenticated/video/spoken-voices/new' }),
    navigate = useNavigate(),
    library = useSpokenLibrary(ownerId)
  const qualification = user?.plan === 'master' ? search.qualification : undefined
  const address = { ...search, qualification }
  const copied = library.voices.find((v) => v.id === search.copy)
  const initial: SpokenDraftInput | undefined = copied
    ? {
        name: copied.name,
        description: copied.description,
        previewText: copied.previewText,
        profileId: '',
        profileRevision: 0n,
        qualificationSessionId: qualification ?? '',
      }
    : undefined
  return (
    <main className={pageStyles({ width: 'prose', className: 'min-w-0' })}>
      <Link
        to="/spoken-voices"
        className={buttonStyles({ variant: 'ghost', className: 'mb-4 -ml-3' })}
      >
        {t('back')}
      </Link>
      {qualification && (
        <Typography variant="meta" className="mb-4">
          {t('qualification')}
        </Typography>
      )}
      {search.copy && !copied ? (
        <Typography variant="body" role="status">
          {library.voicesQuery.isPending ? t('loading') : t('copyFailed')}
        </Typography>
      ) : (
        <SpokenVoiceCreation
          key={ownerId}
          ownerId={ownerId}
          address={address}
          initialInput={initial}
          onAddress={(next) =>
            void navigate({ to: '/spoken-voices/new', search: next, replace: true })
          }
        />
      )}
    </main>
  )
}
