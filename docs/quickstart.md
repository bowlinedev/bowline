# Quickstart

This page sets up a Go module with one procedure, generates a TypeScript client for it, and makes a call. It takes about five minutes.

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

This writes two files, `bowline.contract.json` and `web/src/bowline.ts`. Commit both of them. In CI, `bowline check` will fail if either one is out of date compared to the Go code.

## 3. Call it from TypeScript

    cd web && npm init -y && npm install @bowlinedev/client typescript

`src/main.ts`:

    import { createClient } from "./bowline.js";

    const client = createClient({ url: "http://localhost:8080/api" });
    const greeting = await client.greet({ name: "ada" });
    console.log(greeting.message);

`greeting` has the type `Greeting`, and `client.greet` requires a `name` argument. A typo in either is a compile error. Start the server with `go run .` in the module directory, then run `main.ts` with your bundler or with `node --experimental-strip-types src/main.ts`.

## 4. Keep it in sync while you work

    bowline dev

This watches the module and rewrites `bowline.ts` whenever a save changes the contract. It normally takes well under a second.

## Notes

- Queries are sent as `GET` with the input in the `input` query parameter. This means the input shows up in URLs and access logs. If that is a problem for a particular query, mark it with `bowline.Sensitive()` and it will use `POST` instead.
- Mutations are sent as `POST` with a JSON body.
- Errors are returned as a `BowlineError` with a `code` from a fixed set. Validation failures also include an `issues` list.
