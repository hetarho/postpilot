import { useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  clipPlanToProto,
  clipProjectsKey,
  copyClipPlan,
  editClipPlan,
  toClipProject,
  validateClipPlan,
  type ClipEdit,
  type ClipProject,
} from '@/entities/clip-project'
import { ClipService, appFailureFromConnect } from '@/shared/api'

export function useClipCorrection(ownerId: string, project: ClipProject) {
  const transport = useTransport()
  const cache = useQueryClient()
  const initial = project.editing?.plan ?? { durationMs: 0, cuts: [] }
  const [draft, setDraft] = useState(() => copyClipPlan(initial))
  const [baseline, setBaseline] = useState(() => JSON.stringify(initial))
  const [revision, setRevision] = useState(project.editPlanRevision)
  const saving = useRef(false)
  const dirty = JSON.stringify(draft) !== baseline
  const validation = project.editing ? validateClipPlan(draft, project.editing) : undefined
  const mutation = useMutation({
    mutationFn: async () => {
      const result = await createClient(ClipService, transport).saveClipEditPlan({
        projectId: project.id,
        expectedRevision: revision,
        plan: clipPlanToProto(draft),
      })
      if (!result.project?.editing) throw new Error('Missing saved correction')
      return toClipProject(result.project)
    },
    retry: false,
  })
  function adopt(next: ClipProject) {
    if (!next.editing) return
    setDraft(copyClipPlan(next.editing.plan))
    setBaseline(JSON.stringify(next.editing.plan))
    setRevision(next.editPlanRevision)
  }
  // Reconcile only a clean draft during render; dirty edits never adopt a remote
  // revision. React restarts this render before children see a mixed snapshot.
  if (!dirty && !mutation.isPending && project.editing && revision !== project.editPlanRevision) {
    setDraft(copyClipPlan(project.editing.plan))
    setBaseline(JSON.stringify(project.editing.plan))
    setRevision(project.editPlanRevision)
  }
  const change = (edit: ClipEdit) => {
    if (saving.current || !project.editing) return
    mutation.reset()
    setDraft((current) => editClipPlan(current, edit, project.editing!.fadeMs))
  }
  async function save() {
    if (saving.current || !dirty || !validation?.valid) return
    saving.current = true
    try {
      const result = await mutation.mutateAsync()
      adopt(result)
      cache.setQueryData([...clipProjectsKey(transport, ownerId), 'detail', project.id], result)
      void cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
    } catch {
      /* Revision conflicts preserve every local field. */
    } finally {
      saving.current = false
    }
  }
  const reload = useMutation({
    mutationFn: async () => {
      const result = await createClient(ClipService, transport).getClipProject({ id: project.id })
      if (!result.project?.editing) throw new Error('Missing current correction')
      return toClipProject(result.project)
    },
    retry: false,
    onSuccess: (next) => {
      adopt(next)
      mutation.reset()
      cache.setQueryData([...clipProjectsKey(transport, ownerId), 'detail', project.id], next)
    },
  })
  return {
    draft,
    revision,
    dirty,
    validation,
    change,
    save,
    reset: () => {
      adopt(project)
      mutation.reset()
    },
    pending: mutation.isPending || reload.isPending,
    failure: mutation.error
      ? appFailureFromConnect(mutation.error)
      : reload.error
        ? appFailureFromConnect(reload.error)
        : undefined,
    reload: () => reload.mutate(),
  }
}
