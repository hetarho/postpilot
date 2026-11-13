# SETTINGS
> Package behavior the owner chose. Show or change with /gusta-preference (`vibe settings`); decide unset (?) keys with /gusta-init. A missing key uses its default.

- land: main  # branch finished work lands on (gusta-start session end, gusta-self-improve-remote, work groups); current = stay on the branch the work was done on
- push: true  # true = push land to the remote once at session end or loop stop (every push runs remote CI)
- agents: codex,claude  # AI tools gusta runs and recommends for tasks (gusta agents lists their models), comma-separated in fallback order; a tool at its usage limit falls back only within this list
- version: auto  # auto = when vibe push or vibe deploy moves the release branch (deploy, else land) it adds a commit raising the root package.json version: minor when a SSOT decision is new, else patch; a version a person changed stands (the only way to a major); never with land-via pr; off = the CLI never touches the version
- agent-block: true  # true = gusta keeps its managed block (between the gusta:begin/gusta:end comments) in this project's CLAUDE.md/AGENTS.md — run gusta agent-block after setting it; false = it never writes those files (gusta agent-block --remove deletes an existing block)
