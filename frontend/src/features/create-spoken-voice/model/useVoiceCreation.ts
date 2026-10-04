import { useState } from 'react'
import { useSpeechProfiles } from '@/entities/model-catalog'
import {
  useSpokenActions,
  useSpokenDraft,
  useSpokenOperation,
  useSpokenWorkActions,
  spokenOperationActive,
  SPOKEN_AUDITION_TEXT,
  SPOKEN_NAME_MAX,
  SPOKEN_DESCRIPTION_MIN,
  SPOKEN_PREVIEW_MIN,
  type SpokenDraftInput,
  type SpokenDraft,
  type SpokenWorkInput,
  type SpokenWorkQuote,
} from '@/entities/spoken-voice'
export interface CreationAddress {
  draft?: string
  operation?: string
  qualification?: string
  copy?: string
}
export function useVoiceCreation(
  ownerId: string,
  address: CreationAddress,
  onAddress: (next: CreationAddress) => void,
  initialInput?: SpokenDraftInput,
) {
  const query = useSpokenDraft(ownerId, address.draft ?? ''),
    profiles = useSpeechProfiles(address.qualification ?? ''),
    actions = useSpokenActions(ownerId),
    work = useSpokenWorkActions(ownerId),
    job = useSpokenOperation(ownerId, address.operation ?? '')
  const draft = query.draft
  const [edited, setEdited] = useState<SpokenDraftInput | null>(null),
    [failure, setFailure] = useState<unknown>(),
    [note, setNote] = useState(''),
    [approval, setApproval] = useState<{
      quote: SpokenWorkQuote
      input: SpokenWorkInput
      key: string
    } | null>(null)
  const blank: SpokenDraftInput = {
    name: '',
    description: '',
    previewText: SPOKEN_AUDITION_TEXT,
    profileId: '',
    profileRevision: 0n,
    qualificationSessionId: address.qualification ?? '',
  }
  const saved: SpokenDraftInput = draft
    ? {
        name: draft.name,
        description: draft.description,
        previewText: draft.previewText,
        profileId: draft.profile.id,
        profileRevision: draft.profile.revision,
        qualificationSessionId: draft.qualificationSessionId,
      }
    : (initialInput ?? blank)
  const input = edited ?? saved
  const selected = profiles.profiles.find(
    (p) => p.id === input.profileId && p.revision === input.profileRevision,
  )
  const dirty =
    edited !== null &&
    Object.keys(saved).some(
      (k) => saved[k as keyof SpokenDraftInput] !== input[k as keyof SpokenDraftInput],
    )
  const valid =
    !!selected?.available &&
    input.name.trim().length > 0 &&
    [...input.name.trim()].length <= SPOKEN_NAME_MAX &&
    [...input.description.trim()].length >= SPOKEN_DESCRIPTION_MIN &&
    [...input.description.trim()].length <= selected.descriptionMax &&
    [...input.previewText.trim()].length >= SPOKEN_PREVIEW_MIN &&
    [...input.previewText.trim()].length <= selected.previewMax
  const running = !!job.operation && spokenOperationActive(job.operation.state)
  const busy = running || actions.pending || work.pending
  const edit = (patch: Partial<SpokenDraftInput>) => {
    setEdited({ ...input, ...patch })
    setApproval(null)
    setNote('')
    setFailure(undefined)
  }
  async function save(): Promise<SpokenDraft> {
    const key = crypto.randomUUID()
    const next = draft
      ? await actions.update(draft.id, draft.revision, input, key)
      : await actions.create(input, key)
    setEdited(null)
    onAddress({ ...address, draft: next.id })
    return next
  }
  async function act(fn: () => Promise<unknown>) {
    setFailure(undefined)
    setNote('')
    try {
      await fn()
    } catch (error) {
      setFailure(error)
    }
  }
  async function estimate(confirm = false) {
    await act(async () => {
      let current = draft
      if (!confirm && (!current || dirty)) current = await save()
      if (!current) throw new Error('Missing saved voice draft')
      const workInput: SpokenWorkInput = {
        kind: confirm ? 'voice_confirm' : 'voice_design',
        draftId: current.id,
        revision: current.revision,
        candidateId: confirm ? current.selectedCandidateId : undefined,
      }
      const quote = await work.quote(workInput)
      if (quote.existingVoiceId) {
        setNote('complete')
        return
      }
      setApproval({ quote, input: workInput, key: crypto.randomUUID() })
    })
  }
  async function start() {
    if (!approval) return
    const approved = approval
    await act(async () => {
      const operation = await work.start(approved.input, approved.key, approved.quote)
      setApproval(null)
      onAddress({ ...address, draft: approved.input.draftId, operation: operation.id })
    })
  }
  async function select(candidateId: string) {
    if (!draft) return
    await act(() => actions.select(draft.id, draft.revision, candidateId, crypto.randomUUID()))
  }
  async function acknowledge(candidateId: string, playbackId: string) {
    if (!draft || draft.candidates.find((c) => c.id === candidateId)?.auditionedAt) return
    const next = await actions.acknowledge(
      draft.id,
      draft.revision,
      candidateId,
      playbackId,
      crypto.randomUUID(),
    )
    setApproval(null)
    return next
  }
  const candidate = draft?.candidates.find((c) => c.id === draft.selectedCandidateId)
  return {
    query,
    profiles,
    draft,
    input,
    selected,
    dirty,
    valid,
    busy,
    running,
    job,
    approval,
    failure,
    note,
    edit,
    estimate,
    start,
    select,
    acknowledge,
    setFailure,
    playbackFailed: () => {
      setNote('playFailed')
      setFailure(undefined)
    },
    closeApproval: () => setApproval(null),
    save: () =>
      act(async () => {
        await save()
        setNote('saved')
      }),
    cancel: () =>
      act(async () => {
        if (job.operation) await work.cancel(job.operation.id)
      }),
    recover: () =>
      act(async () => {
        if (job.operation) await work.recover(job.operation.id)
      }),
    confirmable:
      !(
        job.operation?.kind === 'voice_confirm' &&
        ['received', 'unresolved', 'cancelled'].includes(job.operation.state)
      ) &&
      !!candidate?.auditionedAt &&
      !dirty &&
      !busy &&
      !!selected?.available,
  }
}
