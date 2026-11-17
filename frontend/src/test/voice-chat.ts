import { screen } from '@testing-library/react'
import type userEvent from '@testing-library/user-event'

/** The learning chat opens on one experience (VOICE-78): `생각나는 게 없어요` on a first turn and
 *  `다른 장면으로 답하기` on a later one take the imagined situations, which is where a screen test
 *  about the answer form starts. */
export async function chooseSituations(user: ReturnType<typeof userEvent.setup>) {
  await user.click(
    await screen.findByRole('button', { name: /^(생각나는 게 없어요|다른 장면으로 답하기)$/ }),
  )
}
