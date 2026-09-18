# Provisional API

Everything in `api-freeze.md` is guaranteed for the life of 1.x, with one carve-out: the identifiers below. They are exported, documented and tested like the rest, but they arrived in 1.1.0 and have not been used by anyone outside this repository yet. They may change, or be removed, in a later 1.x minor release.

The reason for saying so out loud is that 1.0.0 froze the API three days after the first commit. Freezing a design nobody has used is a promise made without evidence. Rather than quietly break it later, or carry a shape that turns out wrong for the rest of the major version, this page names the parts that are still settling.

If you use one of these, pin a minor version and read the changelog before upgrading. If you need one of them to be stable, say so in an issue: that is the evidence this page is waiting for.

`TestProvisionalSurfaceIsExported` checks that every identifier here still exists, so the list cannot rot. `scripts/apidiff.sh` reports a change to one of them as `provisional:` instead of failing.

## github.com/bowlinedev/bowline

```
func CallTimeout
func Drain
func Method
func Observe
func ObserveFunc
func OnHandlerReady
func Path
func Typed
type Observer
type TypedNext
```

`Path` and `Method` declare a REST route. The open questions are whether a route should be able to set a response status, how two services should resolve a collision when they are composed behind a gateway, and whether `Method` should accept a verb outside the five it takes today.

`Observer`, `Observe`, `ObserveFunc` and `OnHandlerReady` are the lifecycle hooks. The open question is whether `CallFinished` should also see the response body and status, which it cannot today.

`CallTimeout` and `Drain` bound a call and end streams at shutdown. The open questions are whether a per-procedure override belongs alongside the handler-wide deadline, and whether `Drain` should also refuse new calls rather than only ending open streams.

`Typed` and `TypedNext` give a middleware the procedure's real input and output types. The open question is whether the same should be expressed as a generic `Procedure` wrapper instead, which would remove the runtime type assertion.

## github.com/bowlinedev/bowline/contract

```
func ParsePath
func PathParams
type PathSegment
```

These are public only because the runtime, the analyzer and every generator parse the same path templates and must agree. If the route syntax gains a feature, such as a wildcard or a typed parameter, this is where it lands.

## github.com/bowlinedev/bowline/otel

The whole module is provisional. It is not in `api-freeze.md` at all, because only the root, `contract` and `signing` packages are frozen. The attribute and metric names it reports follow OpenTelemetry semantic conventions that are themselves still moving.
