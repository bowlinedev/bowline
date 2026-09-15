package analyzer

import (
	"go/types"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/internal/corpus"
)

func TestGoTypeNameMatchesReflect(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := Load(root, testEnv(), "github.com/bowlinedev/bowline/internal/corpus")
	if err != nil {
		t.Fatal(err)
	}
	pkg := prog.Package("github.com/bowlinedev/bowline/internal/corpus")
	st := pkg.Types.Scope().Lookup("Instances").Type().Underlying().(*types.Struct)
	rt := reflect.TypeOf(corpus.Instances)
	for i := 0; i < st.NumFields(); i++ {
		want := contract.GoTypeName(rt.Field(i).Type)
		got := goTypeName(st.Field(i).Type())
		if got != want {
			t.Errorf("field %s: go/types %q, reflect %q", st.Field(i).Name(), got, want)
		}
	}
}
