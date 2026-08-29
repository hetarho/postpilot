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

| Constraint            | Value                                                                          |
| --------------------- | ------------------------------------------------------------------------------ |
| Design width          | **360 CSS px** — the realistic small-Android floor. 390 and 430 are also checked |
| Conformance floor     | **320 CSS px** with no horizontal page scroll (WCAG 1.4.10 Reflow, AA)          |
| Grip                  | One hand. On a 430×932 phone the top ~40% of the glass is out of thumb reach    |
| Software keyboard     | Covers roughly the **bottom 40%** of the screen whenever a field has focus      |
| Script                | Korean. A Hangul syllable advances a full em — Korean text is ~1.8× wider per character than the same count of Latin ones, and it breaks between syllables unless told not to |
| Network               | Cellular. Every RPC can be slow, and every RPC can fail                         |

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
  the scale in `index.css`, not the call site. (The one tolerated arbitrary is a tiny label size
  such as `text-[10px]` for a metadata chip; do not spread it.) Note that `pnpm lint:style` gates
  _colour_ escapes only — an arbitrary **size** passes the scanner and is caught at review, so it
  is on the author, not the tool.

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
- a spinner's ring, where the border *is* the shape being drawn.

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

Two breakpoints carry meaning, and no others are introduced without a reason written down:

| Prefix | Width  | What changes                                                                  |
| ------ | ------ | ----------------------------------------------------------------------------- |
| _none_ | ≥ 0    | The phone. Single column, one scroller, thumb-reachable actions               |
| `sm:`  | 640 px | **Density** — tighter type, restored horizontal padding, a second grid column |
| `md:`  | 768 px | **Shape** — the pointer is assumed fine: side-by-side panes, dialog-not-sheet |

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
clear; jumping several levels makes ordinary chrome look detached from the page.

| Foundation                                                  | Meaning                                                                    |
| ----------------------------------------------------------- | -------------------------------------------------------------------------- |
| `surface-lowest`                                            | The furthest-back plane; deep wells and deliberately sunken regions        |
| `surface-recessed`                                          | Inputs and regions pressed one step into the current plane                 |
| `surface-base`                                              | The normal page canvas                                                     |
| `surface-raised`                                            | Rows on interaction, compact controls, cards, and resting floating content |
| `surface-highest`                                           | The frontmost floating plane: menus, sheets, dialogs, popovers, and a docked action bar |
| `content-primary` · `secondary` · `tertiary` · `disabled`   | Main copy → supporting copy → metadata → non-interactive copy              |
| `stroke-subtle` · `strong`                                  | Rare structural hairlines; never a substitute for a surface step           |
| `intent-accent` · `danger` · `success` · `warning` · `info` | Meaning before it is assigned to a component                               |
| `interaction-focus`                                         | The one keyboard-focus colour                                              |
| `physical-scrim` · `physical-on-scrim` · `physical-shadow`  | Physical light/occlusion, not product meaning                              |

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
| Labels/feedback | `badge-neutral-*`, `notice-{danger,success,warning,info}-*`                                                   |
| System/media    | `focus-ring`, `selection-*`, `media-scrim-*`, `shadow-color`                                                  |
| Brand           | `brand-wordmark`, `brand-mark`                                                                                |

State names are explicit (`bg`, `bg-hover`, `bg-active`, `fg`) so a primitive never invents a
hover colour by changing opacity. Function names describe purpose: the committing action is
`button-cta`, not "purple" or a provider/model-specific name.

### 2.5 Themes

A theme is one `[data-theme='<key>']` block re-mapping the semantic foundations. `night` (dark,
the default on `:root`) and `day` (light) ship; there is **no switcher yet**. Adding one is: set
`data-theme` on `<html>` from the app layer. Adding a _third_ theme is one more block. Nothing in
a component ever changes, because nothing in a component names a palette step — that is the
point of the layers.

Both themes must keep primary/secondary/tertiary content on every surface, and every functional
foreground on its background, at WCAG AA (4.5:1 for body, 3:1 for large text). Check all mapped
pairs with a contrast tool when a foundation moves. For Korean, "large text" is the CJK size
equivalent of 18 pt, not the Latin pixel value — do not claim the 3:1 exemption on 19px Hangul.

The browser chrome is part of the theme. `<meta name="theme-color">` in `index.html` carries the
header's plane, because Chrome for Android tints its address bar from that tag alone;
`color-scheme` does not reach it, and a light toolbar band above a near-black app is the most
visible surface in the product.

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

