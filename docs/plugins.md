# External generators

A target in `bowline.json` normally names one of the generators built into the CLI. It can instead name a command. Bowline then runs that command to produce the target's files. This is how you add support for a new language. The generator packages under `cmd/bowline/internal` are not a public API, but the protocol described on this page is.

```json
{
  "entry": "./api.Routes",
  "targets": {
    "kotlin": { "command": "bowline-gen-kotlin", "out": "client/Api.kt" }
  }
}
```

You can pick any target name that the CLI does not already use. `command` is a command name, not a path. It is looked up on `PATH` and never resolved relative to the module directory. This means that checking out a repository cannot cause `bowline gen` to run a program shipped inside that repository.

## The protocol

Bowline runs the command once per target. It provides:

- the contract document, as JSON, on **stdin**
- `BOWLINE_CONTRACT_VERSION`, the document's format version, for example `1.2`
- `BOWLINE_OUT`, the target's `out` path, relative to the module

It waits up to two minutes and then looks at the exit code:

| Exit code | Meaning |
|---|---|
| 0 | success; stdout is the output, in one of the two shapes below |
| 2 | diagnostics; stderr carries one per line and nothing is written |
| anything else | failure; stderr is reported and nothing is written |

"Nothing is written" applies to the whole run, not just to the failing target. If any generator fails or reports a diagnostic, `bowline gen` does not write any files, including the contract. This avoids leaving a module in a half-generated state.

### Single file

This is the common case. Exit 0 and write the file's content to stdout. It is written to `out`.

### Several files

Write the line `bowline-files/1` first, followed by a JSON object mapping paths to content:

```
bowline-files/1
{"files": {"Api.kt": "…", "Models.kt": "…"}}
```

Paths are relative to the directory containing `out`. With `"out": "client/Api.kt"`, the two files above are written to `client/Api.kt` and `client/Models.kt`. A path can go into subdirectories, which are created if needed. It cannot be absolute and cannot contain a `..` element.

### Diagnostics

If a generator cannot represent something in the contract, it should report that rather than fall back to a dynamic type. All of the built-in generators follow this rule. Write one diagnostic per line to stderr, using the same format the analyzer uses, and exit with status 2:

```
file:line:col: path: message. fix
```

The position at the start is optional. Everything after the last `. ` is treated as the suggested fix. Bowline prints the diagnostics in the same way it prints its own and exits with status 1.

## `bowline check`

External targets are checked the same way as built-in ones. `bowline check` runs the generator and compares each file it produces with the file on disk, reporting `ok`, `outdated`, or `missing` for each. If you commit the generated files, the drift check works with no extra configuration.

## A generator in Go

sketch: a complete minimal generator, written out here because an external generator lives in its own repository, not this one

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

Build this as `bowline-gen-kotlin`, put it on `PATH`, and `bowline gen` will use it.

## A generator in Node

The same generator, but writing two files:

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

A generator that passes the checks in `docs/certification.md` can be added to `docs/certified.md`. `bowline certify --target <name> --generator <command>` runs those checks against your command using the same protocol, so you can check the result before submitting.

## What the contract document looks like

`spec/contract.md` describes the document your generator reads, and `spec/mapping-table.md` describes how Go types map to it. A good way to get started is to generate a document for your own module with `bowline gen` and read it alongside those two pages. Every built-in generator reads exactly this shape.
