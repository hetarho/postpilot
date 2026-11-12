import { CLIP_SOURCE_CONTAINERS, CLIP_SOURCE_MAX_FILE_BYTES } from '@/entities/clip-design'
import { EDITOR_HANDOFF_TTL_MS } from '@/shared/config'

/** A source picked on an unminted `/clips/new` mints the project first (CLIP-130); its upload
 *  belongs to the minted id, whose workspace the address change mounts. The files ride this
 *  one-shot handoff to that workspace's upload session, which claims them once it is active. */
let pending: { projectId: string; files: readonly File[]; at: number } | undefined

export function stashClipSourcePick(projectId: string, files: readonly File[]): void {
  pending = { projectId, files: [...files], at: Date.now() }
}

/** Takes the handed files for this project, once. The TTL drops a pick nothing claimed. */
export function claimClipSourcePick(projectId: string): readonly File[] | undefined {
  if (!pending || pending.projectId !== projectId) return undefined
  const { files, at } = pending
  pending = undefined
  return Date.now() - at > EDITOR_HANDOFF_TTL_MS ? undefined : files
}

/** Whether the picker would read this file at all: a supported container within the size bound.
 *  A pick made only of files it would skip mints nothing (CLIP-130, POST-7). */
export function pickableClipSource(file: File): boolean {
  const extension = file.name.split('.').pop()?.toLowerCase() ?? ''
  const types = Object.values(CLIP_SOURCE_CONTAINERS).flat()
  return (
    file.size > 0 &&
    file.size <= CLIP_SOURCE_MAX_FILE_BYTES &&
    (Object.hasOwn(CLIP_SOURCE_CONTAINERS, extension) || types.includes(file.type))
  )
}
