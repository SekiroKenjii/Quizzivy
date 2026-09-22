# September 2026 issue remediation

The user requested remediation of every open issue and a browser review of the
released student experience against the mockups. The baseline and individual
claims are recorded in `../audits/2026-09-22-open-issues.md`.

## Decisions

- Use the mockup type scale throughout the application (#96). Keep text-entry
  controls at 16px on phones to avoid Safari's automatic input zoom, and make
  primary student touch targets at least 44px high.
- The initial pass preserved the student layouts. The user subsequently approved
  the independent redesign in `92-student-ux.md`, which supersedes that choice.
  The flag belongs beside the prompt; it
  must not narrow the answer options. Keyboard navigation starts each question
  at the top and works after clicking an answer. Text-entry controls retain
  their normal editing keys. The final next-arrow opens the submission review.
- The user approved #82: retain integrity events for 13 months; retain the
  audit log; anonymize students only on a manual request; do not automatically
  erase disabled accounts. Production deletion is a separate, explicit
  maintenance operation.
- The user has no Neon CLI access or alerting service configured. Prepare
  reproducible operational tooling and documentation for #79/#81, and report
  external configuration or production verification still required.

## Work checklist

- [x] Student keyboard, typed answers, save/submit races, exit and reload safety.
- [x] Browser review of student home, classes, intro, paper, review and results
      at phone and desktop breakpoints, including narrow desktop layouts.
- [x] #85 publish diagnostics and section/question jumps.
- [x] #90 dashboard data and #93 roster/count parity.
- [x] #95 remaining UI parity items and #96 shared typography.
- [x] #78 headers, request limits and deployment checks.
- [x] #80 mockup checks in CI.
- [x] #82 maintenance tooling and retention documentation.
- [x] #79 local restore drill/tooling and #81 opt-in monitor. Neon production
      PITR and delivered notifications remain pending external setup.
- [x] Full local tests, build, lint and generated contract verification; PR #98
      repeats the full checks in CI. Evidence and remaining production acceptance
      are recorded in `../audits/2026-09-22-remediation.md`.

## Newly reproduced defects

- Clicking a radio focuses an `input`, which the shortcut guard treats as a
  text field. Arrow keys then change its answer instead of navigating.
- The fill-blank Markdown renderer declares a new component during each
  render. Every keystroke replaces the input and loses focus; only the first
  character survives normal typing.
- Submitting while an autosave is outstanding does not wait for that save.
  A failed final save still allows submission. An old response can also clear
  dirty answers after switching attempts. Regression tests cover these races.

## Resource limits after desktop crash

Run checks sequentially on this 14.5 GiB machine with no swap. Web tests use one
worker; Go uses GOMAXPROCS=2 and -p=1. A local runner refuses to start below
2.5 GiB available and terminates its own process group below 2 GiB. Do not stop
unrelated applications. ESLint needs more than a 512 MiB heap; its isolated
ceiling is 1 GiB with the same host-memory guard.
