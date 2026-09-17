# Contract registry

The registry answers one question before a change is merged: who breaks if I remove this field. It stores every service, every published contract version, and what each consumer actually uses, and joins these using the semantic diff.

It is self-hosted, optional, and file-backed by default. Nothing else in Bowline depends on it. The gateway reads contracts from disk, and the breaking-change gate works from git alone.

## Running it

```bash
bowline registry serve --store ./registry-data --listen :8095 --token "$CI_TOKEN"
```

Reads are open. Every write needs an `Authorization: Bearer <token>` header matching one of the configured tokens, so with no token configured the server is read-only. Records are stored as plain JSON under the store directory, with one file per service, version, tag, consumer, and composition, so they can be read and diffed by hand.

The server includes a browser UI at `/`, built into the binary. It lists every service with its owners and latest version, shows a version's hash, ref, and tags next to the consumers recorded against it, draws the dependency graph, lets you browse a contract's types and procedures, and runs an impact query against a document you paste or upload. Pass `--ui=false` to serve only the API.

## Publishing

```bash
bowline publish --registry "$REGISTRY" --service ledger --tag main --ref "$GITHUB_SHA"
```

This runs the analyzer in the current module and posts the resulting document. Publishing the same hash twice is a no-op, so re-running CI on a green commit changes nothing. A consumer registers what it uses with the file the recorder already writes:

```bash
bowline publish --registry "$REGISTRY" --consumer web --provider ledger --usage contracts/consumers/web.json
```

## Asking who breaks

```bash
bowline check --registry "$REGISTRY" --service ledger
```

This regenerates the local document, posts it, and prints the report. The registry diffs it against the version tagged `main` and checks every breaking or narrowing change against each consumer's recorded usage. A hit is a removed output field that a consumer reads, a removed procedure it calls, a new required input field on a procedure it calls, or an enum value it has seen. Each hit names the consumer and the reason:

```
broken    consumer web: procedure invoices.list output field items elem field total: field removed; reads items.*.total
```

The command exits 1 if any consumer is affected. A breaking change that does not touch a known consumer is reported under `unattributed` and exits 0, because the registry cannot know about every consumer. Pass `--strict` to make those fail too.

source: registry/impact.go:16-30

```go
type ImpactReport struct {
	Baseline            string            `json:"baseline"`
	Changes             []contract.Change `json:"changes"`
	Affected            []Affected        `json:"affected"`
	Unattributed        []contract.Change `json:"unattributed"`
	OK                  bool              `json:"ok"`
	SkippedCompositions bool              `json:"skippedCompositions,omitempty"`
}

type Affected struct {
	Consumer string          `json:"consumer"`
	Via      string          `json:"via,omitempty"`
	Change   contract.Change `json:"change"`
	Reason   string          `json:"reason"`
}
```

`Via` names the gateway when a consumer reaches the service through a composition rather than directly. This is how a change to the ledger can be attributed to a browser that only talks to the edge gateway.

## In a pull request

The gate is the same command teams already run, plus one flag. A pull request that removes a field then fails with the consumer's name instead of a generic warning. Publish on merge, and check on every pull request:

```yaml
- run: bowline check --registry ${{ vars.BOWLINE_REGISTRY }} --service ledger
  env:
    BOWLINE_REGISTRY_TOKEN: ${{ secrets.BOWLINE_REGISTRY_TOKEN }}
```

## Seeing it work

`examples/federation` has a runnable version of all of this. `./examples/federation/registry-seed.sh` starts a registry, publishes the ledger and billing contracts, and registers what the ledger's web consumer uses. `./examples/federation/impact-check.sh` then copies the ledger, renames the JSON tag of a field that consumer reads, and runs the gate:

```
breaks    ledger-web: procedure invoices.create output field total (reads total)
bowline: 1 consumer break(s)
```

Both scripts run in the `federation` end-to-end job on every push.

The registry is one more place to look when the answer is not obvious from the repository itself. Everything it knows came from an artifact that some pipeline produced: a contract from the provider's build, a usage file from the consumer's tests, a composition from a gateway's deploy.
