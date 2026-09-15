package walker

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDocsAttachesMetadataToActualRoutes(t *testing.T) {
	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) { r.Get("/users/{id}", func(http.ResponseWriter, *http.Request) {}) })
	r.Get("/undocumented", func(http.ResponseWriter, *http.Request) {})
	var docs Docs
	docs.Describe(" get ", "/api/users/{id}", Summary("Read user"), Description("Details"), Group("API/Users"),
		PathParam("id", "42"), Query(QueryParam{Key: "expand", Value: "profile"}), Headers(Header{Key: "Accept", Value: "application/json"}),
		Authentication(Auth{Type: "bearer", Token: "{{token}}"}), RequestExample(map[string]any{"id": 42}),
		Examples(RequestVariant{Name: "default"}),
	)
	routes, err := docs.Extract(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes[0].Summary != "Read user" || routes[0].PathParams["id"] != "42" || routes[1].Summary != "" {
		t.Fatalf("%+v", routes)
	}
	// Repeated extraction must not share mutable metadata containers.
	routes[0].Headers[0].Value = "mutated"
	routes[0].PathParams["id"] = "mutated"
	routes[0].Auth.Token = "mutated"
	routes[0].Examples[0].Name = "mutated"
	again, err := docs.Extract(r)
	if err != nil || again[0].Headers[0].Value != "application/json" || again[0].PathParams["id"] != "42" || again[0].Auth.Token != "{{token}}" || again[0].Examples[0].Name != "default" {
		t.Fatalf("%+v %v", again, err)
	}
	var independent Docs
	empty, err := independent.Extract(chi.NewRouter())
	if err != nil || len(empty) != 0 {
		t.Fatalf("shared state: %+v %v", empty, err)
	}
}

func TestDocsRejectsStaleAndDuplicateDeclarations(t *testing.T) {
	r := chi.NewRouter()
	var docs Docs
	docs.Describe("POST", "/z", Summary("Missing"))
	docs.Describe("GET", "/a", Summary("Missing"))
	if _, err := docs.Extract(r); err == nil || !strings.Contains(err.Error(), "GET /a, POST /z") {
		t.Fatalf("%v", err)
	}
	for _, setup := range []func(*Docs){
		func(d *Docs) { d.Describe("GET", "/", nil) },
		func(d *Docs) { d.Describe("GET", "/", Authentication(Auth{Type: "wrong"})) },
		func(d *Docs) { d.Describe("GET", "/"); d.Describe("get", "/") },
	} {
		var d Docs
		setup(&d)
		d.Describe("GET", "/ignored") // Must not erase a previous setup error.
		if _, err := d.Extract(r); err == nil {
			t.Fatal("expected declaration error")
		}
	}
}

func TestMetadataValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Route)
		message string
	}{
		{"unknown-path", func(r *Route) { r.PathParams = map[string]string{"typo": "1"} }, "does not exist"},
		{"header-injection", func(r *Route) { r.Headers = []Header{{Key: "X-ID", Value: "ok\r\nInjected: 1"}} }, "control"},
		{"header-name", func(r *Route) { r.Headers = []Header{{Key: "Bad Name"}} }, "header name"},
		{"duplicate-headers", func(r *Route) { r.Headers = []Header{{Key: "Accept"}, {Key: "accept"}} }, "duplicate header"},
		{"empty-query", func(r *Route) { r.Query = []QueryParam{{Value: "1"}} }, "query"},
		{"query-control", func(r *Route) { r.Query = []QueryParam{{Key: "q", Value: "\x00"}} }, "query"},
		{"group-empty-part", func(r *Route) { r.Group = "API//Users" }, "group"},
		{"group-parent", func(r *Route) { r.Group = "API/../Users" }, "group"},
		{"group-whitespace", func(r *Route) { r.Group = " Users" }, "group"},
		{"summary-linebreak", func(r *Route) { r.Summary = "foo\nbar" }, "summary"},
		{"empty-example", func(r *Route) { r.Examples = []RequestVariant{{Name: " "}} }, "example name"},
		{"duplicate-example", func(r *Route) { r.Examples = []RequestVariant{{Name: "same"}, {Name: "same"}} }, "duplicate example"},
		{"example-path", func(r *Route) {
			r.Examples = []RequestVariant{{Name: "bad", PathParams: map[string]string{"wrong": "42"}}}
		}, `example "bad"`},
		{"path-control", func(r *Route) { r.PathParams = map[string]string{"id": "\n"} }, "control"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Route{Method: "GET", Path: "/users/{id}"}
			tt.mutate(&r)
			if _, err := Normalize([]Route{r}); err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestAuthValidation(t *testing.T) {
	for _, auth := range []*Auth{nil, {Type: "noauth"}, {Type: "bearer", Token: "{{token}}"}, {Type: "basic", Username: "{{username}}"}, {Type: "apikey", Key: "X-Key", Value: "{{key}}", In: "header"}, {Type: "apikey", Key: "key", Value: "{{key}}", In: "query"}} {
		if err := validateAuth(auth); err != nil {
			t.Errorf("%+v: %v", auth, err)
		}
	}
	for _, auth := range []*Auth{{}, {Type: "oauth2"}, {Type: "bearer"}, {Type: "basic"}, {Type: "apikey", Key: "key", Value: "value", In: "cookie"}, {Type: "bearer", Token: "x", Password: "wrong"}, {Type: "noauth", Token: "x"}, {Type: "bearer", Token: "x\n"}, {Type: "apikey", Key: "Invalid Header", Value: "x", In: "header"}} {
		if err := validateAuth(auth); err == nil {
			t.Errorf("accepted %+v", auth)
		}
	}
}

func TestNormalizeClonesNestedMetadata(t *testing.T) {
	original := Route{Method: "GET", Path: "/{id}", Examples: []RequestVariant{{Name: "one", Headers: []Header{{Key: "Accept", Value: "x"}}, Query: []QueryParam{{Key: "q", Value: "1"}}, PathParams: map[string]string{"id": "42"}, Auth: &Auth{Type: "bearer", Token: "x"}}}}
	data, _ := json.Marshal(original)
	routes, err := Normalize([]Route{original})
	if err != nil {
		t.Fatal(err)
	}
	e := &routes[0].Examples[0]
	e.Headers[0].Value = "y"
	e.Query[0].Value = "2"
	e.PathParams["id"] = "43"
	e.Auth.Token = "y"
	after, _ := json.Marshal(original)
	if !reflect.DeepEqual(data, after) {
		t.Fatal("mutated caller metadata")
	}
}

func TestExplicitNullBodyRoundTrip(t *testing.T) {
	const manifest = `[{"method":"POST","path":"/","body":{"id":9007199254740993},"examples":[{"name":"inherit"},{"name":"null","body":null}]}]`
	var routes []Route
	if err := json.Unmarshal([]byte(manifest), &routes); err != nil {
		t.Fatal(err)
	}
	if routes[0].Examples[0].Body != nil {
		t.Fatal("omitted body must inherit")
	}
	if routes[0].Examples[1].Body == nil {
		t.Fatal("explicit null must override")
	}
	c := decodeCollection(t, routes, Options{})
	if !strings.Contains(c.Items[0].Request.Body.Raw, "9007199254740993") || c.Items[1].Request.Body.Raw != "null" {
		t.Fatalf("%+v", c.Items)
	}
	encoded, err := json.Marshal(routes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"body":null`) {
		t.Fatal(string(encoded))
	}
	var roundTrip []Route
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip[0].Examples[1].Body == nil {
		t.Fatal("null lost in roundtrip")
	}
	for _, input := range []string{`[{"method":"GET","path":"/","unknown":true}]`, `[{"method":"GET","path":"/","examples":[{"name":"x","unknown":true}]}]`} {
		if err := json.Unmarshal([]byte(input), &routes); err == nil {
			t.Fatal("unknown field accepted")
		}
	}
	var route Route
	if err := json.Unmarshal([]byte(`{"Method":"POST","Path":"/","Body":null}`), &route); err != nil || route.Body == nil {
		t.Fatalf("%+v %v", route, err)
	}
}
