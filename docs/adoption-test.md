# Ten-minute adoption test

Run this before every minor release with someone who has not used Bowline. Record the result in the release pull request.

## Setup

- A clone of `examples/ledger` with `bowline.json`, `api/bowline.contract.json`, `web/src/bowline.ts`, and everything under `web/src` except `bowline.ts` deleted, so only the Go server remains.
- Go 1.24 or later, Node 22 or later, pnpm installed. No prior reading.

## Task

Starting a timer, the tester must:

1. Read `docs/quickstart.md`.
2. Add `bowline.json` and run `bowline gen`.
3. Create a Vite React app in `web/` that lists invoices using `@bowline/client` and `@bowline/react-query`, with `createdAt` rendered through `toLocaleDateString()`.
4. Stop the timer when the list renders in a browser.

## Record

| Date | Tester role | Minutes | Blockers |
|---|---|---|---|
| 2026-09-15 | automated run of `scripts/quickstart.sh` by the author, not a fresh tester | 0.2 | none; a run by a person new to Bowline is still owed |

A result over ten minutes is a documentation bug first. File an issue for every blocker before the release.
