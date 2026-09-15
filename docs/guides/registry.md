# Contract registry

The registry answers one question before a change merges: who breaks if I remove this field. It stores every service, every published contract version, and what each consumer actually uses, and it joins them with the semantic diff.

It is self-hosted, optional, and file-backed by default. Nothing else in Bowline needs it: the gateway reads contracts from disk, and the breaking-change gate works from git alone.

## Running it

```bash
bowline registry serve --store ./registry-data --listen :8095 --token "$CI_TOKEN"
```

Reads are open; every write needs `Authorization: Bearer <token>` matching one of the configured tokens, so with no token the server is read-only. Records are plain JSON under the store directory, one file per service, version, tag, consumer, and composition, so a human can read and diff them.

## Publishing

```bash
bowline publish --registry "$REGISTRY" --service ledger --tag main --ref "$GITHUB_SHA"
```

runs the analyzer in the current module and posts the fresh document. Publishing the same hash twice is a no-op, so re-running CI on a green commit changes nothing. A consumer registers what it uses with the file the M4 recorder already writes:

```bash
bowline publish --registry "$REGISTRY" --consumer web --provider ledger --usage contracts/consumers/web.json
```

## Asking who breaks

```bash
bowline check --registry "$REGISTRY" --service ledger
```

regenerates the local document, posts it, and prints the report. The registry diffs it against the version tagged `main` and intersects every breaking or narrowing change with each consumer's recorded usage: a removed output field a consumer reads, a removed procedure it calls, a new required input field on a procedure it calls, or an enum value it has seen. Each hit names the consumer and why:

```
broken    consumer web: procedure invoices.list output field items elem field total: field removed; reads items.*.total
```

The command exits 1 when any consumer is affected. A breaking change that touches no known consumer is reported under `unattributed` and exits 0, because the registry cannot know every consumer; `--strict` makes those fail too.

source: registry/impact.go:14-28

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

`Via` names the gateway when a consumer reaches the service through a composition rather than directly, so a change to the ledger can be attributed to a browser that only ever talks to the edge gateway.

## In a pull request

The gate is the command teams already run with one more flag, so a pull request that removes a field fails with the consumer's name rather than a generic warning. Publish on merge, check on every pull request:

```yaml
- run: bowline check --registry ${{ vars.BOWLINE_REGISTRY }} --service ledger
  env:
    BOWLINE_REGISTRY_TOKEN: ${{ secrets.BOWLINE_REGISTRY_TOKEN }}
```

The registry is one more place to look when the answer is not obvious from the repository. Everything it knows came from an artifact some pipeline produced: a contract from the provider's build, a usage file from the consumer's tests, a composition from a gateway's deploy.