| Role    | Classes                                                                 | Used for                             |
| ------- | ----------------------------------------------------------------------- | ------------------------------------ |
| display | `text-2xl font-semibold tracking-tight`                                 | the page's one title, the post title |
| title   | `text-lg font-semibold tracking-tight`                                  | a section heading, a dialog title    |
| body    | `text-sm leading-relaxed`                                               | prose the user reads                 |
| input   | `text-base sm:text-sm`                                                  | **anything the user types into** — see §3.1 |
| label   | `text-sm text-content-secondary`                                        | a generic label or secondary line    |
| meta    | `text-xs text-content-tertiary`                                         | timestamps, counts, status lines     |
| eyebrow | `text-[10px] font-medium uppercase tracking-wide text-content-tertiary` | a category chip above a group        |

- Prose columns are capped: `max-w-measure` (`--container-measure`, 40rem).
- Field labels and placeholders use `text-field-label` and `text-field-placeholder`; a field
  primitive, not its caller, owns those choices.
- An eyebrow is not a heading; a heading is a real `<h*>`.
- `text-xs` (12px) is the floor, and it is for metadata only. Explanatory copy the user is meant
  to act on is never 12px — if a sentence matters enough to render, it is `text-sm`.

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

| Rule                                                                    | Source                             |
| ----------------------------------------------------------------------- | ---------------------------------- |
| **44 × 44 CSS px** is the house minimum for anything the thumb presses  | Apple HIG; WCAG 2.5.5 (AAA)        |
| **24 × 24 CSS px** is the absolute conformance floor, never gone below  | WCAG 2.5.8 Target Size (AA)        |
| **≥ 8 px of clear space** between two adjacent targets                  | Material / Lighthouse tap-targets  |

Both dimensions count. `min-h-11` on a bare text link sets the height and leaves the **width** to
the label — and a two-syllable Korean label like `말투` is ~28px wide, so the target is 28 × 44
and fails. A control sized by its text always gets horizontal padding too.

When the visible box must stay small (a checkbox, a small glyph), the **primitive** grows the hit
area with padding or a pseudo-element — the caller is never asked to remember. A row in a list is
one target, not a row with a small button inside it.

### 4.2 Padding is a ratio, not a leftover

**`min-h-11` is a touch-target floor, not a padding value.** This is the single most common
density bug: a control sets `px-3 py-2 min-h-11`, the 44px floor overrides the computed height,
and the *effective* vertical padding becomes 12px against 12px of horizontal — a 1 : 1 box.
Because text is far wider than it is tall, a 1 : 1 control always reads squat and bloated, and a
screen full of them reads clumsy no matter how good the type and colour are.

The rule: **a control's horizontal padding is roughly twice its effective vertical padding.**
Author the horizontal padding for the height the control will actually have, not for the padding
you wrote.

| Element                        | Padding                | Effective ratio        |
| ------------------------------ | ---------------------- | ---------------------- |
| Button, field, select (44px)   | `px-4` (`px-5` for a CTA) | ≈ 2 : 1 against the 12px the floor produces |
| Icon-only button               | `size-11`, no padding  | square by definition   |
| List row (44px, full-bleed)    | `px-4 py-3`            | equal to the page gutter it sits in |
| Badge / chip                   | `px-2 py-0.5`          | ≈ 3 : 1 — small boxes need proportionally more |
| Inline notice                  | `px-4 py-3`            | ≈ 1.3 : 1 — a text block, not a control |
| Card / panel                   | `p-4` (`p-5` for a sheet) | uniform               |

Two corollaries. A **row** is never inset less than the gutter rhythm it sits in — `px-2` inside a
`px-4` page reads as a mistake, not as a nested step. And a control and the panel it sits in never
share a padding step for the same reason they never share a radius (§5).

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
- **One docked bar per scroller.** Two sticky bars in the same scroll container pin to the same
  offset, and the later one in DOM order paints over the earlier — both are opaque. A section
  rendered inside a page that already docks puts its action in flow instead.
- **Feedback renders where the user is looking.** A success message 1,000px below the button that
  caused it has not been shown. A validation message under a keyboard has not been shown. A live
  region that is *inserted* with its text already in it announces nothing: mount it before its
  content changes and swap the text inside.

### 4.4 One scroller per screen

A phone screen has exactly **one** scroll container: the document. A nested `overflow-y-auto`
region steals every vertical swipe that lands inside it, and on a full-width element there is
nowhere left to start a page scroll but the 16px gutters.

