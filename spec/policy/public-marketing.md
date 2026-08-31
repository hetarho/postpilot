# Policy — The public marketing page (`/about`)

Canonical rules for the product's one public explanation surface. Source:
[plan/15](../plan/15.public-marketing-page.md), built by job 34.

## The route

- `/about` is a **direct child of the root route**, beside `/login`, and is deliberately not a child of the
  authenticated pathless layout. Being public is therefore structural, not a check someone has to remember.
- It has no `beforeLoad`, no loader, no session branch and no query. Mounting it issues **no Connect call at all** —
  not even `GetMe` — because the page exists for a visitor who has no account ([I5]).
- It is refreshable and deep-linkable through the existing Cloudflare `single-page-application` fallback; there is no
  SSR, prerender, marketing deployment or metadata worker.
- `/about` carries one search param, `redirect`, which it never reads for itself: it hands the value straight back to
  `/login` so a detour through the public page does not cost a visitor the destination their session expired on. Both
  ends filter it with `isInAppPath`, so an off-site value cannot survive the round trip.
- `/login` links to it as `Postpilot이란? / What is Postpilot?`, below the credential action and **outside the form**.
  Login submission, its one generic `INVALID_CREDENTIALS` failure, its pending state and its redirect validation are
  unchanged. The signed-in guard on `/login` is unchanged: a signed-in visitor who follows About's Login CTA is
  redirected by that guard, and About itself never special-cases session state.

## Claim discipline

Every sentence on the page must be true of **shipped** behavior, as recorded in the SSOT at the time it is written.

- Planning, in-progress or unverified behavior is never presented as generally available. Automated Naver publishing
  is currently stated as an operator-tier surface whose live verification is still in progress (plan 12's job 25), and
  softening that sentence is a policy violation, not a copy improvement.
- No guarantee the product does not own: no "matches your voice perfectly", no "never stores your data", no "fully
  automatic publishing", no availability, security or performance superlative.
- No testimonial, usage metric, case study or fabricated social proof.
- The **plans section mirrors the code-owned limits table** in `backend/internal/plan` ([plans](plans.md)): daily job
  starts, daily and monthly budgets, and model access, for `free` / `basic` / `max`. It is static localized copy, not
  a `GetMyPlan` read — a visitor with no account has no plan to read. Changing the ladder means changing this copy in
  the same change; a divergence is a copy bug.
- `master` appears only as prose describing the operator tier (unlimited, owns publishing and administration) and is
  never presented as a tier a user can obtain or a fourth row of the table.

## No collection, no selling, no tracking

- There is no signup, invitation request, waitlist, contact or sales form, newsletter, comment field or CMS. The page
  contains no `<form>` and no input of any kind.
- Plans are presented, never sold: no price tag beyond the shipped budget figures, and no purchase, upgrade or
  checkout affordance. Access is private and operator-provisioned, and the page says so.
- **Login is the page's only product CTA**, in the header. The footer is product identity only — no second CTA.
- No analytics, tag manager, tracking pixel, cookie banner, advertising or marketing experiment.
- No screenshot, generated hero art, embedded media, remote font or any other third-party request. Every asset the
  page references is same-origin.

## Localization and preferences

- All visible labels, accessible names and metadata strings live in the typed `marketing` namespace for both `ko` and
  `en`, at full parity ([languages](languages.md)). The page composes keys; it embeds no language.
- Locale follows and persists by the existing browser-local rules and **never changes the URL** — there is no locale
  prefix and no automatic locale redirect ([languages](languages.md)).
- Theme follows and persists by the same single browser-local preference the app uses ([themes](themes.md)). The
  public header composes the shared `InterfacePreferences` widget rather than forking either control.

## Client-rendered metadata

- While `/about` is mounted it applies a localized `<title>` and `description`, `og:type=website`, localized
  `og:title` / `og:description` / `og:locale`, and canonical + `og:url` equal to the **current origin** plus the fixed
  `/about` path. No `og:image` is invented.
- The whole set is reapplied when the locale changes. Leaving the route removes every tag the page created and hands
  the title and description back to the app default **for the locale that is current then** — so switching language on
  `/about` and then navigating away cannot leave the previous language's title behind.
- Exactly **one** module writes document head metadata: `shared/lib/document-metadata`'s transactional
  `applyDocumentMetadata`, which records each touched tag's prior state and returns an exact undo. The i18n provider's
  app-default application goes through it too, discarding the undo because a baseline has nothing above it to restore
  to. A second head writer is forbidden — two of them cannot share the head safely.
- `index.html` keeps a **product-level** static title and description for clients that do not run JavaScript. It is
  neither route-specific nor localized and carries no Open Graph promise: it is the only metadata a crawler that
  refuses JavaScript will ever see, on every deep link, because this remains a CSR SPA. The product therefore does not
  claim localized social previews for such crawlers.

## Structure and accessibility

- One H1, descriptive H2 sections, and the product flow as a real ordered list — the order is semantic information,
  not four numerals.
- Section separation uses spacing and semantic surface steps, never bordered card stacks. Existing `Logo`, button/link
  and icon primitives are reused; no About-only control is hand-rolled.
- The header is sticky and safe-area padded so the single Login CTA stays reachable without being repeated. Controls
  keep the 44 px floor, focus is visible, and no state is carried by colour alone.
- There is no horizontal **page** scroll at 320 px. The plans table — the one element wider than that — scrolls inside
  its own container, and the page keeps exactly one vertical scroller.
