import type { ClipPreparationScreen } from '@/features/edit-clip-project'

/** A required answer missing at run start brings its screen back with that field focused
 *  (CLIP-225); the screen itself is the caller's to select first. */
export function focusPreparationField(target: ClipPreparationScreen) {
  window.requestAnimationFrame(() =>
    document
      .querySelector<HTMLElement>(
        `[data-clip-screen="${target}"] [aria-invalid="true"]:not([disabled])`,
      )
      ?.focus(),
  )
}
