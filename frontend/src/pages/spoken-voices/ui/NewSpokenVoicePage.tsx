import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { useSpokenLibrary, type SpokenDraftInput } from '@/entities/spoken-voice'
import { SpokenVoiceCreation } from '@/widgets/spoken-voice-creation'
import {
  LoadingState,
  LoadingStatus,
  ReturnIcon,
  TitleRow,
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
  // The wizard's way back stands at the start of its title's row (THEME-75).
  const back = (
    <Link
      to="/spoken-voices"
      aria-label={t('back')}
      title={t('back')}
      className={returnControlStyles()}
    >
      <ReturnIcon />
    </Link>
  )
  return (
    <main className={pageStyles({ width: 'prose', className: 'min-w-0' })}>
      {qualification && (
        <Typography variant="meta" className="mb-4">
          {t('qualification')}
        </Typography>
      )}
      {search.copy && !copied ? (
        <>
          <TitleRow variant="display" back={back}>
            <Typography variant="display">{t('title')}</Typography>
          </TitleRow>
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
          back={back}
          onAddress={(next) =>
            void navigate({ to: '/spoken-voices/new', search: next, replace: true })
          }
        />
      )}
    </main>
  )
}
