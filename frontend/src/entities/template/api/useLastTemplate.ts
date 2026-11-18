import { useSyncExternalStore } from 'react'
import { readLastTemplate, subscribeLastTemplate } from '../model/last-template'

/** The account's last specified template (`lastTemplate`), kept current as it is specified
 *  anywhere — 홈's 템플릿 dropdown, the post guide's 템플릿 step, another tab. */
export function useLastTemplate(ownerId: string): string | undefined {
  return useSyncExternalStore(
    subscribeLastTemplate,
    () => readLastTemplate(ownerId),
    () => undefined,
  )
}
