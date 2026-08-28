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

## 1. The four hard rules

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

### 1.2 Only Tailwind-registered tokens

A class must resolve to a token this repo registered in `@theme` — the colour foundations and
functional roles (§2), the radius/duration/ease/shadow/font scale, and Tailwind's stock spacing
and type scales. Use the narrowest role that describes the element: a CTA uses
`button-cta-*`, a field uses `field-*`, and an inline error uses `notice-danger-*`. Surface and
content foundations are for page composition and typography, not a shortcut around a component
role.

Forbidden, with no exceptions in `frontend/src`:

- Tailwind's **stock colour palette** (`bg-gray-100`, `text-red-400`, `border-neutral-800`). It is
  removed in `index.css` (`--color-*: initial`), so such a class emits **no CSS at all** — the UI
  silently loses its styling. `pnpm lint:style` fails on any occurrence.
- **Arbitrary colour values** (`bg-[#1a1a2e]`, `text-[oklch(…)]`) and raw colour literals in
  TS/TSX/CSS. Same gate.
- **Arbitrary sizes** off the scale (`p-[13px]`, `rounded-[7px]`, `text-[15px]`). Density is
  chosen by picking a step. If two steps are both wrong, the scale is wrong — change the scale in
  `index.css`, not the call site. (The one tolerated arbitrary is a tiny label size such as
  `text-[10px]` for a metadata chip; do not spread it.)

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
- a table rule.

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
| `surface-highest`                                           | The frontmost floating plane: menus, sheets, dialogs, and popovers         |
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
| Navigation/list | `link-*`, `row-bg-*`, `divider`                                                                               |
| Labels/feedback | `badge-neutral-*`, `notice-{danger,success,warning,info}-*`                                                   |
| System/media    | `focus-ring`, `selection-*`, `media-scrim-*`, `shadow-color`                                                  |

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
pairs with a contrast tool when a foundation moves.

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
| body    | `text-sm leading-relaxed`                                               | prose the user reads and writes      |
| label   | `text-sm text-content-secondary`                                        | a generic label or secondary line    |
| meta    | `text-xs text-content-tertiary`                                         | timestamps, counts, status lines     |
| eyebrow | `text-[10px] font-medium uppercase tracking-wide text-content-tertiary` | a category chip above a group        |

- Prose columns are capped: `max-w-measure` (`--container-measure`, 40rem).
- Field labels and placeholders use `text-field-label` and `text-field-placeholder`; a field
  primitive, not its caller, owns those choices.
- An eyebrow is not a heading; a heading is a real `<h*>`.

## 4. Space, hierarchy, density

