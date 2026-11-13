import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useSpokenLibrary, type SpokenDraftInput } from '@/entities/spoken-voice'
import { SpokenVoiceCreation } from '@/widgets/spoken-voice-creation'
import {
  LoadingState,
  LoadingStatus,
  ReturnIcon,
  Typography,
  typographyStyles,
  pageStyles,
  returnControlStyles,
} from '@/shared/ui'
export function NewSpokenVoicePage() {
  const { t } = useTranslation('spokenVoices'),
    { user } = useSession(),
    ownerId = user?.id ?? '',
    search = useSearch({ from: '/authenticated/spoken-voices/new' }),
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
        aria-label={t('back')}
        title={t('back')}
        className={returnControlStyles('mb-4')}
      >
        <ReturnIcon />
      </Link>
      {qualification && (
        <Typography variant="meta" className="mb-4">
          {t('qualification')}
        </Typography>
      )}
      {search.copy && !copied ? (
        <>
          <Typography variant="display">{t('title')}</Typography>
          <LoadingStatus
            label={library.voicesQuery.isPending ? t('loading') : t('copyFailed')}
            className={
              library.voicesQuery.isPending
                ? 'sr-only'
                : typographyStyles({ variant: 'body', className: 'mt-4 block' })
            }
          />
          {library.voicesQuery.isPending && (
            <div aria-hidden="true">
              <LoadingState variant="form" label={t('loading')} className="mt-4" />
            </div>
          )}
        </>
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
