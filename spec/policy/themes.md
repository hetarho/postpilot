# Theme policy

## Supported preferences and effective themes

Postpilot supports exactly three browser-owned preferences:

- `system` follows `prefers-color-scheme`;
- `light` resolves to the `day` semantic map; and
- `dark` resolves to the `night` semantic map.

`system` is the default. The effective theme is always exactly `day|night`; components consume
semantic or functional design tokens and never branch on a preference, an effective theme, or a
palette step.

## Persistence and failure behavior

The fixed storage key is `localStorage['postpilot.theme']`. It contains only the explicit values
`light|dark`; choosing System removes the key. A missing, malformed, unknown, unreadable, or
throwing value resolves to System. Storage read, write, and removal failures never prevent the
application from rendering, and a failed write leaves the current tab's in-memory selection
active for that session.

Theme is not account or session state. It survives navigation, logout, later login, and reload on
the same browser, but it has no RPC, database row, cross-device synchronization, provider call,
job, analytics event, or environment value.

## Bootstrap and runtime lifecycle

The application resolves and applies one synchronous bootstrap snapshot before React creates its
root. The mounted provider starts from that snapshot instead of rereading storage, so the first
application frame and runtime state cannot disagree.

While System is active, the provider listens to
`matchMedia('(prefers-color-scheme: dark)')`. An explicit Light or Dark selection detaches that
listener and ignores later OS changes. A same-origin `storage` event for `postpilot.theme` updates
the open tab without writing the event back; a removed, cleared, or invalid value means System.
Media and storage listeners are cleaned and reattached safely under React Strict Mode.

## Document and browser chrome synchronization

Every effective-theme transition atomically synchronizes:

- `<html data-theme="day|night">`;
- the document element's native `color-scheme`;
- `<meta name="color-scheme">`; and
- `<meta name="theme-color">`.

The day/night browser-chrome colours are fixed design values owned by `frontend/index.html` as
the theme-colour element's `data-day` and `data-night` attributes. The runtime selects one of
those values; it does not duplicate raw colours in TypeScript. The static/no-JavaScript fallback
is Night. Theme changes are immediate and add no full-page colour transition. The fixed app-icon
artwork does not change with theme.

## Interface contract

The translated `InterfacePreferences` surface is the single reusable public composition of
locale and theme controls. It is available on login and every authenticated route, and plan 15's
public page reuses the same widget. Theme and locale remain independent: changing either one does
not change the other, the URL, session state, query state, or server data.

The control exposes a translated accessible name and native System/Light/Dark select, preserves
the 44 px target floor, traps focus while its popover is open, closes on Escape with focus return,
and uses a compact icon trigger at phone widths so locale, theme, and session actions do not cause
horizontal overflow.

## Fixed code contracts

| Value                              | Owner                          | Value                                  |
| ---------------------------------- | ------------------------------ | -------------------------------------- |
| Theme storage key                  | FE `shared/config`             | `postpilot.theme`                      |
| Default preference                 | FE `shared/config`             | `system`                               |
| System media query                 | FE `shared/lib/theme`          | `(prefers-color-scheme: dark)`         |
| Supported preferences              | FE `shared/lib/theme`          | `system`, `light`, `dark`              |
| Supported effective semantic maps  | FE design system               | `day`, `night`                         |

These are product and web-design contracts, not deployment configuration.
