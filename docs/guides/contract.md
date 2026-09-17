# The contract and the breaking-change gate

`bowline gen` writes `bowline.contract.json`. This is the language-neutral description of your API that every generator reads. Commit it. Two commands keep it accurate.

## Drift

`bowline check` regenerates everything in memory and fails if any committed file differs from what the Go code would produce now. Run it in CI on every push. The ledger example does this in `.github/workflows/ci.yml`.

## Semantic diff

`bowline diff old.json new.json` compares two contract documents and puts every change into one of these categories:

| Category | Meaning |
|---|---|
| `added` | something new that no existing client can notice |
| `widened` | a client may send less or receive more, still compatible |
| `narrowed` | a client receives less; compatible for inputs, not for outputs |
| `removed` | something a client may depend on is gone |
| `breaking` | an existing client would fail to compile or would misbehave |

The rules follow variance. Inputs are contravariant, so adding a required input field breaks callers while adding an optional one widens. Outputs are covariant, so removing an output field breaks readers while adding one is `added`. Adding a value to an enum that appears in an output is breaking, because an exhaustive `switch` on the client no longer compiles. The full set of rules is in `spec/diff.md`.

## The gate

`bowline check --against origin/main` reads the contract committed at that ref, regenerates the current one, prints the diff as markdown, and exits 1 if there is any breaking change, unless `--allow-breaking` is passed. The composite action in `.github/actions/contract-gate` runs it for a directory and posts a comment on the pull request, updating the same comment on later pushes:

```yaml
- uses: bowlinedev/bowline/.github/actions/contract-gate@main
  with:
    working-directory: services/billing
    base-ref: origin/${{ github.base_ref }}
```

If you push again with the change reverted, the comment is updated to say `no contract changes`. An intentional break should be shipped with `allow-breaking: "true"` and a version bump, so that it is visible.

If the repository has recorded consumer contracts under `contracts/consumers`, the report gets an extra Consumers column. Each breaking change lists the consumers whose recorded interactions read the changed field, with their interaction counts. A breaking change that no consumer uses is labeled `unused by consumers`. `bowline diff --consumers <dir>` does the same for two documents on disk. See `consumer-contracts.md`.
