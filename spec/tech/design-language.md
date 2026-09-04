# Tech — Design language

What the postpilot interface looks like, and the rules every frontend change is held to. This is
the **binding style guide** for `frontend/src`: `/implement-job` reads it before touching any UI
surface, and `pnpm lint:style` enforces the mechanical half of it. The architectural frame is
[ARCHITECTURE.md §3](../ARCHITECTURE.md) (Feature-Sliced Design); this doc owns what the slices
_look like_.

Owner in code: `frontend/src/app/styles/index.css` (the tokens) and `frontend/src/shared/ui/*`
(the primitives, built incrementally — see §1).

## 0. The premise

Postpilot is a tool you use on a phone, often one-handed, to get a post out of your head and onto
a server before the moment passes (PRD F-2). Everything visual serves that:

- **Content is the interface.** A post's title, its text, its photos are the largest, brightest,
  most central things on the screen. Chrome is small, quiet, and at the edges.
- **Planes, not lines.** Structure is shown by _where a surface changes colour_, not by drawing
  boxes around things. A border is the last tool, not the first (§4).
- **Colour means something or it is absent.** A hue is a role — the primary action, a danger, a
  status. The rest of the interface is a near-monotone mauve so that the one violet thing on the
  screen is unmistakably the thing to press.
- **Nothing announces itself.** Motion confirms; it does not perform. Emphasis is the smallest
  amount that works.

### 0.1 The device contract

"On a phone" is a measurable claim, not a mood. Every surface is designed and reviewed against
this contract, and a change that has only been looked at in a desktop browser is not reviewed:

| Constraint        | Value                                                                                                                                                                         |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Design width      | **360 CSS px** — the realistic small-Android floor. 390 and 430 are also checked                                                                                              |
| Conformance floor | **320 CSS px** with no horizontal page scroll (WCAG 1.4.10 Reflow, AA)                                                                                                        |
| Grip              | One hand. On a 430×932 phone the top ~40% of the glass is out of thumb reach                                                                                                  |
| Software keyboard | Covers roughly the **bottom 40%** of the screen whenever a field has focus                                                                                                    |
| Script            | Korean. A Hangul syllable advances a full em — Korean text is ~1.8× wider per character than the same count of Latin ones, and it breaks between syllables unless told not to |
| Network           | Cellular. Every RPC can be slow, and every RPC can fail                                                                                                                       |

Two consequences follow that are easy to forget on a 27-inch monitor. A Korean label is much
wider than its character count suggests, so any row that survives an English mock can still
overflow. And "below the fold" on a 640px screen is ~1.2 screenfuls, not "a bit further down".

## 1. The five hard rules

These are not preferences. A PR that breaks one is not done.

### 1.1 Only design-system UI

Every generic control that the product currently renders lives in
`frontend/src/shared/ui/<name>/` and is used from there. **Slices never hand-roll a control**
with a bare `<button className=…>`.

The primitive inventory is demand-driven, not a kit to complete in advance. For each UI need:

1. Use the existing `shared/ui` primitive when it satisfies the contract.
2. If none exists, add the smallest domain-agnostic primitive needed by the current screen,
   then use it from the slice.
3. Do **not** prebuild unused controls, variants, states, or composition helpers for hypothetical
   future screens. Popover, dialog, toast, switch, checkbox, skeleton, tooltip, and any other
   primitive are created only when an implemented screen needs them.

A primitive is written once to this document's rules, and every later slice inherits it. A
generic control hand-rolled inside one slice is a design-system bug, not a shortcut; a generic
control that no current screen needs is unnecessary inventory.

What counts as a primitive: anything with no product noun in its name. `Button`, `TextField`,
`Menu` are primitives. `PhotoStrip`, `SaveStatus`, `PostRow` are not — they are slice UI composed
_from_ primitives.

**The corollary is a smell test.** If two slices reach for the same shape — a sticky bar of
committing actions, a scrolling tab row, a pending button — that shape is a missing primitive,
and the second slice's copy is the bug report.

### 1.2 Only Tailwind-registered tokens

A class must resolve to a token this repo registered in `@theme` — the colour foundations and
functional roles (§2), the radius/duration/ease/shadow/font scale, the safe-area spacing tokens
(§8.1), and Tailwind's stock spacing and type scales. Use the narrowest role that describes the
element: a CTA uses `button-cta-*`, a field uses `field-*`, and an inline error uses
`notice-danger-*`. Surface and content foundations are for page composition and typography, not a
shortcut around a component role.

Forbidden, with no exceptions in `frontend/src`:

- Tailwind's **stock colour palette** (`bg-gray-100`, `text-red-400`, `border-neutral-800`). It is
  removed in `index.css` (`--color-*: initial`), so such a class emits **no CSS at all** — the UI
  silently loses its styling. `pnpm lint:style` fails on any occurrence.
- **Arbitrary colour values** (`bg-[#1a1a2e]`, `text-[oklch(…)]`) and raw colour literals in
  TS/TSX/CSS. Same gate.
- **Arbitrary sizes** off the scale (`p-[13px]`, `rounded-[7px]`, `text-[15px]`, `max-h-[65vh]`).
  Density is chosen by picking a step. If two steps are both wrong, the scale is wrong — change
  the scale in `index.css`, not the call site. The sole 10px recipe is the `eyebrow` category chip
  owned by `Typography`; do not spread it. `pnpm lint:style` catches arbitrary text sizes and
  weights in slice `.tsx` alongside colour escapes. Non-type geometry escapes, and arbitrary type
  inside exempt `shared/ui` primitives, remain review responsibilities.

A line that is genuinely not UI colour — a canvas compositing fill, a test fixture — carries an
inline `// style-escape: <why>` pragma. The reason is mandatory.

The retired `bg` / `surface` / `text` / `primary` / bare status / `border` / `overlay` / `depth`
vocabulary is not exported. `pnpm lint:style` rejects those utility families, and
`pnpm lint:style:probe` proves the production scanner catches representative retired classes
without rejecting the current surface/content and functional-role names.

### 1.3 Borders are the last resort

Depth and grouping come from the five **surface steps** (`surface-lowest` → `surface-recessed` →
`surface-base` → `surface-raised` → `surface-highest`) and from spacing. A `border-*` class is
allowed only where a plane change _cannot_ do the job:

- the **outlined** button variant (a rim _is_ its identity),
- a hairline `divide-divider` between list rows that have no background of their own,
- a table rule,
- a spinner's ring, where the border _is_ the shape being drawn.

Not allowed: a border around a card, a panel, an input at rest, a header, a section. If you reach
for `border` to "separate" two things, step one of them to a different surface or add space.

### 1.4 Cards are rare

A card is a raised surface with its own padding and radius. It says "these things belong together
_and_ are separate from their neighbours". Use one **only** when both halves are true — a set of
controls that act on one object, a preview that must read as a single unit, an item in a grid of
peers.

Not a card: a page section (use a heading and spacing), a form (use spacing), a list (use rows and
a `divide-divider` hairline), a single paragraph, the whole page content. A card whose only content
is one line of text is always wrong. Never nest a card in a card.

### 1.5 The base breakpoint is the phone

Every layout is authored **unprefixed for a 360px phone**, and `sm:` / `md:` only ever override
_upward_. `sm:` is never used to mean "on small screens" — Tailwind's breakpoints are `min-width`,
so `sm:grid-cols-2` means "two columns from 640px up", and the unprefixed classes beside it are
the phone design.

