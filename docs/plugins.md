# External generators

A target in `bowline.json` normally names a generator built into the CLI. It can instead name a command, and Bowline will run that command to produce the target's files. This is the supported way to add a language: the generator packages inside `cmd/bowline/internal` are not a public API, the protocol on this page is.

```json
{
  "entry": "./api.Routes",
  "targets": {
    "kotlin": { "command": "bowline-gen-kotlin", "out": "client/Api.kt" }
  }
}
```

The target's name is yours to pick and must not be one the CLI already has. `command` is a command name, not a path: it is resolved on `PATH` and never from the module directory, so checking out a repository can never make `bowline gen` run a program that repository ships.

## The protocol

Bowline runs the command once per target, with:

- the contract document, as JSON, on **stdin**
- `BOWLINE_CONTRACT_VERSION`, the document's format version, such as `1.2`
- `BOWLINE_OUT`, the target's `out` path, relative to the module

It waits up to two minutes and reads the exit code:

| Exit code | Meaning |
|---|---|
| 0 | success; stdout is the output, in one of the two shapes below |
| 2 | diagnostics; stderr carries one per line and nothing is written |
| anything else | failure; stderr is reported and nothing is written |

"Nothing is written" is whole-run, not per-target: if any generator fails or reports a diagnostic, `bowline gen` writes no files at all, including the contract. A half-generated module is never left behind.

### Single file

The common case. Exit 0 and write the file's content to stdout; it lands at `out`.

### Several files

Write the line `bowline-files/1` first, then a JSON object mapping paths to content:

```
bowline-files/1
{"files": {"Api.kt": "…", "Models.kt": "…"}}
```

Paths are relative to the directory of `out`, so with `"out": "client/Api.kt"` those two files land at `client/Api.kt` and `client/Models.kt`. A path may descend into subdirectories, which are created as needed; it may not be absolute and may not contain a `..` element.

### Diagnostics

A generator that cannot represent something in the contract must say so rather than degrade to a dynamic type — that is the reject rule every built-in generator follows. Write one diagnostic per line on stderr, in the same shape the analyzer uses, and exit 2:

```
file:line:col: path: message. fix
```

The leading position is optional; everything after the final `. ` is treated as the suggested fix. Bowline prints them exactly as it prints its own and exits 1.

## `bowline check`

External targets are checked like built-in ones. `bowline check` runs the generator and compares every file it produces against what is on disk, reporting `ok`, `outdated`, or `missing` per file. Commit the generated files and the drift gate works with no extra configuration.

## A generator in Go

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type document struct {
	Bowline    string `json:"bowline"`
	Procedures []struct {
		Path   string `json:"path"`
		Kind   string `json:"kind"`
		Method string `json:"method"`
	} `json:"procedures"`
}

func main() {
	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var doc document
	if err := json.Unmarshal(body, &doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var out strings.Builder
	fmt.Fprintf(&out, "// generated from contract %s; do not edit\n", doc.Bowline)
	for _, p := range doc.Procedures {
		if p.Kind == "subscription" {
			fmt.Fprintf(os.Stderr, "%s: subscriptions are not supported yet. drop the target or remove the subscription\n", p.Path)
			os.Exit(2)
		}
		fmt.Fprintf(&out, "fun %s()\n", p.Path)
	}
	fmt.Print(out.String())
}
```

Build it as `bowline-gen-kotlin`, put it on `PATH`, and `bowline gen` picks it up.

## A generator in Node

The same generator writing two files:

```js
#!/usr/bin/env node
const chunks = [];
process.stdin.on("data", (c) => chunks.push(c));
process.stdin.on("end", () => {
  const doc = JSON.parse(Buffer.concat(chunks).toString());
  const header = `// generated from contract ${doc.bowline}; do not edit\n`;
  const calls = doc.procedures.map((p) => `export function ${p.path.replace(".", "_")}() {}`).join("\n");
  const models = Object.keys(doc.types).map((id) => `// ${id}`).join("\n");

  process.stdout.write("bowline-files/1\n");
  process.stdout.write(
    JSON.stringify({
      files: {
        [require("path").basename(process.env.BOWLINE_OUT)]: header + calls + "\n",
        "models.js": header + models + "\n",
      },
    }),
  );
});
```

## Getting it certified

A generator that passes the checks in `docs/certification.md` can be listed in `docs/certified.md`. `bowline certify --target <name> --generator <command>` runs those checks against your command through this same protocol, so you can see the result before submitting.

## What the contract document looks like

`spec/contract.md` is the normative description of the document your generator reads, and `spec/mapping-table.md` is the normative mapping from Go types to it. Generate a document for your own module with `bowline gen` and read it alongside those two pages; every built-in generator is a reader of exactly that shape.
