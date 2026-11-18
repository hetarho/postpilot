import { useSession } from '@/entities/session'
import type { KitKey } from '../model/kit'
import { useWritingKit } from '../model/useWritingKit'

/** One piece of the signed-in owner's kit, read from the same queries 홈's picture reads. */
export function useKitElement(kit: KitKey) {
  const { user } = useSession()
  return useWritingKit(user?.id ?? '').elements[kit]
}
