import { useEffect, useRef, useState } from 'react'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { ClipSourceImportNotice } from '@/features/upload-clip-sources'

/** Keep the result on its owned destination, while consuming the history handoff once. */
export function ClipImportFeedback({ ownerId, clipId }: { ownerId: string; clipId: string }) {
  const incoming = useRouterState({ select: (state) => state.location.state.clipImport })
  const [report, setReport] = useState(incoming)
  const consumed = useRef(false)
  const navigate = useNavigate()
  useEffect(() => {
    if (!incoming || consumed.current) return
    consumed.current = true
    void navigate({
      to: '/clips/$clipId',
      params: { clipId },
      replace: true,
      resetScroll: false,
      state: (previous) => ({ ...previous, clipImport: undefined }),
    })
  }, [incoming, clipId, navigate])
  if (!report || report.ownerId !== ownerId || report.projectId !== clipId) return null
  return (
    <ClipSourceImportNotice
      problems={report.problems}
      uncertain={report.uncertain}
      onDismiss={() => setReport(undefined)}
    />
  )
}
