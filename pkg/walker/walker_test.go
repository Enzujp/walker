package walker

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestExtractNestedAndIndependent(t *testing.T) {
	r := chi.NewRouter()
	handler := func(http.ResponseWriter, *http.Request) {}
	r.Get("/health", handler)
	r.Route("/api", func(r chi.Router) {
		r.Get("/users/{id}", handler)
		r.Post("/users", handler)
	})
	want := []Route{{Method: "POST", Path: "/api/users"}, {Method: "GET", Path: "/api/users/{id}"}, {Method: "GET", Path: "/health"}}
	for i := 0; i < 2; i++ {
		got, err := Extract(r)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, %v; want %#v", got, err, want)
		}
	}
	empty, err := Extract(chi.NewRouter())
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty: %#v %v", empty, err)
	}
	if _, err := Extract(nil); err == nil {
		t.Fatal("expected nil router error")
	}
	var typedNil *chi.Mux
	if _, err := Extract(typedNil); err == nil {
		t.Fatal("expected typed nil router error")
	}
}

func TestNormalize(t *testing.T) {
	for _, routes := range [][]Route{
		{{Method: "GET", Path: "users"}},
		{{Method: "", Path: "/"}},
		{{Method: "BAD METHOD", Path: "/"}},
		{{Method: "GET", Path: "/has space"}},
		{{Method: "GET", Path: "/has\u00a0space"}},
		{{Method: "GET", Path: "/has\x00control"}},
		{{Method: "get", Path: "/"}, {Method: "GET", Path: "/"}},
	} {
		if _, err := Normalize(routes); err == nil {
			t.Errorf("accepted %#v", routes)
		}
	}
	input := []Route{{Method: " post ", Path: "/z"}, {Method: "GET", Path: "/a"}}
	got, err := Normalize(input)
	if err != nil || got[0].Path != "/a" || got[1].Method != "POST" {
		t.Fatalf("%#v %v", got, err)
	}
	if input[0].Method != " post " {
		t.Fatal("mutated input")
	}
}

func TestJSONExample(t *testing.T) {
	type Embedded struct{ Visible string }
	type node struct {
		Next *node `json:"next"`
	}
	type payload struct {
		Embedded
		Name     string `json:"name,omitempty"`
		Hidden   string `json:"-"`
		private  string
		Count    uint16            `json:"count,string"`
		Tags     []string          `json:"tags"`
		Values   [2]int            `json:"values"`
		Metadata map[string]string `json:"metadata"`
		Created  time.Time         `json:"created"`
		Node     *node             `json:"node"`
	}
	got, err := JSONExample(&payload{Name: "do-not-copy-this-secret"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["name"] != "string" || decoded["Visible"] != "string" || decoded["count"] != "1" {
		t.Fatalf("%s", got)
	}
	for _, key := range []string{"Hidden", "private", "name,omitempty"} {
		if _, ok := decoded[key]; ok {
			t.Errorf("unexpected %s", key)
		}
	}
	if decoded["created"] != "2024-01-01T00:00:00Z" {
		t.Fatalf("bad time: %s", got)
	}
	if !bytes.Contains(got, []byte(`"next": null`)) {
		t.Fatalf("recursive example: %s", got)
	}
	for _, value := range []any{nil, (**string)(nil), []byte{}, map[int]string{}, true, 1.5} {
		if _, err := JSONExample(value); err != nil {
			t.Errorf("%T: %v", value, err)
		}
	}
	if got, err := JSONExample(nil); err != nil || string(got) != "null" {
		t.Fatalf("nil: %s %v", got, err)
	}
	if _, err := JSONExample(make(chan int)); err == nil {
		t.Fatal("expected unsupported type error")
	}
}

func TestPostman(t *testing.T) {
	routes := []Route{
		{Method: "POST", Path: "/users", Body: map[string]any{"name": "Ada"}},
		{Method: "GET", Path: "/users/{id:[0-9]{2,4}}"},
	}
	data, err := Postman(routes, Options{Name: "Example", BaseURL: "https://example.com/api/"})
	if err != nil {
		t.Fatal(err)
	}
	var c collection
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	if c.Info.Name != "Example" || !strings.Contains(c.Info.Schema, "v2.1.0") {
		t.Fatalf("%+v", c.Info)
	}
	if c.Variables[0].Value != "https://example.com/api" {
		t.Fatalf("%+v", c.Variables)
	}
	if c.Items[1].Request.URL.Raw != "{{base_url}}/users/:id" {
		t.Fatal(c.Items[1].Request.URL)
	}
	if c.Items[0].Request.Body == nil || len(c.Items[0].Request.Headers) != 1 || c.Items[1].Request.Body != nil {
		t.Fatalf("%s", data)
	}
	again, err := Postman([]Route{routes[1], routes[0]}, Options{Name: "Example", BaseURL: "https://example.com/api/"})
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("output depends on input order")
	}
	empty, err := Postman(nil, Options{})
	if err != nil || !bytes.Contains(empty, []byte(`"item": []`)) {
		t.Fatalf("%s %v", empty, err)
	}
	for _, base := range []string{"ftp://example.com", "localhost:8080", "https://example.com?x=1", "https://user:pass@example.com"} {
		if _, err := Postman(nil, Options{BaseURL: base}); err == nil {
			t.Errorf("accepted %s", base)
		}
	}
	if _, err := Postman([]Route{{Method: "POST", Path: "/", Body: make(chan int)}}, Options{}); err == nil {
		t.Fatal("body error swallowed")
	}
	for _, path := range []string{"/{", "/{}", "/stray}"} {
		if _, err := Postman([]Route{{Method: "GET", Path: path}}, Options{}); err == nil {
			t.Errorf("accepted %s", path)
		}
	}
}

func TestPostmanRegexParameters(t *testing.T) {
	for _, path := range []string{`/{id:[0-9]{2,4}}`, `/{id:[{}]}`, `/{id:\{value\}}`} {
		got, params, err := postmanPath(path)
		if err != nil || got != "/{{path_id}}" || !reflect.DeepEqual(params, []string{"path_id"}) {
			t.Errorf("%q: got %q, %v, %v", path, got, params, err)
		}
	}
}

func FuzzPostmanPath(f *testing.F) {
	for _, path := range []string{"/users/{id}", "/{id:[0-9]{2,4}}", "/{", "/", "/{}"} {
		f.Add(path)
	}
	f.Fuzz(func(t *testing.T, path string) { _, _, _ = postmanPath(path) })
}
