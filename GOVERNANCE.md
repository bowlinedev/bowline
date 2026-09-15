# Governance

## Roles

**Contributors** open issues and pull requests. Anyone can be a contributor.

**Maintainers** review and merge pull requests, cut releases, and answer security reports. They are listed in `MAINTAINERS.md`.

## How decisions are made

Ordinary changes are decided in the pull request. A change needs approval from one maintainer who did not write it; while there is a single maintainer, self-merge is allowed for changes that are covered by tests and do not change exported API or the contract format.

A change that alters exported API, the contract document format, the wire protocol, or a support window needs the agreement of every maintainer and a written rationale in the pull request that states the problem, the alternatives considered, and why this one was chosen. Disagreement is resolved by discussion in that pull request; if it cannot be, the change does not land.

Rationales of that kind are part of the permanent record: they are not edited after merge. Reversing one means writing a new pull request that links the old one and explains what changed.

## Adding and removing maintainers

A contributor is invited to be a maintainer after a sustained history of reviewed contributions and demonstrated judgment about what belongs in the project. Any maintainer may propose an addition in a pull request that edits `MAINTAINERS.md`; it lands with the agreement of every existing maintainer.

A maintainer may step down at any time by opening the same pull request. A maintainer who has been unreachable for six months is moved to an emeritus section by the remaining maintainers. Commit access is removed at the same time as the listing changes.

## Releases

A release is cut by a maintainer following `docs/lts.md`. The release commit, the tags, and the publish workflows are described in `CONTRIBUTING.md`.
