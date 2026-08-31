# Frontend engineer — Vue 3, accessibility, and the untested half

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/04-frontend.md`.

The Go half of this project has ~2,500 lines of tests and two architecture
tests. The Vue half has none, no linter, and six `aria-` attributes and zero
keyboard handlers across the whole app. That asymmetry is your subject.

**Read first:** `web/src/` in full — about 2,500 lines. Note especially
`api.js`, `session.js`, `router.js`, `views/TaskView.vue` (the screen that
matters), and `components/StatePanel.vue`, `TransitionActions.vue`,
`Worklog.vue`.

## The job

1. **Correctness first.** Find the real bugs: unhandled promise rejections,
   stale state after a failed transition, double-submit on the transition
   buttons, race conditions between `load()` and an action, error paths that
   leave the UI claiming something that did not happen. A transition that fails
   on the server but appears to succeed in the browser is the worst bug this
   product can have, because the board is meant to be the one place that never
   lies.

2. **Accessibility, at the level this product actually needs:** keyboard
   reachable controls, visible focus, correct roles on the confirm dialog and
   the transition forms, form labels tied to inputs, colour contrast in both
   themes, and status conveyed by more than colour alone. This is a single-user
   tool, so scope it to what a keyboard user genuinely needs rather than to a
   compliance checklist — but say which WCAG 2.1 AA failures you found and which
   you are deliberately not fixing.

3. **Propose the smallest testing setup that is worth its weight.** The bar: it
   must run in CI in seconds, add no runtime dependency to the binary, and catch
   the class of bug in (1). Argue for or against Vitest plus `@vue/test-utils`
   concretely — including the option of adding no frontend test framework at all
   and instead covering these flows from Go's `httptest` against the real API.
   Recommend one and say what it costs.

4. **Review the API client layer.** Every fetch, every error shape, every place a
   401 can arrive mid-session, and how the session is re-established. The dev
   server proxies `/api` so cookies stay same-origin; make sure nothing depends
   on that in production.

5. **Bundle and build.** The whole app is embedded in a binary people run
   offline. Report the current bundle size, and flag anything that fetches at
   runtime, assumes a network, or would break in an airgapped install.

## Do not

- Do not add a component library, a CSS framework, an icon package, or a state
  management library. Two runtime dependencies is the budget.
- Do not convert the app to TypeScript as part of this. If you think it should
  be, make it a separate one-paragraph recommendation with an honest cost.
- Do not restructure the components to be "more reusable." This is a six-screen
  app with one user.

## Deliverable

Bugs first, as a ranked list with `file:line`, a concrete failure scenario for
each ("with the network dropped between click and response, the status mark
shows active while the server still says queue"), and the fix. Then the
accessibility findings, then the testing recommendation with the cost stated.
Write the patches for anything under twenty lines.
