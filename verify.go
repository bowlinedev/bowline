package bowline

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type signature struct {
	kind, method, in, out string
}

func (r *Router) Verify(document []byte) error {
	doc, err := contract.Parse(document)
	if err != nil {
		return err
	}
	declared := map[string]signature{}
	for _, p := range doc.Procedures {
		declared[p.Path] = signature{kind: p.Kind, method: p.Method, in: p.GoInput, out: p.GoOutput}
	}
	registered := map[string]signature{}
	for _, p := range r.Procedures() {
		registered[p.Path] = signature{kind: string(p.Kind), method: p.Method(), in: contract.GoTypeName(p.In), out: contract.GoTypeName(p.Out)}
	}
	var problems []string
	for path, want := range declared {
		got, ok := registered[path]
		if !ok {
			problems = append(problems, path+": in contract but not registered")
			continue
		}
		problems = append(problems, compare(path, "kind", got.kind, want.kind)...)
		problems = append(problems, compare(path, "method", got.method, want.method)...)
		problems = append(problems, compare(path, "input", got.in, want.in)...)
		problems = append(problems, compare(path, "output", got.out, want.out)...)
	}
	for path := range registered {
		if _, ok := declared[path]; !ok {
			problems = append(problems, path+": registered but not in contract")
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return errors.New("bowline: contract drift; run bowline gen\n  " + strings.Join(problems, "\n  "))
}

func compare(path, what, got, want string) []string {
	if got == want {
		return nil
	}
	return []string{fmt.Sprintf("%s: %s is %s at runtime but %s in contract", path, what, got, want)}
}