So: no `max-h-* overflow-y-auto` on a content panel, and no fixed-`rows` textarea holding text
longer than it shows. A textarea that must grow, grows (`autoGrow`); a panel that is long, is
long, and the page scrolls it.

Three exceptions are deliberate. A **horizontal** strip (`overflow-x-auto`, a photo strip or a tab
row) does not compete with the page's vertical scroll. A **sheet's** own body scrolls because the
sheet is a separate surface with its own bounds. And a **field inside a form** is capped at
`max-h-field` and scrolls past it — an uncapped `autoGrow` field holding a long generated value
would put the control that commits it thousands of pixels from the caret, which §4.3 forbids, and
between the two rules reach wins. The editor's bare fields are *not* capped: there the page is the
paper. All three set `overscroll-behavior: contain` so a scroll that reaches the end does not chain
to the page.

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
  through to.

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
  both 44 px tall/wide where pressed, padded to the §4.2 ratio. A `pending` prop swaps the label
  for a `Spinner` while holding the box and setting `aria-busy`. `buttonStyles` applies the same
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
- **The editor is bare.** Title and body are `bg-transparent` fields with no well at all — the
  page _is_ the paper. This is the one place a field has no surface. It is not an exception to
  §3.1: the caller sets `text-base` itself.
- **Menu / Select** — a bounded choice in a _form_ is a native `<select>` wearing the field well
  (platform keyboard and a11y for free), with a chevron so it does not read as a text input on a
  device with no hover. In a _toolbar_ it is a `Menu` behind a `ghost` button that shows the held
  value. An option's text is the choice, not the explanation — a reason or a capability goes in
  the message slot under the field, where it is not truncated by the native control.
- **SegmentedControl** — a bounded switch of 2–5 options. It scrolls horizontally rather than
  crushing or wrapping its labels, since a Korean option set outgrows the width long before an
  English one does. It is the primitive for a tab row; a slice never hand-rolls
  `role="tablist"`.
- **Badge / chip** — `rounded-sm`, no border, `px-2 py-0.5`. A neutral chip uses
  `badge-neutral-*`; a **status** chip takes the matching `notice-*` tone and always carries its
  text label, so colour is never the only signal.
- **Notice** — the §2.6 inline-notice contract as a primitive: a tone, explanatory text, no border,
  `px-4 py-3`. It takes the `role` (`alert` for something that went wrong, `status` for progress and
  confirmation) because only the caller knows which. Colour never travels alone — the words carry
  the meaning and the tone reinforces it.
- **ActionBar** — the dock for a view's committing actions, on `surface-highest` with `rounded-xl`
  and `shadow-md`. It is the one surface that floats over content without interrupting it, which is
  why it takes the frontmost plane but a floating panel's shadow rather than a modal's. It clears
  the phone tab bar and the home indicator itself, so a caller never writes a safe-area class. One
  per scroller (§4.3), and it goes at the end of a `flex-1 flex-col` page with `mt-auto` — `sticky`
  can pull a bar up to the scrollport edge but can never push one down, so on a short page an
  undocked bar floats mid-screen.
- **Overlays** — tooltip explains, toast reports, dialog/sheet interrupts. All portalled, all
  return focus where they found it, all `Escape` closes, all lock the body scroll while open.
  **On a phone a dialog is a bottom sheet**: full-bleed to the bottom edge, `rounded-t-xl`, safe-area
  padded, its body the one thing that scrolls, dismissible by the scrim and a visible control —
  becoming a centred `rounded-xl` dialog from `md:` up. A destructive action is confirmed through
  this primitive, never through `window.confirm`, which mobile browsers let the user suppress
  permanently.
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
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
```

`user-scalable=no`, `maximum-scale`, and `minimum-scale` are never added — they are a WCAG 1.4.4
failure on Android and are ignored on iOS, so they cost accessibility and buy nothing.

`viewport-fit=cover` lets content reach the physical edges, which makes `env(safe-area-inset-*)`
non-zero and therefore **mandatory** on anything anchored to an edge. Because §1.2 forbids
arbitrary values at the call site, the insets are registered once as spacing tokens in `@theme`
and consumed as ordinary utilities (`pb-safe-b`). Backgrounds and scrolling content run to the
edge; only _controls_ are inset.

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
- [ ] Type roles from §3 only; no ad-hoc size/weight/colour combinations.

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
