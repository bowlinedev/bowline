#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
start=$(date +%s)

cd "$work"
mkdir hello && cd hello
go mod init example.com/hello >/dev/null
go mod edit -replace "github.com/bowlinedev/bowline=$repo"
go get github.com/bowlinedev/bowline@v0.0.0 >/dev/null 2>&1 || go mod tidy >/dev/null

mkdir api
cat > api/api.go <<'EOF'
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

func Greet(ctx context.Context, in GreetInput) (Greeting, error) {
	return Greeting{Message: "hello, " + in.Name}, nil
}

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("greet", Greet))
}
EOF

cat > main.go <<'EOF'
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
EOF

go mod tidy >/dev/null
(cd "$repo/cmd/bowline" && go build -buildvcs=false -o "$work/bowline" .)

cat > bowline.json <<'EOF'
{
  "entry": "./api.Routes",
  "targets": { "ts": { "out": "web/src/bowline.ts" } }
}
EOF

"$work/bowline" gen
test -f bowline.contract.json
test -f web/src/bowline.ts

cd web
npm init -y >/dev/null
npm install "$repo/packages/client" typescript >/dev/null 2>&1
cat > src/main.ts <<'EOF'
import { createClient } from "./bowline.js";

const client = createClient({ url: "http://localhost:8080/api" });
const greeting = await client.greet({ name: "ada" });
console.log(greeting.message);
EOF
cat > tsconfig.json <<'EOF'
{ "compilerOptions": { "strict": true, "module": "ESNext", "moduleResolution": "Bundler", "target": "ES2022", "noEmit": true, "skipLibCheck": true }, "include": ["src"] }
EOF
npx tsc -p tsconfig.json

cd ..
go build -o "$work/hello-server" .
"$work/hello-server" &
server=$!
sleep 2
out="$(curl -sf 'http://localhost:8080/api/greet?input=%7B%22name%22%3A%22ada%22%7D')"
kill $server
wait $server 2>/dev/null || true
echo "$out" | grep -q 'hello, ada'

end=$(date +%s)
elapsed=$((end - start))
echo "quickstart completed in ${elapsed}s"
if [ "$elapsed" -gt 300 ]; then
  echo "quickstart exceeded five minutes" >&2
  exit 1
fi
