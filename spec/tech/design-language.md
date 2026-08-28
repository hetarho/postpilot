# Tech — Design language

What the postpilot interface looks like, and the rules every frontend change is held to. This is
the **binding style guide** for `frontend/src`: `/implement-job` reads it before touching any UI
surface, and `pnpm lint:style` enforces the mechanical half of it. The architectural frame is
[ARCHITECTURE.md §3](../ARCHITECTURE.md) (Feature-Sliced Design); this doc owns what the slices
*look like*.

Owner in code: `frontend/src/app/styles/index.css` (the tokens) and `frontend/src/shared/ui/*`
(the primitives, built incrementally — see §1).

## 0. The premise

Postpilot is a tool you use on a phone, often one-handed, to get a post out of your head and onto
a server before the moment passes (PRD F-2). Everything visual serves that:

- **Content is the interface.** A post's title, its text, its photos are the largest, brightest,
  most central things on the screen. Chrome is small, quiet, and at the edges.
- **Planes, not lines.** Structure is shown by *where a surface changes colour*, not by drawing
  boxes around things. A border is the last tool, not the first (§4).
- **Colour means something or it is absent.** A hue is a role — the primary action, a danger, a
  status. The rest of the interface is a near-monotone mauve so that the one violet thing on the
  screen is unmistakably the thing to press.
- **Nothing announces itself.** Motion confirms; it does not perform. Emphasis is the smallest
  amount that works.

## 1. The four hard rules

These are not preferences. A PR that breaks one is not done.

### 1.1 Only design-system UI

Every generic control — button, text field, textarea, select/dropdown, popover, dialog, toast,
badge, switch, checkbox, skeleton, tooltip — is a primitive in `frontend/src/shared/ui/<name>/`
and is used from there. **Slices never hand-roll a control** with a bare `<button className=…>`.

When a slice needs a domain-agnostic control that does not exist yet, **add it to `shared/ui`
first, then use it.** The primitive is written once, to this document, and every later slice
inherits it. A control that exists in only one slice is a design-system bug, not a shortcut.

What counts as a primitive: anything with no product noun in its name. `Button`, `TextField`,
`Menu` are primitives. `PhotoStrip`, `SaveStatus`, `PostRow` are not — they are slice UI composed
*from* primitives.

### 1.2 Only Tailwind-registered tokens

A class must resolve to a token this repo registered in `@theme` — the semantic colour roles (§2),
the radius/duration/ease/shadow/font scale, and Tailwind's stock spacing and type scales.

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

### 1.3 Borders are the last resort

Depth and grouping come from **surface steps** (`bg` → `surface` → `surface-raised`) and from
spacing. A `border-*` class is allowed only where a plane change *cannot* do the job:

- the **outlined** button variant (a rim *is* its identity),
- a hairline `divide-border` between list rows that have no background of their own,
- a table rule,
- a `border-strong` state on a control that must show validation without colour fill.

Not allowed: a border around a card, a panel, an input at rest, a header, a section. If you reach
for `border` to "separate" two things, step one of them to a different surface or add space.

### 1.4 Cards are rare

A card is a raised surface with its own padding and radius. It says "these things belong together
*and* are separate from their neighbours". Use one **only** when both halves are true — a set of
controls that act on one object, a preview that must read as a single unit, an item in a grid of
peers.

Not a card: a page section (use a heading and spacing), a form (use spacing), a list (use rows and
a `divide-border` hairline), a single paragraph, the whole page content. A card whose only content
is one line of text is always wrong. Never nest a card in a card.

## 2. Colour

### 2.1 Two tiers, one file

Colour is authored once, in `frontend/src/app/styles/index.css`:

1. **Palette** — `--palette-<hue>-<step>`, OKLCH ramps (`mauve`, `violet`, `red`, `green`,
   `gold`, `blue`, plus `white`/`black`), eleven steps on one shared lightness scale. These are the
   only raw colour literals in the frontend. They are **not** Tailwind theme variables, so no
   utility can reach them — by design.
