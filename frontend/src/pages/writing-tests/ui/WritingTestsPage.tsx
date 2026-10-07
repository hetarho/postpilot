import { useWritingTestTranslation } from '@/features/writing-test'
import { useMemo } from 'react'
import { useSearch, useParams } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { eligibleTestPair, useModels, useModelSetup } from '@/entities/model-catalog'
import {
  useWritingTestSource,
  type WritingTestPlan,
  type WritingTestContext,
} from '@/entities/writing-test'
import { POST_TARGET_LENGTH_DEFAULT, POST_TAG_COUNT_DEFAULT } from '@/entities/post'
import { WritingTestStudio, WritingTestHistory } from '@/widgets/writing-test'
import { WRITING_TEST_NEW_GUIDELINE_SLOT } from '@/features/writing-test'
import { activeLocale } from '@/shared/lib'
import { Button, Typography, pageStyles } from '@/shared/ui'
import {
  writingTestSearchSchema,
  writingTestHistorySearchSchema,
  type WritingTestSearch,
} from '../model/search'

function initialContext(seed: WritingTestSearch): WritingTestContext {
  return {
    sourcePostSlug: seed.source ?? '',
    expectedInputRevision: 0n,
    expectedContentRevision: 0n,
    material: { text: '', fictional: false, attachmentIds: [], templateAnswers: [] },
    voiceId: seed.voiceId ?? '',
    templateId: seed.templateId ?? '',
    guidelineSlotId:
      seed.guidelineId ?? (seed.factor === 'guideline' ? WRITING_TEST_NEW_GUIDELINE_SLOT : ''),
    targetLanguage: activeLocale(),
    targetLength: POST_TARGET_LENGTH_DEFAULT,
    tagCount: POST_TAG_COUNT_DEFAULT,
    useMemory: false,
    qualityRules: [],
  }
}
export function WritingTestsPage() {
  const { user } = useSession(),
    raw = useSearch({ strict: false })
  const seed = writingTestSearchSchema(raw as Record<string, unknown>)
  return (
    <OwnedNewPage
      key={JSON.stringify([user?.id ?? '', seed])}
      ownerId={user?.id ?? ''}
      seed={seed}
    />
  )
}
function OwnedNewPage({ ownerId, seed }: { ownerId: string; seed: WritingTestSearch }) {
  const { t } = useWritingTestTranslation()
  const source = useWritingTestSource(ownerId, seed.source ?? '')
  const binaryModel = seed.factor === 'model' && seed.count === 2
  const pairs = useModelSetup(binaryModel && !!ownerId)
  const models = useModels()
  const context = useMemo(() => source.data?.context ?? initialContext(seed), [source.data, seed])
  const pair =
    binaryModel && !pairs.isError && !models.isError
      ? eligibleTestPair(pairs.pairs, models.models, seed.stage)
      : undefined
  const initialPlan: WritingTestPlan = {
    factor: seed.factor,
    modelStage: seed.stage,
    count: seed.count,
    entrants: pair?.map((model) => ({ type: 'model' as const, model })) ?? [],
    context,
  }
  if (seed.source && (source.isPending || source.isError))
    return (
      <main className={pageStyles({ width: 'workspace' })}>
        <Typography variant="body" role={source.isError ? 'alert' : 'status'}>
          {t(source.isError ? 'failed' : 'loading')}
        </Typography>
        {source.isError && (
          <Button variant="secondary" onClick={() => void source.refetch()}>
            {t('refresh')}
          </Button>
        )}
      </main>
    )
  // Resolve this optional seed before the actor mounts. Its owner/entry recovery takes
  // precedence over initialPlan, so a later recommendation never replaces retained work.
  if (
    !ownerId ||
    (binaryModel && !pairs.isError && !models.isError && (pairs.isPending || models.isPending))
  )
    return (
      <main className={pageStyles({ width: 'workspace' })}>
        <Typography variant="body" role="status">
          {t('loading')}
        </Typography>
      </main>
    )
  return (
    <main className={pageStyles({ width: 'workspace', className: 'flex flex-1 flex-col' })}>
      <Typography variant="title" as="h1">
        {t('title')}
      </Typography>
      <WritingTestStudio
        ownerId={ownerId}
        seedKey={JSON.stringify(seed)}
        initialPlan={initialPlan}
      />
    </main>
  )
}
export function WritingTestPage() {
  const { user } = useSession(),
    params = useParams({ strict: false }),
    { t } = useWritingTestTranslation()
  const testId = (params as { testId?: string }).testId ?? ''
  const initialPlan: WritingTestPlan = {
    factor: 'model',
    modelStage: 'write',
    count: 2,
    entrants: [],
    context: initialContext({ factor: 'model', stage: 'write', count: 2 }),
  }
  return (
    <main className={pageStyles({ width: 'workspace', className: 'flex flex-1 flex-col' })}>
      <Typography variant="title" as="h1">
        {t('title')}
      </Typography>
      <WritingTestStudio
        ownerId={user?.id ?? ''}
        seedKey={`test:${testId}`}
        testId={testId}
        initialPlan={initialPlan}
      />
    </main>
  )
}
export function WritingTestHistoryPage() {
  const { user } = useSession(),
    { t } = useWritingTestTranslation(),
    raw = useSearch({ strict: false })
  const search = writingTestHistorySearchSchema(raw as Record<string, unknown>)
  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <Typography variant="display">{t('history')}</Typography>
      <Typography variant="body" className="max-w-measure mt-3">
        {t('historyHelp')}
      </Typography>
      <Button
        variant="ghost"
        className="mt-4 self-start"
        onClick={() =>
          window.location.assign(`/tests?draft=${encodeURIComponent(crypto.randomUUID())}`)
        }
      >
        {t('newTest')}
      </Button>
      <WritingTestHistory
        ownerId={user?.id ?? ''}
        voiceId={search.voiceId}
        stage={search.stage}
        sourcePostSlug={search.source}
      />
    </main>
  )
}
