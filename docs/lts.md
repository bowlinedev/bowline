# Support and long-term support

## Release cadence

Bowline releases a minor version when a milestone lands. Patch releases go out as needed and contain fixes only: no new exported identifiers, no contract format changes, no generator output changes beyond the fix.

Every module in the repository shares one version number. A `vX.Y.Z` tag is accompanied by a `<module>/vX.Y.Z` tag for each Go module, so `go get github.com/bowlinedev/bowline/gateway@vX.Y.Z` resolves.

## Support windows

| Line | Security fixes for |
| --- | --- |
| The current minor | until the next minor ships |
| Every other minor of the current major | twelve months from its release date |
| The last minor of a previous major | twenty-four months from the release of the next major |

Twelve months matches what Go and Node users already plan around. The longer window on the last minor of a major exists so an upgrade across a major version never has to happen under time pressure.

## What gets backported

Only two classes of change are backported to a supported line:

- security fixes, including fixes to the signing, CSRF, and rate limiting paths
- data-loss and data-corruption fixes, including a contract or generator bug that silently produces a client that reads the wrong field

Everything else — features, performance work, new generator targets, dependency bumps that are not security fixes — lands on the current minor only.

A backport keeps the fix's test and nothing else. If a fix cannot be applied without an API change, the line does not get the fix; the advisory says so and names the minimum version that has it.

## Contract format

The contract document format is versioned separately from the library. A format version is supported for as long as any supported library line can read it. `bowline migrate-contract` converts an older document forward and is never removed.

## End of life

A line reaching the end of its window is announced in the changelog of the release that follows. There is no separate announcement channel; watch releases on GitHub.
