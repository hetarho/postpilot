import { useState } from 'react'
import { POST_TAG_COUNT_DEFAULT } from '@/entities/post'

/** One value the brief EDITS and the server OWNS. The local copy follows the server's whenever
 *  the server's moves, and is otherwise the owner's to change — and it is adjusted DURING RENDER
 *  rather than from an effect, because an effect would paint one frame carrying the value the
 *  server has already moved past. */
function useMirrored<S, T>(server: S, project: (value: S) => T) {
  const [value, setValue] = useState<T>(() => project(server))
  const [seen, setSeen] = useState(server)
  if (seen !== server) {
    setSeen(server)
    setValue(project(server))
  }
  return [value, setValue] as const
}

/** The two brief fields the generate action sends or the server reads off the post: the target
 *  length (GEN) and the tag count (GEN-46, which no start call carries). The brief widget SETS
 *  them and the action SENDS them from different layers, so the screen they both hang off owns
 *  the value. */
export function useBriefMirror(post?: { targetLength?: number; tagCount?: number }) {
  const [targetLength, setTargetLength] = useMirrored(post?.targetLength, (value) => value)
  const [tagCount, setTagCount] = useMirrored(
    post?.tagCount,
    (value) => value ?? POST_TAG_COUNT_DEFAULT,
  )
  return { targetLength, setTargetLength, tagCount, setTagCount }
}
