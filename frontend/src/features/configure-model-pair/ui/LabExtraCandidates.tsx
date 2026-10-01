import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  filterForStage,
  modelChoiceIssue,
  refKey,
  savedChoiceIssue,
  type ComparisonPair,
  type StageName,
  useModels,
  useSaveLabExtraCandidates,
} from '@/entities/model-catalog'
import { AppFailureMessage, Button, Typography } from '@/shared/ui'
import { ModelSelect } from './ModelSelect'

/** The lab owns C/D/E; the editor's pair form remains exactly two fields. */
export function LabExtraCandidates({
  stage,
  pair,
  onShownChange,
}: {
  stage: StageName
  pair: ComparisonPair | undefined
  onShownChange: (keys: string[]) => void
}) {
  const { t } = useTranslation('models')
  const { models } = useModels()
  const suitable = filterForStage(models, stage)
  const save = useSaveLabExtraCandidates()
  const saved = (pair?.extraCandidates ?? []).map((item) => refKey(item.ref))
  const [keys, setKeys] = useState<string[]>(saved)
  const [notice, setNotice] = useState<{ index: number; text: string }>()
  const [changed, setChanged] = useState(-1)
  const labels = [t('labExtra.candidateC'), t('labExtra.candidateD'), t('labExtra.candidateE')]

  useEffect(() => {
    onShownChange(keys)
  }, [keys, onShownChange])

  const pairKeys = [pair?.candidateA, pair?.candidateB]
    .filter((item) => item && !item.missing)
    .map((item) => refKey(item!.ref))
  const commit = (next: string[], index: number) => {
    setKeys(next)
    setChanged(index)
    const duplicate =
      new Set([...pairKeys, ...next.filter(Boolean)]).size !==
      pairKeys.length + next.filter(Boolean).length
    const invalid = next.find((key) => {
      const model = suitable.find((item) => refKey(item.ref) === key)
      return !model || Boolean(modelChoiceIssue(model, stage))
    })
    if (duplicate || invalid || next.some((key) => !key)) {
      setNotice({
        index,
        text: duplicate
          ? t('differentModels')
          : invalid
            ? t('labExtra.unavailable')
            : t('labExtra.choose'),
      })
      return
    }
    setNotice(undefined)
    const refs = next.map((key) => suitable.find((item) => refKey(item.ref) === key)!.ref)
    void save.save(stage, refs).catch(() => {})
  }
  const remove = (index: number) => {
    const next = keys.filter((_, at) => at !== index)
    commit(next, index)
  }
  return (
    <div className="mt-4 space-y-4">
      {keys.map((key, index) => {
        const selection = pair?.extraCandidates[index]
        const storedIssue =
          selection && refKey(selection.ref) === key ? savedChoiceIssue(selection) : ''
        const model = suitable.find((item) => refKey(item.ref) === key)
        const conflict = key && pairKeys.includes(key) ? t('labExtra.conflict') : ''
        return (
          <div key={index} className="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-3">
            <ModelSelect
              label={labels[index]}
              stage={stage}
              value={key}
              models={suitable}
              savedIssue={storedIssue || (selection?.missing ? t('labExtra.unavailable') : '')}
              onChange={(value) =>
                commit(
                  keys.map((item, at) => (at === index ? value : item)),
                  index,
                )
              }
              saving={save.isPending}
              error={changed === index ? save.failure : undefined}
              notice={
                conflict ||
                (notice?.index === index ? notice.text : '') ||
                (model ? modelChoiceIssue(model, stage) : '')
              }
            />
            <Button
              variant="ghost"
              className="min-h-11"
              disabled={save.isPending}
              onClick={() => remove(index)}
            >
              {t('labExtra.remove')}
            </Button>
          </div>
        )
      })}
      {keys.length < 3 && (
        <Button
          variant="secondary"
          className="min-h-11"
          disabled={save.isPending}
          onClick={() => setKeys([...keys, ''])}
        >
          {t('labExtra.add')}
        </Button>
      )}
      {save.failure && changed >= keys.length && (
        <Typography variant="body" as="div" role="alert" className="text-field-error">
          <AppFailureMessage failure={save.failure} />
        </Typography>
      )}
    </div>
  )
}
