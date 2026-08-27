---
name: library-setup
description: >-
  Before installing, adding, upgrading, scaffolding, or configuring ANY third-party library,
  framework, SDK, cloud service, CLI, or doing project/environment setup (auth, database, payments,
  analytics, storage, deploy, styling, etc.), first fetch the CURRENT official docs and install the
  LATEST version — never rely on memory or random internet/blog snippets, which go stale (APIs get
  renamed, keys change, patterns deprecate). Use this skill whenever the user says things like
  "install X", "set up Y", "add the Z SDK", "integrate a service", "wire up auth/db/payments",
  "환경설정", "라이브러리 설치", "셋업", "연동", or whenever you are about to add a dependency or
  scaffold an integration — even if the user doesn't explicitly ask you to "check the docs".
---

# Set up libraries from the official docs, on the latest version

Your training data has a cutoff. Libraries don't. Between then and now, packages rename env vars,
swap APIs, deprecate patterns, and change their recommended setup — and the broken result often
**fails only at runtime** (a renamed env var throws on boot; a removed option is silently ignored),
which is slow and confusing to debug. Reaching for a remembered snippet or an old blog post is the
single most common way setup goes subtly wrong.

So treat any setup/integration task as **"go read the current docs first,"** not "I know this one."
Even for libraries you know well — *especially* those, because that's when you skip checking.

## The workflow

**1. Pin down the exact target — library + your stack.**
"Set up React auth" has different correct answers for a Vite SPA vs Next.js vs React Router (SSR)
vs React Native. Name the specifics that change the answer: framework, bundler, router, language,
package manager, runtime. The docs almost always have a per-stack path; you want *yours*.

**2. Read the current docs — `context7` MCP first when it's connected.**
`context7` exists precisely because training data is stale ("use even when you think you know the
answer"). Call `resolve-library-id` → `query-docs` for the library, querying for the quickstart that
matches your stack. If context7 isn't available in this session, or a detail is missing or ambiguous,
`WebFetch`/`WebSearch` the **official** site
(the library's own docs/getting-started, not a third-party tutorial). Extract: the install command,
the **current latest version**, the init/config code, **exact env-var and config-key names**, and any
migration / breaking-change / "this changed recently" notes.

**3. Install the latest, and verify what landed.**
Use the package manager's latest resolver (`pnpm add <pkg>` / `pnpm add <pkg>@latest`) rather than
hardcoding a version you remember. Then confirm the resolved version in the lockfile — don't assume.

**4. Match the docs verbatim.**
Copy env-var names, config keys, and API signatures from the docs you just read — not from memory.
This is exactly where stale snippets bite. If your recollection disagrees with the docs, the docs win.

**5. Prefer official scaffolding/registry/CLI — but only the variant that fits your stack.**
Official starters (`create-*`, shadcn registries, framework CLIs) are great *when they target your
setup*. If the only official block targets a different framework (e.g. a Next.js / React-Router SSR
starter when you're on a Vite + TanStack Router SPA), **don't force it** — it'll drag in the wrong
runtime deps and conventions. Fall back to the generic SDK-level quickstart for your stack. Forcing a
mismatched starter is the same stale-code mistake wearing a different hat.

**6. Verify, then flag caveats.**
Run the docs' own "check it works" step. Tell the user any version/transition notes you hit
(deprecations, keys in a migration window, peer-dep requirements) so today's choice doesn't surprise
them later.

## Red flags — stop and read the docs

- You're about to type a config block, env-var name, or init call **from memory**.
- The snippet you're adapting is from a blog/SO answer of unknown age.
- You're unsure what the **current** latest version is, or you're pinning an old one "to be safe".
- An env-var or API name "feels right" but you haven't seen it in today's docs.
- You're copying a starter/template without confirming it targets *your* framework + bundler.

## Why this matters — a real example (from a sibling project)

Supabase auth was first wired from memory with `VITE_SUPABASE_ANON_KEY`. The **current** docs use
`VITE_SUPABASE_PUBLISHABLE_KEY` (`sb_publishable_…`) — Supabase renamed the client key. The mismatch
threw at boot and produced a blank screen that took several round-trips to diagnose. Worse, a tempting
"official" fix — `npx shadcn add @supabase/supabase-client-react-router` — turned out to be a
**React-Router/SSR** block (it pulls `@supabase/ssr` for loaders/actions), wrong for this **Vite +
TanStack Router SPA**. Reading the docs first surfaced both: the right key name, and that the correct
setup for this stack is the plain `@supabase/supabase-js` `createClient(url, publishableKey)`
quickstart — not the framework-specific starter. Two stale-knowledge traps, both avoidable by step 2.

> Pairs with `/implement-job`: when a job's tasks add or configure a dependency, run this discipline
> as part of that task rather than installing from memory.
