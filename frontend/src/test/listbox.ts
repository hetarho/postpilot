import { screen } from '@testing-library/react'
import type userEvent from '@testing-library/user-event'

/** Drives the app-drawn `shared/ui` Listbox the way `user.selectOptions` drove the native select
 *  it replaced: its options exist in the DOM only while the panel is open, so a choice is two
 *  presses rather than one call.
 *
 *  A trigger is found with `getByRole('combobox', { name })`, and that name is now
 *  "<label> <current value>" — the WAI-APG select-only combobox shape — so query it with a
 *  pattern rather than the label alone. */
export async function chooseOption(
  user: ReturnType<typeof userEvent.setup>,
  trigger: HTMLElement,
  option: string | RegExp,
) {
  await user.click(trigger)
  await user.click(await screen.findByRole('option', { name: option }))
}
