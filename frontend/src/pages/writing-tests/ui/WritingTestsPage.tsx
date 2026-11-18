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
import { Button, LoadingState, Typography, PageFrame, TitleRow } from '@/shared/ui'
import {
  writingTestSearchSchema,
  writingTestFactor,
  writingTestFactorNamed,
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
      seed.guidelineId ??
      (writingTestFactor(seed) === 'guideline' ? WRITING_TEST_NEW_GUIDELINE_SLOT : ''),
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
  // Named only by the address the owner opened: the router's normalization of the validated
  // search writes no factor an entry left out, so this never flips after the first render.
  const factorNamed = writingTestFactorNamed(seed)
  return (
    <OwnedNewPage
      key={JSON.stringify([user?.id ?? '', seed, factorNamed])}
      ownerId={user?.id ?? ''}
      seed={seed}
      factorNamed={factorNamed}
    />
  )
}
function OwnedNewPage({
  ownerId,
  seed,
  factorNamed,
}: {
  ownerId: string
  seed: WritingTestSearch
  factorNamed: boolean
}) {
  const { t } = useWritingTestTranslation()
  const source = useWritingTestSource(ownerId, seed.source ?? '')
  const factor = writingTestFactor(seed)
  const binaryModel = factor === 'model' && seed.count === 2
  const pairs = useModelSetup(binaryModel && !!ownerId)
  const models = useModels()
  const context = useMemo(() => source.data?.context ?? initialContext(seed), [source.data, seed])
  const pair =
    binaryModel && !pairs.isError && !models.isError
      ? eligibleTestPair(pairs.pairs, models.models, seed.stage)
      : undefined
  const initialPlan: WritingTestPlan = {
    factor,
    modelStage: seed.stage,
    count: seed.count,
    entrants: pair?.map((model) => ({ type: 'model' as const, model })) ?? [],
    context,
  }
  if (seed.source && (source.isPending || source.isError))
    return (
      <PageFrame width="workspace">
        {source.isError ? (
          <Typography variant="body" role="alert">
            {t('failed')}
          </Typography>
        ) : (
          <LoadingState variant="form" label={t('loading')} />
        )}
        {source.isError && (
          <Button variant="secondary" onClick={() => void source.refetch()}>
            {t('refresh')}
          </Button>
        )}
      </PageFrame>
    )
  // Resolve this optional seed before the actor mounts. Its owner/entry recovery takes
  // precedence over initialPlan, so a later recommendation never replaces retained work.
  if (
    !ownerId ||
    (binaryModel && !pairs.isError && !models.isError && (pairs.isPending || models.isPending))
  )
    return (
      <PageFrame width="workspace">
        <LoadingState variant="form" label={t('loading')} />
      </PageFrame>
    )
  return (
    <PageFrame width="workspace" className="flex flex-1 flex-col pb-0 sm:pb-0">
      <WritingTestStudio
        heading={
          <Typography variant="title" as="h1" className="truncate">
            {t('title')}
          </Typography>
        }
        ownerId={ownerId}
        seedKey={JSON.stringify(seed)}
        initialPlan={initialPlan}
        factorNamed={factorNamed}
      />
    </PageFrame>
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
    <PageFrame width="workspace" className="flex flex-1 flex-col pb-0 sm:pb-0">
      <WritingTestStudio
        heading={(output) => (
          <Typography variant="title" as="h1" className="truncate">
            {t(output === 'observation' ? 'photoTestTitle' : 'title')}
          </Typography>
        )}
        ownerId={user?.id ?? ''}
        seedKey={`test:${testId}`}
        testId={testId}
        initialPlan={initialPlan}
      />
    </PageFrame>
  )
}
export function WritingTestHistoryPage() {
  const { user } = useSession(),
    { t } = useWritingTestTranslation(),
    raw = useSearch({ strict: false })
  const search = writingTestHistorySearchSchema(raw as Record<string, unknown>)
  return (
    <PageFrame width="wide" className="flex flex-1 flex-col">
      <TitleRow variant="display">
        <Typography variant="display">{t('history')}</Typography>
      </TitleRow>
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
        showHeading={false}
        ownerId={user?.id ?? ''}
        voiceId={search.voiceId}
        stage={search.stage}
        sourcePostSlug={search.source}
      />
    </PageFrame>
  )
}