2. **Roles** — `--bg`, `--text`, `--primary`, … Each theme block maps every role to a palette
   *step*, never to a literal. `@theme inline` exposes each role as `--color-<role>`, which is what
   `bg-surface`, `text-text-muted`, `border-border` resolve to.

Because roles reference shared steps, two roles that should match cannot drift, and a theme is a
data change (§2.4).

### 2.2 The neutral is mauve

The greys carry the primary's hue (298°) at a constant, whisper-level chroma (~0.01, about 5% of
the violet ramp's peak). Side by side with a true grey they read faintly cool and violet; alone
they read as grey. This is what makes the interface feel like *one* material rather than a grey
box with a purple button on it. Never use a true-grey or a different-hue neutral.

### 2.3 The roles

| Role                                                                   | Carries                                                                          |
| ---------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `bg` · `surface` · `surface-raised` · `surface-hover`                  | the ground planes: page → panel → raised element; and the hover step of a row    |
| `text` · `text-muted` · `text-subtle` · `text-faint`                   | primary copy → secondary copy → metadata → placeholders/disabled                 |
| `border` · `border-strong`                                             | the hairline (rare, §1.3); its emphasised state (focus/validation on a control) |
| `primary` · `primary-hover` · `primary-foreground` · `primary-surface` | the one accent: the committing action, its hover, ink on top of it, a quiet tint |
| `danger` · `success` · `warning` · `info` (+ `-foreground`, `-surface`) | status: a solid, ink on the solid, and a quiet surface for inline notices        |
| `focus-ring`                                                           | the keyboard focus indicator — one colour for every control, drawn in `:focus-visible` |
| `overlay`                                                              | the scrim behind a modal or over a thumbnail (use with an opacity modifier)      |
| `depth`                                                                | what every shadow is mixed from                                                  |

### 2.4 Themes

A theme is one `[data-theme='<key>']` block re-mapping the roles. `night` (dark, the default on
`:root`) and `day` (light) ship; there is **no switcher yet**. Adding one is: set `data-theme` on
`<html>` from the app layer. Adding a *third* theme is one more block. Nothing in a component ever
changes, because nothing in a component names a step — that is the whole point of the tiers.

Both themes must keep every `text*`-on-`bg`/`surface` pair and every `*-foreground`-on-solid pair at
WCAG AA (4.5:1 for body, 3:1 for large text). Check with a contrast tool when a step moves.

### 2.5 Applying colour

- **Surfaces step up, never out.** A panel is `surface` on `bg`; a raised element is
  `surface-raised`. Depth is the step plus (rarely) a shadow — not a border.
- **One primary per view.** The committing action is `bg-primary text-primary-foreground`. Every
  other button is `ghost` (bare label, `hover:bg-surface-hover`) or, rarely, `outlined`. Two filled
  primaries on one screen means one of them is lying about its importance.
- **Accent is for action and identity, not area.** A large violet field reads as an alarm. The
  accent lives on a control, a selected state, a focus ring, a single word.
- **Status colour never travels alone.** A red text always says what is wrong; a coloured dot is
  always beside a label. Colour-blind users lose nothing.
- **Inline notices sit on `*-surface`, in `*` ink.** `bg-danger-surface text-danger`, no border.

## 3. Typography

One family: the system stack (`--font-sans`, with Apple SD Gothic Neo / Pretendard / Noto Sans KR
for Korean). No second family, no web-font download — the app opens on a phone and must paint at
once. Weight carries emphasis (`font-medium`, `font-semibold`); colour carries importance
(`text` → `text-muted` → `text-subtle`); size carries hierarchy. Never use colour *and* weight
*and* size to say the same thing.

| Role    | Classes                                                         | Used for                              |
| ------- | --------------------------------------------------------------- | ------------------------------------- |
| display | `text-2xl font-semibold tracking-tight`                         | the page's one title, the post title  |
| title   | `text-lg font-semibold tracking-tight`                          | a section heading, a dialog title     |
| body    | `text-sm leading-relaxed`                                       | prose the user reads and writes       |
| label   | `text-sm text-text-muted`                                       | a field label, a secondary line       |
| meta    | `text-xs text-text-subtle`                                      | timestamps, counts, status lines      |
| eyebrow | `text-[10px] font-medium uppercase tracking-wide text-text-subtle` | a category chip above a group      |

