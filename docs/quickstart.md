# Quickstart

Five minutes from an empty directory to a typed call in TypeScript.

## 1. A Go module with one procedure

    mkdir hello && cd hello
    go mod init example.com/hello
    go get github.com/bowlinedev/bowline@latest

`api/api.go`:

    package api

    import (
        "context"

        "github.com/bowlinedev/bowline"
    )

    type GreetInput struct {
        Name string `json:"name" validate:"required"`
    }

    type Greeting struct {
        Message string `json:"message"`
    }

    // Greet says hello.
    func Greet(ctx context.Context, in GreetInput) (Greeting, error) {
        return Greeting{Message: "hello, " + in.Name}, nil
    }

    func Routes() *bowline.Router {
        return bowline.NewRouter(bowline.Query("greet", Greet))
    }

`main.go`:

    package main

    import (
        "log"
        "net/http"

        "example.com/hello/api"
    )

    func main() {
        mux := http.NewServeMux()
        mux.Handle("/api/", api.Routes().Handler())
        log.Fatal(http.ListenAndServe(":8080", mux))
    }

## 2. Generate the contract and the client

    go install github.com/bowlinedev/bowline/cmd/bowline@latest

`bowline.json`:

    {
      "entry": "./api.Routes",
      "targets": { "ts": { "out": "web/src/bowline.ts" } }
    }

    bowline gen

You now have `bowline.contract.json` and `web/src/bowline.ts`. Commit both. `bowline check` in CI fails when they drift from the Go code.

## 3. Call it from TypeScript

    cd web && npm init -y && npm install @bowlinedev/client typescript

`src/main.ts`:

    import { createClient } from "./bowline.js";

    const client = createClient({ url: "http://localhost:8080/api" });
    const greeting = await client.greet({ name: "ada" });
    console.log(greeting.message);

`greeting` is typed as `Greeting`, `client.greet` requires `name`, and a typo in either is a compile error. Run `go run .` in the module and execute `main.ts` with your bundler or `node --experimental-strip-types src/main.ts`.

## 4. Keep it in sync while you work

    bowline dev

Every save that changes the contract rewrites `bowline.ts` in well under a second.

## Notes

- Queries are `GET` with the input in the `input` query parameter. Inputs therefore appear in URLs and access logs; mark a query `bowline.Sensitive()` to force `POST`.
- Mutations are `POST` with a JSON body.
- Errors arrive as `BowlineError` with a `code` from a fixed set and, for validation failures, an `issues` list.