The practical test: **delete every `sm:` and `md:` class from a component and it must still be a
finished screen.** A component whose only layout is `sm:grid-cols-2` has no phone design — it has
a desktop design and a fallback. A table, a side-by-side pane, or a multi-column grid with no
unprefixed alternative is not responsive; it is a desktop layout that happens to reflow.

Three breakpoints carry meaning, and no others are introduced without a reason written down:

| Prefix | Width   | What changes                                                                    |
| ------ | ------- | ------------------------------------------------------------------------------- |
| _none_ | ≥ 0     | The phone. Single column, one scroller, thumb-reachable actions                 |
| `sm:`  | 640 px  | **Density** — tighter type, restored horizontal padding, a second grid column   |
| `md:`  | 768 px  | **Shape** — the pointer is assumed fine: side-by-side panes, dialog-not-sheet   |
| `lg:`  | 1024 px | **The desk** — the shell grows a sidebar and the content column widens (§4.5)   |

`lg:` is the newest and the narrowest in scope: it changes the SHELL and the width of the content
column, and nothing else. A component does not reach for it to rearrange itself — that is still
`md:`'s job, or a `@container` query. Owner decision 2026-09-05.

A breakpoint is chosen because the _content_ breaks there, not because a device is that wide.
When a component's shape depends on its container rather than the window, use a `@container`
query instead of a viewport breakpoint.

## 2. Colour

### 2.1 Three layers, one file

Colour is authored once, in `frontend/src/app/styles/index.css`:

1. **Palette** — `--palette-<hue>-<step>`, OKLCH ramps (`mauve`, `violet`, `red`, `green`,
   `gold`, `blue`, plus `white`/`black`). These are the only raw colour literals in the frontend.
   They are **not** Tailwind theme variables, so no utility can reach them — by design.
2. **Semantic foundations** — `--surface-*`, `--content-*`, `--stroke-*`, `--intent-*`,
   `--interaction-*`, and `--physical-*`. A theme block maps every foundation to a palette step,
   never to a literal. Foundations say what a colour means independently of any component.
3. **Functional roles** — `--button-cta-*`, `--field-*`, `--notice-*`, `--badge-*`, `--row-*`,
   and the other things the current product renders. These reference semantic foundations, not
   palette steps. `@theme inline` exposes functional roles and the surface/content foundations
   required for page composition as Tailwind colours.

The dependency direction is one way: **palette → foundation → function → component**. A function
never reaches back to a palette step, and a primitive never reaches around its function to an
intent colour. Therefore two roles that must match cannot drift, a theme remains a data change,
and changing an accent does not require finding button classes throughout the app.

### 2.2 The neutral is mauve

The greys carry the primary's hue (298°) at a constant, whisper-level chroma (~0.01, about 5% of
the violet ramp's peak). Side by side with a true grey they read faintly cool and violet; alone
they read as grey. This is what makes the interface feel like _one_ material rather than a grey
box with a purple button on it. Never use a true-grey or a different-hue neutral.

### 2.3 Semantic foundations

The five surfaces form an ordered depth scale. Use the smallest step that makes the relationship
clear; jumping several levels makes ordinary chrome look detached from the page. `surface-overlay`
is a sixth, and it is not part of that scale: it exists to be told APART from another surface.

| Foundation                                                  | Meaning                                                                                 |
| ----------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `surface-lowest`                                            | The furthest-back plane; deep wells and deliberately sunken regions                     |
| `surface-recessed`                                          | Inputs and regions pressed one step into the current plane                              |
| `surface-base`                                              | The normal page canvas                                                                  |
| `surface-raised`                                            | Rows on interaction, compact controls, cards, and resting floating content              |
| `surface-highest`                                           | The frontmost floating plane: menus, sheets, dialogs, popovers, and a docked action bar |
| `surface-overlay`                                           | A panel that opens OVER one of the above — a portalled listbox panel. It must differ from `highest`, `base` and `raised` at once, so it is a step DOWN from them plus its own `shadow-lg`: up is unavailable (`highest` is already white in `day`, and a plane lighter than `highest` in `night` drops `content-secondary` below AA). Never used for a surface that is not opening over another one |
| `content-primary` · `secondary` · `tertiary` · `disabled`   | Main copy → supporting copy → metadata → non-interactive copy                           |
| `stroke-subtle` · `strong`                                  | Rare structural hairlines; never a substitute for a surface step                        |
| `intent-accent` · `danger` · `success` · `warning` · `info` | Meaning before it is assigned to a component                                            |
| `interaction-focus`                                         | The one keyboard-focus colour                                                           |
| `physical-scrim` · `physical-on-scrim` · `physical-shadow`  | Physical light/occlusion, not product meaning                                           |

Primary, secondary, and tertiary content meet WCAG AA against all five surfaces in both themes.
Disabled content is intentionally exempt because it is not an available action; never use it for
readable metadata or explanatory copy.

### 2.4 Functional roles

Functional roles are grouped by what the screen renders. Add a role only when a current UI need
cannot be expressed by an existing group; do not create a speculative component-token catalogue.

| Group           | Roles currently defined                                                                                       |
| --------------- | ------------------------------------------------------------------------------------------------------------- |
| Buttons         | `button-cta-*`, `button-secondary-*`, `button-ghost-*`, `button-danger-quiet-*`                               |
| Fields          | `field-bg`, `field-bg-hover`, `field-bg-focus`, `field-fg`, `field-label`, `field-placeholder`, `field-error` |
| Navigation/list | `link-*`, `link-fg-current`, `row-bg-*`, `divider`                                                            |
| Labels/feedback | `badge-{neutral,accent}-*`, `notice-{danger,success,warning,info}-*`                                          |
| System/media    | `focus-ring`, `selection-*`, `media-scrim-*`, `shadow-color`                                                  |
| Brand           | `brand-wordmark`, `brand-mark`                                                                                |

State names are explicit (`bg`, `bg-hover`, `bg-active`, `fg`) so a primitive never invents a
hover colour by changing opacity. Function names describe purpose: the committing action is
`button-cta`, not "purple" or a provider/model-specific name.

### 2.5 Themes

A theme is one `[data-theme='<key>']` block re-mapping the semantic foundations. `night` (dark,
the static/no-JavaScript fallback on `:root`) and `day` (light) ship. The browser-owned
preference is `system|light|dark`: System is the default and follows `prefers-color-scheme`, while
Light and Dark explicitly select `day` and `night`. Nothing in a component changes, because
nothing in a component names a palette step — that is the point of the layers.

The app resolves the valid `localStorage['postpilot.theme']` override and OS preference
synchronously before React creates its root, applies one bootstrap snapshot, and then mounts one
provider from that same snapshot. Choosing System removes the key. Missing, malformed, or
throwing storage is non-fatal. The provider observes OS changes only in System mode and consumes
same-origin storage events without writing them back; route and session changes do not reset it.
Locale and theme remain independent browser preferences.

Both themes must keep primary/secondary/tertiary content on every surface, and every functional
foreground on its background, at WCAG AA (4.5:1 for body, 3:1 for large text). Check all mapped
pairs with a contrast tool when a foundation moves. For Korean, "large text" is the CJK size
equivalent of 18 pt, not the Latin pixel value — do not claim the 3:1 exemption on 19px Hangul.

The browser chrome is part of the theme. Every transition synchronizes `<html data-theme>`, the
document's native `color-scheme`, `<meta name="color-scheme">`, and `<meta name="theme-color">`
from the same effective theme. The theme-colour element in `index.html` owns the raw day/night
header-plane values as `data-day` and `data-night`; TypeScript selects them instead of duplicating
colour literals. Chrome for Android tints its address bar from that tag alone, so `color-scheme`
cannot substitute for it. Theme changes are immediate and introduce no full-page transition.

