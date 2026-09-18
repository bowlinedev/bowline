package bowline

import (
	"net/http"
	"slices"
	"testing"
)

func securedRouter() *Router {
	admin := NewRouter(
		Query("stats", getUser),
		Query("audit", getUser, Requires("adminKey")),
	)
	return NewRouter(
		Query("health", getUser, Public()),
		Query("get", getUser),
		Mount("admin", admin),
	).Scheme("bearer", BearerAuth("JWT")).Secure("bearer")
}

func securityOf(t *testing.T, r *Router, path string) []string {
	t.Helper()
	for _, p := range r.Procedures() {
		if p.Path == path {
			return p.Security
		}
	}
	t.Fatalf("no procedure %q", path)
	return nil
}

func TestSecurityIsInheritedFromTheRouter(t *testing.T) {
	r := securedRouter()
	if got := securityOf(t, r, "get"); !slices.Equal(got, []string{"bearer"}) {
		t.Fatalf("get security %v, want [bearer]", got)
	}
	if got := securityOf(t, r, "admin.stats"); !slices.Equal(got, []string{"bearer"}) {
		t.Fatalf("a mounted procedure must inherit the parent scheme, got %v", got)
	}
}

func TestPublicOptsOutOfSecurity(t *testing.T) {
	if got := securityOf(t, securedRouter(), "health"); len(got) != 0 {
		t.Fatalf("health security %v, want none", got)
	}
}

func TestRequiresAddsToTheInheritedScheme(t *testing.T) {
	got := securityOf(t, securedRouter(), "admin.audit")
	if !slices.Equal(got, []string{"adminKey", "bearer"}) {
		t.Fatalf("security %v, want [adminKey bearer]", got)
	}
}

func TestSchemesAreCollectedFromEveryRouter(t *testing.T) {
	child := NewRouter(Query("x", getUser)).Scheme("adminKey", APIKeyAuth("header", "X-Admin-Key"))
	r := NewRouter(Query("get", getUser), Mount("admin", child)).Scheme("bearer", BearerAuth("JWT")).Secure("bearer")
	schemes := r.Schemes()
	if len(schemes) != 2 {
		t.Fatalf("schemes %v, want bearer and adminKey", schemes)
	}
	if schemes["bearer"].Scheme != "bearer" || schemes["adminKey"].Header != "X-Admin-Key" {
		t.Fatalf("schemes %+v", schemes)
	}
}

func TestSecurityDeclarationDoesNotEnforce(t *testing.T) {
	h := NewRouter(Query("get", getUser)).Scheme("bearer", BearerAuth("JWT")).Secure("bearer").Handler(Logger(discardLogger()))
	rec := do(h, http.MethodPost, "/api/get", `{"id":1}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d; declaring a scheme describes the API, it must not reject a call. Enforcement belongs in middleware", rec.Code)
	}
}

func TestInvalidSchemesAreRejected(t *testing.T) {
	cases := map[string]SecurityScheme{
		"bad http scheme": {Kind: SecurityHTTP, Scheme: "digest"},
		"apiKey nowhere":  {Kind: SecurityAPIKey, In: "body", Header: "X-Key"},
		"apiKey unnamed":  {Kind: SecurityAPIKey, In: "header"},
		"unknown kind":    {Kind: "magic"},
	}
	for name, scheme := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("an invalid scheme was accepted")
				}
			}()
			NewRouter(Query("get", getUser)).Scheme("s", scheme)
		})
	}
}

func TestASchemeNameCannotMeanTwoThings(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("the same name was accepted for two different schemes")
		}
	}()
	NewRouter(Query("get", getUser)).
		Scheme("auth", BearerAuth("JWT")).
		Scheme("auth", APIKeyAuth("header", "X-Key"))
}