- One 4px scale (Tailwind's). **Inside a control** 2–3 · **inside a group** 3–4 · **between
  groups** 6–8 · **between page sections** 10–16.
- Hierarchy is built in this order, each used only when the one before is not enough:
  **position → size → weight → colour → surface → border.**
- Touch targets are ≥ 44px tall on anything the thumb presses. A row in a list is one target, not
  a row with a small button inside it.
- A group of actions reads left to right by rising emphasis: the way out first, the committing
  action last. The committing action is the only filled one.
- Full-bleed on mobile: the page has `px-4` (phone) → `px-6` (≥ sm) gutters and no visible
  container; content sits directly on `surface-base`.

## 5. Elevation and shape

- Three shadow steps mixed from `shadow-color`: `shadow-sm` resting, `shadow-md` a floating panel,
  `shadow-lg` a modal. Shadows encode _distance from the page_, never importance. Most surfaces
  cast none — a plane change is enough.
- Radius scale: `rounded-sm` (6px) chips and small tags · `rounded-md` (10px) controls ·
  `rounded-lg` (14px) panels and cards · `rounded-xl` (20px) sheets and dialogs · `rounded-full`
  avatars and pills. A control and the panel it sits in never share a radius — the inner one is
  one step smaller.
- No gradients on surfaces. No glassmorphism — there is nothing behind the chrome worth seeing
  through to.

## 6. Motion

- Durations: `duration-fast` (120ms) hover/press · `duration-base` (200ms) reveal/dismiss ·
  `duration-slow` (320ms) a sheet or page transition. Eases: `ease-standard` for everything,
  `ease-emphasized` only for something _arriving_.
- Animate `opacity` and `transform` only. Never `height`, `width`, or layout properties.
- Every hover has a press state (`active:scale-[0.98]` or a darker step); every async action shows
  its pending state in place — a spinner _replacing_ the label, the button staying the same size.
- `prefers-reduced-motion` is honoured globally (`index.css`). Don't fight it.

## 7. Components — how the primitives look

- **Button** — the current API exposes only the production-backed `cta`, `secondary`, `ghost`, and
  quiet `danger` variants. `cta` uses `button-cta-*` and is the one committing action;
  `secondary` uses `button-secondary-*`; `ghost` uses `button-ghost-*`; `danger` stays quiet and
  uses `button-danger-quiet-*`. The `default` and `icon` sizes are both 44 px tall/wide where
  pressed. `buttonStyles` applies the same contract to router links and the native file-input
  label without replacing their semantics. No outlined or solid-danger variant or functional
  token is exposed until a current screen needs one (solid danger remains reserved conceptually
  for confirmation contexts).
- **Field** (input, textarea) — a recessed well using `field-bg`, no border at rest. Hover and
  focus use `field-bg-hover` and `field-bg-focus`; focus also shows the global focus ring.
  Validation is **a message in `field-error` under the field** plus `aria-invalid`; the field
  itself does not turn red or grow a border. Labels are always visible (never
  placeholder-as-label) and use the field label role. The current inventory is `TextField`,
  `Textarea`, `FieldLabel`, and `FieldMessage`, with `well` and editor-only `bare` appearances.
- **The editor is bare.** Title and body are `bg-transparent` fields with no well at all — the
  page _is_ the paper. This is the one place a field has no surface.
- **Menu / Select** — a bounded choice in a _form_ is a native `<select>` wearing the field well
  (platform keyboard and a11y for free). In a _toolbar_ it is a `Menu` behind a `ghost` button
  that shows the held value.
- **Badge / chip** — `bg-badge-neutral-bg text-badge-neutral-fg rounded-sm`, no border. Status
  chips use the matching notice background/foreground pair and include a text label.
- **Overlays** — tooltip explains, toast reports, dialog/sheet interrupts. All portalled, all
  return focus where they found it, all `rounded-xl` on `surface-highest` with `shadow-lg` over a
  `bg-media-scrim-bg/60` scrim. On a phone a dialog is a bottom sheet.
- **Empty and loading states** are text on the page (`text-content-tertiary`) — not a card, not an
  illustration. A skeleton mirrors the shape of the content it stands in for and is
  `bg-surface-recessed animate-pulse`.
- **Destructive actions** use the quiet danger treatment until hovered. A solid-danger treatment
  and its functional tokens are added only when a real confirmation dialog needs them.

## 8. Accessibility baseline

Non-negotiable, enforced at review:

- Every control has a visible label or an `aria-label`; icon-only buttons always `aria-label`.
- Focus is always visible (`:focus-visible` ring, never `outline-none` without a replacement).
- Live regions (`role="status"` / `role="alert"`) for save state, upload progress, and errors.
- Contrast ≥ AA in both themes (§2.5).
- Everything reachable by keyboard; overlays trap and return focus; `Escape` closes.

## 9. Review checklist

Run this over any FE diff before calling a job done (it is the design half of the
`/implement-job` Step 4 review):

- [ ] No control is hand-rolled in a slice — it comes from `shared/ui` (or was added there first).
- [ ] No unused primitive or speculative variant was added; `shared/ui` grows only for current UI.
- [ ] New code uses functional colour roles and adds no use of the retired vocabulary.
- [ ] `pnpm lint:style` and `pnpm lint:style:probe` pass; no arbitrary sizes off the scale.
- [ ] Every `border-*` is one of the §1.3 exceptions.
- [ ] Every card passes the §1.4 test (belong together _and_ separate from neighbours).
- [ ] Exactly one CTA per view, using the `button-cta-*` contract.
- [ ] Type roles from §3 only; no ad-hoc size/weight/colour combinations.
- [ ] Touch targets ≥ 44px; keyboard focus visible; live regions on async state.
- [ ] Looks right in both `night` and `day` (toggle `data-theme` on `<html>` in devtools).
