# Support and long-term support

## Release cadence

A minor version goes out when enough has landed to be worth one. Patch releases go out as needed and contain fixes only. A patch does not add exported identifiers, does not change the contract format, and does not change generator output except for the fix itself.

Every module in the repository shares one version number. Each `vX.Y.Z` tag is accompanied by a `<module>/vX.Y.Z` tag for every Go module in the workspace, so that `go get github.com/bowlinedev/bowline/gateway@vX.Y.Z` resolves.

## Support windows

| Line | Security fixes for |
| --- | --- |
| The current minor | until the next minor ships |
| Every other minor of the current major | twelve months from its release date |
| The last minor of a previous major | twenty-four months from the release of the next major |

Twelve months is the window Go and Node users already plan around. The last minor of a major gets a longer window so that upgrading across a major version does not have to happen under time pressure.

## What gets backported

Only two kinds of change are backported to a supported line:

- security fixes, including fixes to the signing, CSRF, and rate limiting code
- data-loss and data-corruption fixes, including any contract or generator bug that silently produces a client which reads the wrong field

Everything else (features, performance work, new generator targets, dependency bumps that are not security fixes) lands on the current minor only.

A backport includes the fix and its test and nothing else. If a fix cannot be applied to a line without an API change, that line does not get the fix. The advisory will say so and name the minimum version that has it.

## Contract format

The contract document format is versioned separately from the library. A format version stays supported for as long as any supported library line can read it. `bowline migrate-contract` converts an older document forward and will not be removed.

## End of life

When a line reaches the end of its window, this is noted in the changelog of the next release. There is no separate announcement channel. Watch releases on GitHub if you need to know.
