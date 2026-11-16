const HIDDEN_KEY = 'postpilot.templateContentHelp.hidden'

/** Whether the owner asked not to be shown the explanation on arrival again. A per-browser
 *  convenience: storage that is blocked or cleared only means it shows once more. */
export function contentKindHelpHidden(): boolean {
  try {
    return window.localStorage.getItem(HIDDEN_KEY) === '1'
  } catch {
    return false
  }
}

export function rememberContentKindHelpHidden(hidden: boolean) {
  try {
    if (hidden) window.localStorage.setItem(HIDDEN_KEY, '1')
    else window.localStorage.removeItem(HIDDEN_KEY)
  } catch {
    // Nothing to keep: the explanation simply opens again next time.
  }
}