- Prose columns are capped: `max-w-measure` (`--container-measure`, 40rem).
- Placeholders are `text-text-faint`, one step below metadata — present, not competing.
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
  container; content sits directly on `bg`.

## 5. Elevation and shape

- Three shadow steps mixed from `depth`: `shadow-sm` resting, `shadow-md` a floating panel,
  `shadow-lg` a modal. Shadows encode *distance from the page*, never importance. Most surfaces
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
  `ease-emphasized` only for something *arriving*.
- Animate `opacity` and `transform` only. Never `height`, `width`, or layout properties.
- Every hover has a press state (`active:scale-[0.98]` or a darker step); every async action shows
  its pending state in place — a spinner *replacing* the label, the button staying the same size.
- `prefers-reduced-motion` is honoured globally (`index.css`). Don't fight it.

## 7. Components — how the primitives look

- **Button** — two axes: `variant` (`solid` · `outlined` · `ghost`) × `tone` (`primary` ·
  `neutral` · `danger`). `solid primary` is the one committing action. `ghost neutral` is the
  default for everything else. `outlined` is for a secondary action that must still read as a
  button next to a `solid` (the *only* legitimate border on a button). Sizes `sm` / `md`; `md` is
  44px tall.
- **Field** (input, textarea) — a recessed well: `bg-surface` on `bg`, no border at rest, `focus`
  steps to `surface-raised` and shows the global focus ring. Validation is **a message in
  `text-danger` under the field** plus `aria-invalid`; the field itself does not turn red or grow
  a border. Labels are always visible (never placeholder-as-label).
- **The editor is bare.** Title and body are `bg-transparent` fields with no well at all — the
  page *is* the paper. This is the one place a field has no surface.
- **Menu / Select** — a bounded choice in a *form* is a native `<select>` wearing the field well
  (platform keyboard and a11y for free). In a *toolbar* it is a `Menu` behind a `ghost` button
  that shows the held value.
- **Badge / chip** — `bg-surface-raised text-text-muted rounded-sm`, no border. Status chips use
  `*-surface` + `*` ink.
- **Overlays** — tooltip explains, toast reports, dialog/sheet interrupts. All portalled, all
  return focus where they found it, all `rounded-xl` on `surface-raised` with `shadow-lg` over a
  `bg-overlay/60` scrim. On a phone a dialog is a bottom sheet.
- **Empty and loading states** are text on the page (`text-text-subtle`) — not a card, not an
  illustration. A skeleton mirrors the shape of the content it stands in for and is
  `bg-surface animate-pulse`.
- **Destructive actions** are `ghost danger` until hovered, `solid danger` only inside a confirm
  dialog.

## 8. Accessibility baseline

Non-negotiable, enforced at review:

- Every control has a visible label or an `aria-label`; icon-only buttons always `aria-label`.
- Focus is always visible (`:focus-visible` ring, never `outline-none` without a replacement).
- Live regions (`role="status"` / `role="alert"`) for save state, upload progress, and errors.
- Contrast ≥ AA in both themes (§2.4).
- Everything reachable by keyboard; overlays trap and return focus; `Escape` closes.

## 9. Review checklist

Run this over any FE diff before calling a job done (it is the design half of the
`/implement-job` Step 4 review):

- [ ] No control is hand-rolled in a slice — it comes from `shared/ui` (or was added there first).
- [ ] `pnpm lint:style` passes; no arbitrary sizes off the scale.
- [ ] Every `border-*` is one of the §1.3 exceptions.
- [ ] Every card passes the §1.4 test (belong together *and* separate from neighbours).
- [ ] Exactly one `solid primary` per view.
- [ ] Type roles from §3 only; no ad-hoc size/weight/colour combinations.
- [ ] Touch targets ≥ 44px; keyboard focus visible; live regions on async state.
- [ ] Looks right in both `night` and `day` (toggle `data-theme` on `<html>` in devtools).
