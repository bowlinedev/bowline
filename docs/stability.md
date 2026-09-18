# Stability guarantee

This page lists what will not change within a major version, so that you can depend on it without reading every release note. It applies from 1.0 onwards. The 0.x line was additive in practice, and `docs/migration-0.x.md` describes what a 0.x upgrade requires.

Six areas are covered. Anything not listed here is not guaranteed.

## 1. The Go API

Every module in this repository follows semantic versioning, and all of them share one version number. Within 1.x:

- no exported identifier is removed
- no exported function or method changes its signature
- no exported struct field is removed or changes type
- new identifiers, fields, and options are added in minor releases

Adding a field to an exported struct is a minor change, so you should construct structs with field names rather than positionally. Adding a method to an exported interface is not a minor change. Interfaces that you implement are frozen for the major version.

`docs/api-freeze.md` lists every identifier this covers. `TestPublicIdentifiersMatchFreezeList` fails if that list and the real exported surface ever disagree. `scripts/apidiff.sh` compares the exported surface against the previous release tag on every pull request. While the previous tag is a `v0.x` it only reports what changed; from the first `v1` tag onwards it fails the build on an incompatible change. `scripts/apidiff.sh --strict` fails on incompatible changes at any version and is what the release checklist runs. `docs/security/api-audit.md` records the reason each identifier is public.

Packages under `internal/` in any module are not part of the API and are not covered. This includes every generator. Third parties should use the external generator protocol in `docs/plugins.md` rather than importing the generator packages.

### Provisional identifiers

`docs/provisional.md` names the one carve-out from the promise above. The identifiers it lists arrived in 1.1.0, have not been used outside this repository yet, and may change or be removed in a later 1.x minor release. 1.0.0 froze the API three days after the first commit, which is a promise made before any evidence; naming the parts that are still settling is better than quietly breaking them later or carrying a shape that turns out wrong for the rest of the major version.

`TestProvisionalSurfaceIsExported` keeps that list from rotting, and `scripts/apidiff.sh` reports a change to one of those identifiers as `provisional:` rather than failing. Everything not on that list is covered in full. The `otel` module is provisional as a whole, and is not in `docs/api-freeze.md` at all: only the root, `contract` and `signing` packages are frozen.

### Minimum Go version

`apidiff` cannot check the minimum Go version, so it has its own rule. The minimum may go up in a minor release, never in a patch, and never beyond the oldest release that Go itself still supports. A minor release that raises it says so in the first line of its changelog entry.

The runtime and every module an application imports declare `go 1.24`. The CLI declares a newer version, because `golang.org/x/tools` only supports the two most recent Go releases. That higher floor does not affect applications, because `go install` fetches whatever toolchain the CLI needs under the default `GOTOOLCHAIN=auto`.

## 2. The contract document

The document has its own format version, separate from the library version.

- every 1.x reader can read every 1.x document
- new optional fields are a minor format bump, and a reader that does not know a field ignores it
- no field is removed or changes meaning within 1.x
- `contract.Version` carries that format version, so its value moves with a minor format bump
- if a 2.0 format is ever released, `bowline migrate-contract` will convert 1.x documents forward, and readers will keep accepting 1.x documents for the life of the 1.x library line

`spec/contract.md` is the normative description and `spec/contract.schema.json` validates it.

## 3. Generated code

Regenerating with a newer 1.x CLI against an unchanged Go module produces code that compiles against the same major version of every client package. What the generated code does with a given contract does not change.

The layout of generated files can change between minor versions. Formatting, declaration order, and how output is split across files are not guaranteed. This is why generated files are committed and diffed: a regeneration shows up as a reviewable diff rather than as a surprise at runtime.

## 4. The CLI

Every command and flag listed in `docs/cli.md` is stable within 1.x. New flags are additive, and every new flag has a default that keeps the existing behaviour.

Human-readable output is not stable and should not be parsed. The machine-readable outputs are stable and are the ones to script against:

- `bowline check --json`
- `bowline diff --json`, or equivalently `bowline diff --format json`

Within 1.x these documents may gain fields but do not lose them or change a field's type. Exit codes are also stable: `0` means success, `1` means the command ran and the answer was no, `2` means the command line was wrong.

### Version constants

`bowline.Version` is the library version and `contract.Version` is the document format version. Both are expected to change, so their values are the only exported values not frozen; `scripts/apidiff.sh` exempts them, and everything else about them, including their existence and type, is frozen as usual.

## 5. The wire format

The wire format is frozen for 1.x, because a client generated by one version needs to work with a server built with another. This covers:

- the response envelope and its error shape
- the sixteen error codes and the HTTP status of each
- by default, `GET` for queries with the input in the `input` query parameter, and `POST` with a JSON body for everything else
- for a procedure that declares its own route with `Path` and `Method`, the path parameters, the query string for a method that carries no body, and a JSON body for one that does
- the server-sent event names used for subscriptions, and the heartbeat
- the multipart layout for uploads
- the idempotency key header and how it behaves

`spec/mapping-table.md` is the normative mapping from Go types to the wire. The conformance suite in the `conformance` package is the executable form of this section. Any mount that passes it speaks the frozen protocol.

## 6. Support

Release cadence, how long each line gets security fixes, and what qualifies for a backport are described in `docs/lts.md`.

## Reporting a break

If a release breaks something that this page guarantees, that is a bug and not a migration. Open an issue with the two versions and the smallest reproduction you have. If the break has a security impact, report it privately through the repository's security advisories instead.
