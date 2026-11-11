import { useEffect } from 'react'
import { completeMyWorkClipReturn, readMyWorkClipReturn } from '@/entities/clip-project'
import { completeMyWorkPostReturn, readMyWorkPostReturn } from '@/entities/post'

/** Work opened from 내 글 전체 comes back to where the owner left the list (THEME-58 →THEME-69).
 *  The editor's or the workspace's way back marks the return; once both lists have loaded, the
 *  page is tall enough to hold that position again and goes there, once. */
export function useMyWorkScrollReturn(ownerId: string, loaded: boolean) {
  useEffect(() => {
    if (!ownerId || !loaded) return
    const entry = readMyWorkPostReturn(ownerId) ?? readMyWorkClipReturn(ownerId)
    if (!entry) return
    const frame = requestAnimationFrame(() => {
      window.scrollTo(0, entry.scrollY)
      completeMyWorkPostReturn(ownerId)
      completeMyWorkClipReturn(ownerId)
    })
    return () => cancelAnimationFrame(frame)
  }, [ownerId, loaded])
}
