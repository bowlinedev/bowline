# The contract and the breaking-change gate

`bowline gen` writes `bowline.contract.json`, the language-neutral description of your API that every generator reads. Commit it. Two commands keep it honest.

## Drift

`bowline check` regenerates everything in memory and fails when a committed file differs from what the Go code would produce today. Run it in CI on every push; the ledger example does in `.github/workflows/ci.yml`.

## Semantic diff

`bowline diff old.json new.json` compares two contract documents and classifies every change:

| Category | Meaning |
|---|---|
| `added` | something new that no existing client can notice |
| `widened` | a client may send less or receive more, still compatible |
| `narrowed` | a client receives less; compatible for inputs, not for outputs |
| `removed` | something a client may depend on is gone |
| `breaking` | an existing client would fail to compile or would misbehave |

The rules follow variance. Inputs are contravariant: adding a required input field breaks callers, adding an optional one widens. Outputs are covariant: removing an output field breaks readers, adding one is `added`. Adding a value to an enum that appears in an output is breaking, because an exhaustive `switch` on the client stops compiling. The full table is in `spec/diff.md`.

## The gate

`bowline check --against origin/main` reads the contract committed at that ref, regenerates the current one, prints the diff as markdown, and exits 1 on any breaking change unless `--allow-breaking` is passed. The composite action in `.github/actions/contract-gate` runs it for a directory and posts or updates one comment on the pull request:

```yaml
- uses: bowlinedev/bowline/.github/actions/contract-gate@main
  with:
    working-directory: services/billing
    base-ref: origin/${{ github.base_ref }}
```

Re-pushing with the change reverted edits the same comment to `no contract changes`. A deliberate break ships with `allow-breaking: "true"` and a version bump, never silently.

When the repository carries recorded consumer contracts under `contracts/consumers`, the report gains a Consumers column: every breaking change names the consumers whose recorded interactions read the changed field, with their interaction counts, and a breaking change nobody uses is labeled `unused by consumers`. `bowline diff --consumers <dir>` does the same for two documents on disk; see `consumer-contracts.md`.
