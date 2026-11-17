import { useCallback, useEffect } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { useWritingTestTranslation } from '@/features/writing-test'
import { PhotoTestStudio } from '@/widgets/writing-test'
import { LoadingState, PageFrame, Typography } from '@/shared/ui'
import { photoTestSearchSchema } from '../model/search'
import { photoTestPlan } from '../model/photo-plan'

/** /tests/photos. Each entry gets its own `draft` key, so a reload resumes the setup while a
 *  new entry starts over (MODEL-91); a started test opens at /tests/$testId. */
export function PhotoTestsPage() {
  const { user } = useSession(),
    { t } = useWritingTestTranslation(),
    navigate = useNavigate(),
    raw = useSearch({ strict: false })
  const seed = photoTestSearchSchema(raw as Record<string, unknown>)
  const { draft, count, entry } = seed
  // The parsed seed is a new object every render; depend on its fields, not on it.
  useEffect(() => {
    if (!draft)
      void navigate({
        to: '/tests/photos',
        search: { count, ...(entry ? { entry } : {}), draft: crypto.randomUUID() },
        replace: true,
      })
  }, [draft, count, entry, navigate])
  const onStarted = useCallback(
    (testId: string) =>
      void navigate({
        to: '/tests/$testId',
        params: { testId },
        search: seed.entry ? { entry: seed.entry } : {},
        replace: true,
      }),
    [navigate, seed.entry],
  )
  if (!user?.id || !seed.draft)
    return (
      <PageFrame width="workspace">
        <LoadingState variant="form" label={t('loading')} />
      </PageFrame>
    )
  return (
    <PageFrame width="workspace" className="flex flex-1 flex-col pb-0 sm:pb-0">
      <PhotoTestStudio
        heading={
          <Typography variant="title" as="h1" className="truncate">
            {t('photoTestTitle')}
          </Typography>
        }
        ownerId={user.id}
        seedKey={`photos:${seed.draft}`}
        initialPlan={photoTestPlan(seed)}
        onStarted={onStarted}
      />
    </PageFrame>
  )
}