The **wordmark** is part of the theme too. `shared/ui/Logo` is inline SVG drawn from
`brand-wordmark` (the page's own ink) and `brand-mark` (the accent), so it re-skins with every
theme instead of freezing one palette into an `.svg`. It carries its own accessible name, so the
element around it — the header's home link, the login `h1` — adds no second text label. Size it by
height (`h-6` in the header, `h-9` on login); the trimmed viewBox supplies the width.

The **app icon** is the one thing that does _not_ follow the theme: it is fixed art on its own
near-black tile, because the OS and the browser tab strip render it, not the page.
`frontend/public/favicon.svg` is the source; `favicon.ico` (16/32/48) and `apple-touch-icon.png`
(180 — what iOS uses for 홈 화면에 추가, where Safari ignores SVG) are rasterised from it with
headless Chrome, and must be regenerated when it changes. All three are linked from `index.html`.

### 2.6 Applying colour

- **Surfaces move one physical step.** A field is `surface-recessed` on `surface-base`; a card is
  `surface-raised`; a modal is `surface-highest`. Depth is the step plus (rarely) a shadow — not a
  border. `surface-lowest` is not a generic dark fill and `surface-highest` is not emphasis.
- **One CTA per view.** The committing action uses `button-cta-*`. Every other button is ghost,
  secondary, or, rarely, outlined. Two filled CTAs on one screen means one of them is lying about
  its importance.
- **Accent is for action and identity, not area.** A large violet field reads as an alarm. The
  accent lives on a control, a selected state, a focus ring, a single word.
- **Status colour never travels alone.** A red text always says what is wrong; a coloured dot is
  always beside a label. Colour-blind users lose nothing.
- **Inline notices use the notice contract.** For example,
  `bg-notice-danger-bg text-notice-danger-fg`, with explanatory text and no border.
- **Use functions, not foundations, inside primitives.** `Button` uses `button-*`; `Field` uses
  `field-*`. Direct `intent-*` utilities are deliberately not registered with Tailwind.

## 3. Typography

One family: the system stack (`--font-sans`, with Apple SD Gothic Neo / Pretendard / Noto Sans KR
for Korean). No second family, no web-font download — the app opens on a phone and must paint at
once. Weight carries emphasis (`font-medium`, `font-semibold`); colour carries importance
(`content-primary` → `content-secondary` → `content-tertiary`); size carries hierarchy. Never use colour _and_ weight
_and_ size to say the same thing.

**The roles are code, not conventions.** `shared/ui/typography` owns the recipes:
`<Typography variant="…">` renders slice text (headings, prose, labels, metadata), and
`typographyStyles({ variant })` hands the same recipe to an element that must keep its own
component — a router `Link`, a `dt`, a field's `className`. A slice **never composes a raw type
utility** (`text-<size>`, `font-<weight>`, `font-mono`, `tracking-*`, `leading-*`);
`pnpm lint:style` fails on one in a non-test slice `.tsx` outside `shared/ui`, and the rare line
that genuinely is not a §3 role carries an inline `// style-escape: <why>` pragma the reviewer
reads. Primitives in `shared/ui` keep composing their own internals from the raw utilities — that
is where recipes live. Owner decision, 2026-09-01 (change 11): per-element type CSS is hierarchy
drift by construction, so the roles became a component.

| Role    | Variant · default element                                 | Recipe (owned by the primitive)                                         | When it is used                                                                                   |
| ------- | --------------------------------------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| display | `variant="display"` · `h1`                                | `text-2xl font-semibold tracking-tight`                                 | the screen's ONE logical top title; the editor's sr-only `h1` mirrors its visible editable display and is not a second title |
| title   | `variant="title"` · `h2` (`as="h3"` for deeper levels)    | `text-lg font-semibold tracking-tight`                                  | every section heading and dialog title; one look per heading level per screen                     |
| fieldTitle | `variant="fieldTitle"` · `h3` (`as="label"` for a field) | `text-base font-bold tracking-tight`                                 | a FIELD's own name where it stands beside a step's action rather than under a heading — smaller than the step title so the outline is unchanged, heavier so it still reads as the name of a control (글 다듬기's 수정 요청을 입력하세요) |
| body    | `variant="body"` · `p`                                    | `text-sm leading-relaxed`                                               | prose the user reads; loading/empty/status lines take `className="text-content-tertiary"` over it |
| input   | field primitives only                                     | `text-base sm:text-sm`                                                  | **anything the user types into** — see §3.1; never exposed as a `Typography` variant              |
| label   | `variant="label"` · `span` (`as="p"`/`as="h3"` as needed) | `text-sm text-content-secondary`                                        | a control caption, a secondary line, a pseudo-heading — with **no ad-hoc weight added**           |
| meta    | `variant="meta"` · `span`                                 | `text-xs text-content-tertiary`                                         | timestamps, counts, status chips' neighbours; `mono` for verbatim values (ids, slugs, model ids)  |
| eyebrow | `variant="eyebrow"` · `span`                              | `text-[10px] font-medium uppercase tracking-wide text-content-tertiary` | a category chip above a group                                                                     |

- A colour in the caller's `className` deliberately wins over the recipe's (the merge resolves
  toward the caller), so `body` + `text-content-tertiary` is the empty-state line and `label` +
  `text-link-fg` is a nav link — the SIZE stays the role's.
- Prose columns are capped: `max-w-measure` (`--container-measure`, 40rem).
- Field labels and placeholders use `text-field-label` and `text-field-placeholder`; a field
  primitive, not its caller, owns those choices.
- An eyebrow is not a heading; a heading is a real `<h*>` — `as` decouples the outline level from
  the visual role, so a third-level heading can still look like `title`.
- `text-xs` (12px) is the floor, and it is for metadata only. Explanatory copy the user is meant
  to act on is never 12px — if a sentence matters enough to render, it is `text-sm`.
- The sanctioned pragma case is a nav control's **active-state emphasis** (`font-medium` on
  `aria-current="page"` — a control state the roles do not model). Bare editor fields use the
  role builder or their field primitive and need no escape. Anything else asking for a pragma is
  a missing role — raise it here first.

### 3.1 The 16px input floor

**Every control the user types into computes to at least 16px on a phone.** iOS Safari zooms the
whole layout when a focused `<input>`, `<textarea>` or `<select>` has a font-size below 16px, and
it does **not** zoom back out on blur — one tap on a 14px field leaves the app horizontally
pannable for the rest of the session, at roughly 1.14× on a 390px screen.

The type role is `text-base sm:text-sm`: 16px where it matters, and the desktop density restored
from 640px up, where the behaviour does not exist. The field primitives own this; a caller that
sets its own size on the `bare` appearance owns it too.

The fix is the font size and nothing else. `user-scalable=no` and `maximum-scale=1` also suppress
the zoom, and both are forbidden — they break pinch-zoom for low-vision users (WCAG 1.4.4) and
iOS ignores them anyway.

### 3.2 Korean line breaking

The document is `lang="ko"` and the default UAX #14 behaviour lets a line break fall between any
two Hangul syllables, so `비교합니다` becomes `비교합` / `니다` in a narrow column. Two properties
are set once, globally, on `body`:

```css
word-break: keep-all; /* break between eojeol (words), not inside one */
overflow-wrap: anywhere; /* but still break a long Latin run rather than overflow */
```

`keep-all` alone would let an unbreakable Latin token — a model id, a slug, a provider error URL
— push the page into horizontal scroll, which is why the pair is inseparable. Never use
`break-all`: it is defined to exclude CJK and does the wrong thing here.

Any container that renders a **server or model-supplied string** — a filename, a slug, a
`providerId/modelId`, a raw provider error — additionally carries `break-words` at the call site.
The global rule protects the layout; the local class protects the box.

## 4. Space, hierarchy, density

- One 4px scale (Tailwind's). **Inside a control** 2–3 · **inside a group** 3–4 · **between
  groups** 6–8 · **between page sections** 10–16.
- Hierarchy is built in this order, each used only when the one before is not enough:
  **position → size → weight → colour → surface → border.**
- A group of actions reads left to right by rising emphasis: the way out first, the committing
  action last. The committing action is the only filled one.
- Full-bleed on mobile: the page has `px-4` (phone) → `px-6` (≥ sm) gutters and no visible
  container; content sits directly on `surface-base`.

### 4.1 Touch targets

| Rule                                                                   | Source                            |
| ---------------------------------------------------------------------- | --------------------------------- |
| **44 × 44 CSS px** is the house minimum for anything the thumb presses | Apple HIG; WCAG 2.5.5 (AAA)       |
| **24 × 24 CSS px** is the absolute conformance floor, never gone below | WCAG 2.5.8 Target Size (AA)       |
| **≥ 8 px of clear space** between two adjacent targets                 | Material / Lighthouse tap-targets |

Both dimensions count. `min-h-11` on a bare text link sets the height and leaves the **width** to
the label — and a two-syllable Korean label like `말투` is ~28px wide, so the target is 28 × 44
and fails. A control sized by its text always gets horizontal padding too.

When the visible box must stay small (a checkbox, a small glyph), the **primitive** grows the hit
area with padding or a pseudo-element — the caller is never asked to remember. A row in a list is
one target, not a row with a small button inside it.

### 4.2 Padding is a ratio, not a leftover

**`min-h-11` is a touch-target floor, not a padding value.** This is the single most common
density bug: a control sets `px-3 py-2 min-h-11`, the 44px floor overrides the computed height,
and the _effective_ vertical padding becomes 12px against 12px of horizontal — a 1 : 1 box.
Because text is far wider than it is tall, a 1 : 1 control always reads squat and bloated, and a
screen full of them reads clumsy no matter how good the type and colour are.

The rule: **a control's horizontal padding is roughly twice its effective vertical padding.**
Author the horizontal padding for the height the control will actually have, not for the padding
you wrote.

| Element                      | Padding                   | Effective ratio                                |
| ---------------------------- | ------------------------- | ---------------------------------------------- |
| Button, field, select (44px) | `px-4` (`px-5` for a CTA) | ≈ 2 : 1 against the 12px the floor produces    |
| Icon-only button             | `size-11`, no padding     | square by definition                           |
| List row (44px, full-bleed)  | `px-4 py-3`               | equal to the page gutter it sits in            |
| List row holding a control   | `px-4 py-2`, `min-h-16`   | the 44px control sits INSIDE the row's height  |
| Badge / chip                 | `px-2 py-0.5`             | ≈ 3 : 1 — small boxes need proportionally more |
| Inline notice                | `px-4 py-3`               | ≈ 1.3 : 1 — a text block, not a control        |
| Card / panel                 | `p-4` (`p-5` for a sheet) | uniform                                        |

Two corollaries. A **row** is never inset less than the gutter rhythm it sits in — `px-2` inside a
`px-4` page reads as a mistake, not as a nested step. And a control and the panel it sits in never
share a padding step for the same reason they never share a radius (§5).

And one rule about the row that holds a control: **its height floor is set by the tallest thing any
row in that list can hold, not by the rows that hold nothing.** A control keeps the 44px floor
(§4.1) wherever it sits, so a row that carries one is 44px plus its padding while its neighbours
that carry none stay at 44px total — and a list where one row is short and the rest are tall reads
as broken rather than as varied. Set the floor once, at the tall shape, and the control sits inside
the row instead of stretching it. The 말투 directory is the case: only the default voice offers
neither 기본으로 설정 nor 삭제, because the server refuses both for it.

### 4.3 Reach and placement

The thumb owns the lower half of the screen. On a 430 × 932 phone held one-handed, the top ~370px
is a re-grip, not a tap.

- **The committing action of a view lives in the lower band**, not in a top corner. `새 글`,
  `생성`, `저장`, `복사` — the action the screen exists for is where the thumb already is.
- **Not flush against the very bottom edge either.** Grips vary, and NN/g's bottom-sheet research
  is explicit that the extreme bottom is not the most reachable region; the middle-to-lower band
  is. A docked action bar with real padding satisfies both readings; a 20px strip at the screen
  edge satisfies neither.
- **Navigation is reachable from anywhere in the scroll.** A destination that only exists in a
  header at scroll-position 0 does not exist on a 4,000px page.
- **A control and the thing it commits stay on one screen.** A 저장 button in a section header
  above a 335px textarea is off-screen exactly when the user is typing into that field. The
  committing action goes _after_ the field, or it docks.
- **A dock for a list's add action is a PHONE dock.** `새 글`, `새 말투 만들기` — the reach argument
  is the only thing holding that bar up, and it evaporates with the thumb (`ActionBar`'s
  `dock="phone"`). Above the phone the card dissolves and the bar becomes the list's last row, with
  its action spanning the column: a floating, shadowed card carrying one left-aligned button over a
  half-empty page reads as debris. What does NOT dissolve is a dock whose reason is the distance
  between the content and the control that commits it — the editor's, an experiment's — because
  that distance is there at every width. And the bar stays ONE element either way: a phone dock
  plus a desktop copy of the same trigger is two overlays waiting to be opened.
- **One docked bar per scroller.** Two sticky bars in the same scroll container pin to the same
  offset, and the later one in DOM order paints over the earlier — both are opaque. A section
  rendered inside a page that already docks puts its action in flow instead.
- **A dock may carry more than one row.** The editor's 글 다듬기 docks the revision instruction above
  its two confirming actions, because the draft between them is routinely thousands of pixels tall
  and an action at the end of that flow is an action off the screen. A row that is not in use
  collapses (the revision's counter and its two save controls) rather than standing over the
  content it hides.
- **Feedback renders where the user is looking.** A success message 1,000px below the button that
  caused it has not been shown. A validation message under a keyboard has not been shown. A live
  region that is _inserted_ with its text already in it announces nothing: mount it before its
  content changes and swap the text inside.
- **A dock holds controls and refusals. What is merely TRUE goes to the page top.** A screen's own
  state — a running job, a save, a lifecycle status — is reported in ONE region at the top of the
  page: a `ProgressBar` pinned to the top edge while something runs, plus ONE `meta` line carrying
  at most one thing in a stated precedence. A state repeated in a second place is a second thing to
  keep in sync and a second thing to read; a state boxed in a `Notice` borrows a container built for
  a warning. The dock below it keeps the controls and the reason a control is refused — and a
  FAILURE with a retry, because something the user can act on is a control, not a status. A bar left
  holding neither is not rendered (§0). Owner decision 2026-09-02 (change 15); the editor is the
  reference implementation (`pages/editor/ui/EditorStatus.tsx`).
  - The bar is `sticky`, not `fixed`, so it holds the top edge through a page thousands of pixels
    tall — which means it must be in the page's normal flow, and it must add **no layout height** or
    its arrival pushes the content down mid-read. Its offset is per-header-shape: `top-0` where the
    shell header is not sticky (the phone), `sm:top-16` where it is.
  - A state that the user has to ACT on outranks one they only have to know: a failing autosave
    beats a running job's stage, because a job runs for minutes and there is no save button to fall
    back on (PRD F-2).
  - Numbers belong to the bar, not to the prose. The line names the stage; `x/y` in a sentence
    beside a bar carrying the same ratio is the same fact twice, in two grammars.
  - A state that STANDS does not get an event's presentation. A success notice nothing takes down
    is a status; report it on the line and delete the notice.

### 4.4 One scroller per screen

A phone screen has exactly **one** scroll container: the document. A nested `overflow-y-auto`
region steals every vertical swipe that lands inside it, and on a full-width element there is
nowhere left to start a page scroll but the 16px gutters.

So: no `max-h-* overflow-y-auto` on a content panel, and no fixed-`rows` textarea holding text
longer than it shows. A textarea that must grow, grows (`autoGrow`); a panel that is long, is
long, and the page scrolls it.

Three exceptions are deliberate. A **horizontal** strip (`overflow-x-auto`, a photo strip, a tab
row, or the editor's contact-sheet carousel) does not compete with the page's vertical scroll. A
carousel's cards are deliberately narrower than the strip so a **sliver of the next one** is
visible: a phone has no hover and no scrollbar, so that sliver is the only thing saying the strip
scrolls at all, and a `현재 / 전체` indicator says where in it you are. A **sheet's** own body scrolls because the
sheet is a separate surface with its own bounds. And a **field inside a form** is capped at
`max-h-field` and scrolls past it — an uncapped `autoGrow` field holding a long generated value
would put the control that commits it thousands of pixels from the caret, which §4.3 forbids, and
between the two rules reach wins. The editor's title and memo are _not_ capped: the title stays
bare, the memo uses a visible field well, and both grow with the page. All three set
`overscroll-behavior: contain` so a scroll that reaches the end does not chain to the page.

### 4.5 The desk

Past ~1024px the window stops being a constraint and starts being a room. The app was authored
phone-first and stayed that way all the way up, so on a 1440px display it rendered a 672px column
centred under a full-width bar — roughly 380px of empty gutter on either side of what still read
as a phone screen. Emptiness is not restraint; it is space the layout declined to have an opinion
about.

Two things change at `lg:`, and only these two.

- **The destinations move into a persistent left sidebar.** Five links in a horizontal strip leave
  the whole navigation huddled in the top-left corner of a wide bar, and every extra destination
  makes the strip longer rather than the list clearer. A vertical rail scans top-to-bottom, which
  is measurably faster to target than a horizontal row, holds an active state as a plane instead of
  a colour on four characters, and has room for a seventh destination that the phone bar does not.
  The top bar STAYS: the brand, the theme, the locale and the account exist exactly once in the
  tree, and folding them into the rail would mean two of each fighting over which one is visible.
  The rail hangs from the bar's bottom edge (`top-header`, `h-sidebar`) and sticks there.
- **The content column widens, by kind rather than by page.** `shared/ui/page`'s `pageStyles`
  gives every screen one of three widths — `prose` for something authored or read, bounded by the
  eye at ~75 characters and not by the window; `wide` for a directory of rows, where the extra
  room goes into the row's own metadata columns; `board` for a screen that is genuinely
  two-dimensional, which is the only kind that earns the whole desk. A page picks its kind once,
  so widening the shell later is one edit rather than fifteen `max-w-*` in fifteen files.

**Three planes, not two.** The rail and the top bar are both chrome and they meet at a corner, so
painting both `surface-raised` fuses them into one L-shaped slab and the eye reads no structure at
all. The scale is there to be walked: the rail sits on `surface-recessed`, one step BEHIND the
page; the content is `surface-base`; the bar is `surface-raised` and floats over both. A row in the
rail then walks the same three steps — `recessed` at rest, `base` under the pointer, `raised` for
the current destination, which is the bar's own plane and so the brightest thing in the column.

This is also why the rail cannot borrow `row-bg-hover` / `row-bg-active`: that pair is calibrated
for a row sitting on `surface-base` (hover lifts to `raised`, a press sinks to `recessed`), and on a
recessed rail the press plane IS the rail and the hover plane is the bar. A row on a plane other
than `base` states its own two steps.

What does NOT change at `lg:`: the number of scrollers (§4.4 still holds — the rail is not a second
one), where a committing action lives (§4.3), and the phone's shape, which is reached by deleting
every prefixed class and must still be a finished screen (§1.5).

A row that runs edge to edge inside a `pageStyles` column cancels all three gutters, not two:
`-mx-4 sm:-mx-6 lg:-mx-8`. A page-level sticky bar offsets past the header with `sm:top-header`,
never a repeated `16`.

## 5. Elevation and shape

- Three shadow steps mixed from `shadow-color`: `shadow-sm` resting, `shadow-md` a floating panel,
  `shadow-lg` a modal. Shadows encode _distance from the page_, never importance. Most surfaces
  cast none — a plane change is enough.
- Radius scale: `rounded-sm` (6px) chips and small tags · `rounded-md` (10px) controls ·
  `rounded-lg` (14px) panels and cards · `rounded-xl` (20px) sheets, dialogs, and a docked bar · `rounded-full`
  avatars and pills. A control and the panel it sits in never share a radius — the inner one is
  one step smaller. A surface attached to a screen edge is rounded on the free side only
  (`rounded-t-xl` for a bottom sheet).
- No gradients on surfaces. No glassmorphism — there is nothing behind the chrome worth seeing
  through to. The docked bar was the one candidate worth re-testing, since it stands over the
  draft the whole time; a translucent variant was built and compared beside the opaque one in both
  themes, and the owner kept the opaque bar (change 12, 2026-09-01).

## 6. Motion

- Durations: `duration-fast` (120ms) hover/press · `duration-base` (200ms) reveal/dismiss ·
  `duration-slow` (320ms) a sheet or page transition. Eases: `ease-standard` for everything,
  `ease-emphasized` only for something _arriving_.
- Animate `opacity` and `transform` only. Never `height`, `width`, or layout properties.
- **Every control has a press state, because on a phone there is no hover.** Tailwind compiles
  `hover:` to `@media (hover: hover)`, so a hover-only fill emits CSS that a touchscreen never
  matches: a variant whose entire resting affordance lives behind `hover:` is invisible on the
  device this product is for. Every interactive element therefore carries an `active:` treatment,
  and a variant that needs a resting plane on touch uses the `pointer-coarse:` variant to get one.
- **A pending action shows its state in place, at a fixed size.** A spinner covers the label; the
  button does not change width. Swapping `생성` for `생성을 시작하는 중…` triples the button's width
  under the thumb that just pressed it and, in a wrapping row, moves its neighbours. The label is
  hidden **visually only** — `opacity-0`, never `visibility: hidden`, `display: none`, or
  unmounting, all of which drop it from the accessibility tree and leave a busy button with no
  accessible name at all, since the spinner beside it is `aria-hidden`.
- Feedback is paced to the wait: under 1s, nothing; 2–9s, an indeterminate indicator; 10s or
  more, determinate progress plus a way out.
- `prefers-reduced-motion` is honoured globally (`index.css`). Don't fight it.

## 7. Components — how the primitives look

- **Button** — the current API exposes the production-backed `cta`, `secondary`, `ghost`, and
  quiet `danger` variants. `cta` uses `button-cta-*` and is the one committing action;
  `secondary` uses `button-secondary-*`; `ghost` uses `button-ghost-*`; `danger` stays quiet and
  uses `button-danger-quiet-*`, with a pressed plane on touch. The `default` and `icon` sizes are
  both 44 px tall/wide where pressed, padded to the §4.2 ratio. `compact` is the one sanctioned
  step below that floor — 36 px, still far above the 24 px WCAG 2.5.8 minimum — and is reserved for
  a **low-emphasis way out that shares a dock with the content it would otherwise cover** (A/B
  비교's 둘 다 사용하지 않기). A committing action never takes it, and inside a `sm:` flex row it
  stretches back to its siblings' height, so the shorter box is a phone-only saving. A button has
  no vertical padding at all — its height IS the `min-h` floor — so the label carries `leading-snug`:
  the only case that decides the box is a label wrapping in a narrow column, and two 20 px lines
  would leave 2 px of air inside 44 px. A `pending` prop swaps the label for a `Spinner` while
  holding the box and setting `aria-busy`. `buttonStyles` applies the same
  contract to router links and the native file-input label without replacing their semantics.
  A committing action on a phone is `w-full sm:w-auto` — the full-bleed rule of §4 applied to the
  one target that matters most. No outlined or solid-danger variant is exposed until a current
  screen needs one.
- **Field** (input, textarea) — a recessed well using `field-bg`, no border at rest. Hover and
  focus use `field-bg-hover` and `field-bg-focus`; focus also shows the global focus ring. Type
  size is the `input` role (§3.1). Validation is **a message in `field-error` under the field**
  plus `aria-invalid`; the field itself does not turn red or grow a border. Labels are always
  visible (never placeholder-as-label) and use the field label role. `Textarea` takes `autoGrow`
  so a long value extends the page rather than opening a nested scroller (§4.4). The current
  inventory is `TextField`, `Textarea`, `FieldLabel`, and `FieldMessage`, with `well` and
  editor-only `bare` appearances.
- **Every field states its keyboard.** `type`, and `inputMode` where type is not enough, plus
  `autoComplete`, `autoCapitalize`, `autoCorrect` and `enterKeyHint`. An id field that
  auto-capitalises its first character is a login failure the user cannot explain.
- **The editor title is bare; the memo is a well.** The title is a `bg-transparent` field because
  the page is its paper. The memo uses the standard recessed field background so the writing
  surface remains visibly editable. Both auto-grow with the page, and the field primitive owns
  the memo's §3.1 input size while the bare title takes the `display` role from its caller.
- **Listbox / Menu — there is no native select.** The OS draws a native select's open option list,
  so it cannot wear the app's surfaces or tokens and it visibly breaks the design system the
  moment it opens (owner decision, 2026-08-31). The primitive is gone from `shared/ui`, and
  `pnpm lint:style` fails on a literal `<select` in any non-test `.tsx` — the ban is mechanical.
  Three shapes replace it, chosen per surface:
  - **`Listbox`** is the labelled FORM FIELD: a 44px trigger wearing the `field-bg` well with the
    current option's label and a chevron, `role="combobox"` + `aria-haspopup="listbox"` (the role a
    native select exposed), and an app-drawn `surface-overlay` panel of `role="option"` rows with a
    check on the current one — `surface-overlay` and not `surface-highest`, which is the token
    `Sheet`, `Popover` and `ActionBar` all wear, so an open list used to be exactly the colour of
    the plane behind it (§2.3). The trigger is the only tab stop; arrow/Home/End move programmatic
    focus, Enter/Space select, Escape and an outside press close and return focus. Its name is
    `"<label> <current value>"` — the WAI-APG select-only combobox shape — so the closed control
    still reports its value. It is generic over the option value, so a numeric enum round-trips
    without a stringly-typed cast. Its panel is a bounded scroller, which is the §4.4 overlay
    exception, and its bound is MEASURED the way `Popover`'s is: a `dvh` token is the same number
    for a field at the top of the page and for one in a docked bar, where a panel half the screen
    tall opens entirely past the bottom edge. So the panel takes the room actually free beneath
    its trigger, flips above it when that room is too small and the room overhead is larger, and
    scrolls inside whichever it took — capped at `LISTBOX_MAX_VIEWPORT_RATIO` of the viewport
    where there is more room than a list needs, and floored at `LISTBOX_MIN_PANEL_PX`.
    Its panel is **portalled to the document body** and positioned `fixed` from the same measured
    rect, stacked above every overlay (`z-overlay-panel`, registered above `Sheet`'s `z-50`). See
    the portalled-panel rule below.
  - **`SegmentedControl`** where the choice is a bounded 2–5 and every option is worth showing at
    once — the editor steps, the admin plan ladder.
  - **`Menu`** where the trigger is an icon and the list is a short preference set — a WAI-APG menu
    button (`aria-haspopup="menu"`, `menuitemradio` rows). The header's theme and locale controls
    are `Menu`s whose trigger icon shows the current value, so the closed control still reports its
    state.

  An option's text is the choice, not the explanation — a reason or a capability goes in the
  message slot under the field, where the trigger's truncation cannot cut it off.
- **A panel anchored to a trigger is portalled, or it is clipped.** An `absolute` panel is bounded
  by every scrolling ancestor it has, and this app has several by design — `Sheet`'s body is the
  sheet's one scroller (§4.4), and a field near its bottom is exactly the field whose panel flips
  UPWARD, straight out of that scroller. So a panel that opens from an arbitrary trigger renders
  through a portal at the document body, `position: fixed` from the trigger's measured rect. The
  portal is not free, and what it costs is stated here so the next one pays it too:
  - It must TRACK the trigger — re-measure on `scroll` (captured, since `scroll` does not bubble)
    and on `resize` — and **close** once the trigger leaves the viewport on either axis, rather
    than floating on, anchored to nothing.
  - Closing that way must still hand focus back to the trigger (with `preventScroll`, so returning
    it does not undo the scroll that caused it). The focused option is about to be unmounted, and
    focus landing on `document.body` is focus outside a sheet's trap.
  - Every "is this press outside me?" test in an ANCESTOR overlay now sees the panel as outside
    itself, and must be taught otherwise — `Popover` exempts a press inside `[role="listbox"]` the
    same way it already exempts one inside `[aria-modal="true"]`.
  - `Escape` still has to dismiss exactly the innermost overlay. React dispatches portalled events
    through the component tree and attaches its listener at the portal container, so the panel's own
    handler calling `stopPropagation()` does stop the native event before it reaches the
    document-level handlers `Sheet` and `Popover` install. That is the mechanism the contract rests
    on; do not replace it with a capture-phase listener without re-checking it.
  - Its position and size are measured values on `style`, not utilities. Keep the measured numbers
    in state by VALUE — callers pass their `options` inline, so the measuring effect re-runs on
    every render of the field's parent, and a fresh object each time is a render loop.
- **TabLinks** — a tab row whose tabs are ADDRESSES: a row of links marking the current one
  `aria-current="page"`. It is `SegmentedControl`'s shape without its `onChange`, and it is a `nav` rather than
  `role="tablist"`, because announcing a navigation as tab selection would tell a screen reader the page stayed put.
  Reach for it when the panels are routes; reach for `SegmentedControl` when they are state.
  A text-only row scrolls horizontally rather than crushing its labels. When **every** item also
  carries an `icon` (and usually a `shortLabel`), the row instead reshapes with its own width
  (`@container`): below the named `@tabs` token (38rem) each tab stacks its icon over a compact caption and all tabs share
  the row evenly — nothing scrolls off a 320px screen — while the full label stays the link's one
  accessible name; from `@tabs` up it is the text row again. The wide row uses `px-1` so the five
  full English labels still fit inside VoiceLayout's 39rem maximum content width. Icon-only tabs are not an option:
  an icon alone is a guess (the voice tabs are the shipped example).
- **Editable** — read first, edit on request: a value renders as text until its pencil (always `aria-label`led with
  the field it edits) is pressed, then the caller's edit view replaces it. The primitive owns only the toggle and the
  affordance; the caller supplies both views and every action, so leaving edit mode stays the caller's decision — a
  successful save exits, a rejected one keeps the draft on screen.
- **SegmentedControl** — a bounded switch of 2–5 options. It scrolls horizontally rather than
  crushing or wrapping its labels, since a Korean option set outgrows the width long before an
  English one does. It is the primitive for a tab row; a slice never hand-rolls
  `role="tablist"`.
- **Badge / chip** — `rounded-sm`, no border, `px-2 py-0.5`. A neutral chip uses
  `badge-neutral-*`; a chip for a state the user is mid-way through (a post in review) uses
  `badge-accent-*`; a **status** chip that means good/bad/urgent takes the matching `notice-*` tone.
  Every chip carries its text label, so colour is never the only signal.
- **Notice** — the §2.6 inline-notice contract as a primitive: a tone, explanatory text, no border,
  `px-4 py-3`. It takes the `role` (`alert` for something that went wrong, `status` for progress and
  confirmation) because only the caller knows which. Colour never travels alone — the words carry
  the meaning and the tone reinforces it.
- **ActionBar** — the dock for a view's committing actions, on `surface-highest` with `rounded-xl`
  and `shadow-md`. Its padding is one step tighter on a phone (`p-3 sm:p-4`), where the bar may
  carry two rows of controls and the content behind it is what the screen is for. It stays OPAQUE:
  a translucent glass variant was built and compared side by side in both themes, and the owner
  chose the opaque one (change 12, 2026-09-01) — §5's "no glassmorphism" stands. It is the one surface that floats over content without interrupting it, which is
  why it takes the frontmost plane but a floating panel's shadow rather than a modal's. It clears
  the phone tab bar and the home indicator itself, so a caller never writes a safe-area class, and
  it FLOATS a step clear of whatever is under it — the tab bar on a phone (`bottom-dock-nav`), the
  viewport edge from `sm:` up. Resting the card on either reads as a cut-off sheet, or as one
  two-storey slab of chrome, rather than as a dock hovering over the page. One per scroller (§4.3), and it goes at
  the end of a `flex-1 flex-col` page with `mt-auto` — `sticky` can pull a bar up to the scrollport
  edge but can never push one down, so on a short page an undocked bar floats mid-screen. Mount it
  only when it has something to hold: a bar with no action and nothing to report is chrome with
  nothing to say (§0).
- **Overlays** — tooltip explains, toast reports, dialog/sheet interrupts. All portalled, all
  return focus where they found it, all `Escape` closes, all lock the body scroll while open.
  **On a phone a dialog is a bottom sheet**: full-bleed to the bottom edge, `rounded-t-xl`, safe-area
  padded, its body the one thing that scrolls, dismissible by the scrim and a visible control —
  becoming a centred `rounded-xl` dialog from `md:` up. A sheet also RISES from the edge it is
  docked to and sinks back into it (`duration-slow`, `ease-emphasized` in, `ease-standard` out),
  because arriving from below is the one thing that says it came from somewhere rather than
  appearing over the page; the centred dialog only fades and settles, since a slide from the
  bottom edge would be a lie about where it lives. The departure costs mounted time React does not
  give away — the node would be gone before a frame of it played — so the panel outlives `open` by
  exactly one animation and `animationend` unmounts it, while everything the overlay OWES the page
  (the body scroll, the returned focus, its claim on `Escape`) is surrendered the moment `open`
  goes false. What is left is a picture, and it is `inert`. Where nothing actually animates, no
  `animationstart` arrives and the close is immediate instead. Those mechanics belong to
  **`Sheet`**, which takes arbitrary content; **`Dialog`** is `Sheet` with the confirm shape (one
  title, one explanation, cancel and confirm) fixed on top. Its pair is ONE row split 3 : 7 on a
  phone, cancel left and the CTA right — confirming is what the sheet was opened to do, and 취소
  needs only its two syllables — collapsing to the right-aligned desktop row from `md:` up.
  Stacking them full-width spends two 44px rows and a gap on a decision with one obvious answer,
  on the shape with the least room for it. A destructive action is confirmed through `Dialog`,
  never through `window.confirm`, which mobile browsers let the user suppress permanently.
  **`Popover`** anchors a panel to a trigger: `align` picks the pinned edge, and BOTH of the
  panel's bounds are measured against where that trigger actually sits, because nothing in CSS
  knows. Horizontally the overflow is corrected back inside the page gutters. Vertically the panel
  is capped at the room between the trigger and the edge it grows toward and scrolls inside it —
  a `dvh` token can only guess at that room, and the part of a tall panel that runs past the edge
  is unreachable, since the page behind it is scroll-locked on a phone and the trigger is a docked
  bar that does not move on a pointer. The token stays as the pre-measurement ceiling, and a floor
  (`POPOVER_MIN_PANEL_PX`) keeps a panel usable rather than squeezing it to a sliver. `phone="sheet"`
  swaps the panel for a `Sheet` below `sm:` — a long options surface needs the whole phone screen
  and its own scroller, not a 288px card over the content. `triggerSize` takes the `Button` sizes,
  so a glyph-only trigger is `icon`; `label` is already the trigger's `aria-label` and the panel's
  heading, so an icon trigger keeps its name and the closed surface still reports what it holds. A
  `PopoverHandle` ref opens the surface from elsewhere on the page — the state stays inside the
  primitive, where the listeners that close it capture one stable `close` for their lifetime. A
  press inside a modal opened FROM a popover is not a press outside it; closing there would
  unmount the control that opened the modal.
- **Empty and loading states** are text on the page (`text-content-tertiary`) — not a card, not an
  illustration. A section that renders nothing at all is worse than an empty state: it leaves a
  gap the user reads as a bug. A skeleton mirrors the shape of the content it stands in for and is
  `bg-surface-recessed animate-pulse`.
- **Destructive actions** use the quiet danger treatment, with a pressed plane so the control is
  discoverable on touch. An undoable single-item delete is not confirmed; an unrecoverable one is,
  through the sheet.

## 8. The phone

Everything in this section is a platform behaviour, not a taste. The owner is
`frontend/index.html` and the base layer of `frontend/src/app/styles/index.css`.

### 8.1 Viewport and safe area

The one correct viewport meta:

```html
<meta
  name="viewport"
  content="width=device-width, initial-scale=1, viewport-fit=cover"
/>
```

`user-scalable=no`, `maximum-scale`, and `minimum-scale` are never added — they are a WCAG 1.4.4
failure on Android and are ignored on iOS, so they cost accessibility and buy nothing.

`viewport-fit=cover` lets content reach the physical edges, which makes `env(safe-area-inset-*)`
non-zero and therefore **mandatory** on anything anchored to an edge. Because §1.2 forbids
arbitrary values at the call site, the insets are registered once as spacing tokens in `@theme`
and consumed as ordinary utilities (`pb-safe-b`). Backgrounds and scrolling content run to the
edge; only _controls_ are inset.

**The inset ADDS to the design padding; it never replaces it.** Two padding utilities on the same
side collide and the longhand wins, so `p-4 pb-safe-b` resolves padding-bottom to the bare inset —
zero in every desktop browser and on every phone without a home indicator — and the box's last
control ends up flush against its own rounded corner. This has bitten the docked bar and the
bottom sheet, and both are fixed the same way: a token that sums the two
(`--spacing-dock-b`, `--spacing-sheet-b`), or a `mb-safe-b` MARGIN where the padding must stay
untouched (the about page's footer).

Height is `dvh` / `svh`, never `vh` — `100vh` is the tall-viewport height, so a `100vh` element is
clipped by the URL bar on every phone.

### 8.2 The scroll

`html` sets `overscroll-behavior-y: contain`. Without it, Chrome for Android fires pull-to-refresh
whenever the document is at `scrollTop: 0` and the user swipes down — which reloads the SPA and
discards anything autosave has not flushed. The editor opens at `scrollTop: 0`.

Every overlay and every inner scroll region sets `overscroll-behavior: contain` for the same
reason at a smaller scale, and an open sheet locks the body scroll (§7).

### 8.3 The keyboard

The software keyboard covers the bottom ~40% of the screen and, on modern Chrome and iOS Safari,
**does not resize the layout viewport** — so `position: fixed` and `sticky bottom-0` elements stay
pinned behind it rather than riding above it. Design for that:

- A form's committing action and its error message are **above** the fold of the keyboard, which
  in practice means directly after the last field rather than at the bottom of a centred block.
- A vertically centred form is a desktop pattern: centring pushes the submit button into the
  keyboard's band on every phone size. Anchor forms to the top.
- Removing a focused element from the DOM dismisses the keyboard, and a `focus()` call that is not
  inside a user gesture will not bring it back on iOS. Do not re-mount the field the user is
  typing into.

### 8.4 Touch affordance

- Nothing is reachable only by hovering. A control revealed on `:hover`, or whose only fill is a
  `hover:` class, does not exist on this device (§6).
- `touch-action: manipulation` on tappable controls, so a tap is not held for 300ms waiting for a
  double-tap-to-zoom that will not come.
- Dragging is never the only way to do something, and nothing commits on the down-event.
- `user-select: none` is for chrome — a tab bar, a segmented control — never for content the user
  might want to copy.

### 8.5 Not overflowing

At 320 CSS px the page scrolls vertically and only vertically. The recurring causes, all of which
have appeared in this codebase:

- A **flex or grid child with text** and no `min-w-0` — its automatic minimum size is its
  min-content width, so a long token pushes the row wider than the page. `truncate` does nothing
  without it.
- A `shrink-0` on the item that _should_ give way, which pins a long string at its max-content
  width and crushes its neighbour to zero.
- An unbreakable string with no `break-words` (§3.2).
- A fixed `w-*` on a card that is wider than the phone's content column.

`overflow-x: hidden` on `html`/`body` is not a fix — it hides the symptom and makes the
overflowing content unreachable. Find the element.

### 8.6 Weight

- Every `<img>` carries `width`, `height` and `alt`; below-the-fold images carry
  `loading="lazy" decoding="async"`. Reserve the box with `aspect-ratio` so late-arriving images
  do not shift the text the user is reading.
- Budget INP at ≤ 200 ms: a tap handler does the visual update and defers the rest.

## 9. Accessibility baseline

Non-negotiable, enforced at review:

- Every control has a visible label or an `aria-label`; icon-only buttons always `aria-label`.
- Focus is always visible (`:focus-visible` ring, never `outline-none` without a replacement).
  One global indicator — a primitive does not add a second ring of its own.
- **A scrolling container reserves room for that ring.** The global indicator is `outline: 2px` at
  `outline-offset: 2px`, so a control needs 4px of clear space on every side to show one — and CSS
  resolves a scroll container's cross axis away from `visible`, which cuts the left and right edges
  of a `w-full` field's ring the moment it takes focus. A scroller whose contents can be focused
  therefore carries `p-focus-gutter -m-focus-gutter`: the padding reserves the space, the equal
  negative margin gives it back to the layout, and nothing inside moves. A scroller that already
  has padding ≥ the gutter on every side needs nothing.
- Live regions (`role="status"` / `role="alert"`) for save state, upload progress, and errors —
  and the region renders where the user is looking (§4.3), not merely somewhere in the DOM.
- The current navigation destination is marked `aria-current="page"` and shown, not just implied
  by the URL — on a phone there is often no visible URL at all.
- Contrast ≥ AA in both themes (§2.5).
- Everything reachable by keyboard; overlays trap and return focus; `Escape` closes.
- Orientation is never locked, and text survives a 200% zoom.

## 10. Review checklist

Run this over any FE diff before calling a job done (it is the design half of the
`/implement-job` Step 4 review):

**Design system**

- [ ] No control is hand-rolled in a slice — it comes from `shared/ui` (or was added there first).
- [ ] No unused primitive or speculative variant was added; `shared/ui` grows only for current UI.
- [ ] New code uses functional colour roles and adds no use of the retired vocabulary.
- [ ] `pnpm lint:style` and `pnpm lint:style:probe` pass; no arbitrary sizes off the scale.
- [ ] Every `border-*` is one of the §1.3 exceptions.
- [ ] Every card passes the §1.4 test (belong together _and_ separate from neighbours).
- [ ] Exactly one CTA per view, using the `button-cta-*` contract.
- [ ] Slice text renders through `Typography` / `typographyStyles` (§3); the `pnpm lint:style`
      type gate passes, and every `// style-escape:` pragma carries a reason a reviewer accepts.
- [ ] State is reported ONCE, in the page-top status region — not also in the dock, and not in a
      `Notice` when nothing takes it down (§4.3).
- [ ] Any scroller whose contents take focus reserves the focus-ring gutter (§9).
- [ ] Any panel anchored to a trigger inside a scroller is portalled, and pays the portal's four
      costs: tracking, focus return, ancestor outside-press tests, and `Escape` (§7).

**The phone** — checked at 360 px, not just at desktop width

- [ ] Deleting every `sm:`/`md:` class still leaves a finished screen (§1.5).
- [ ] No horizontal page scroll at 320 px; every flex/grid text child has `min-w-0`.
- [ ] Every target ≥ 44 × 44 including its **width**, with ≥ 8 px between neighbours (§4.1).
- [ ] Control padding follows the §4.2 ratio — no 1 : 1 boxes from an unpaid `min-h-11`.
- [ ] Every typeable control computes to ≥ 16 px on phone, and states its keyboard (§3.1, §7).
- [ ] The committing action is in the lower band and on the same screen as what it commits (§4.3).
- [ ] One scroller: no nested `overflow-y-auto`, no fixed-`rows` textarea holding more than it shows (§4.4).
- [ ] Every interactive element has an `active:` state; nothing is hover-only (§6).
- [ ] Pending states hold the button's size AND its accessible name; feedback renders where the
      user is looking, in a live region that was already mounted.
- [ ] At most one docked `ActionBar` per scroller, and its page fills the shell so it docks rather
      than floating mid-screen.
- [ ] Edge-anchored chrome is safe-area padded (§8.1).

**Both themes**

- [ ] Looks right in both `night` and `day` (toggle `data-theme` on `<html>` in devtools).
