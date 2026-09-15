package walker

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func decodeCollection(t *testing.T, routes []Route, options Options) collection {
	t.Helper()
	data, err := Postman(routes, options)
	if err != nil {
		t.Fatal(err)
	}
	var c collection
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPostmanMetadataInheritance(t *testing.T) {
	c := decodeCollection(t, []Route{{
		Method: "POST", Path: "/users/{id}", Summary: "Update", Description: "Route description", Group: "API/Users",
		Body: map[string]string{"name": "default"}, Headers: []Header{{Key: "Accept", Value: "application/json"}, {Key: "X-Mode", Value: "default"}},
		Query:      []QueryParam{{Key: "limit", Value: "20"}, {Key: "tag", Value: "old"}, {Key: "tag", Value: "older"}},
		PathParams: map[string]string{"id": "42"},
		Examples: []RequestVariant{
			{Name: "B", Description: "Variant description", Body: json.RawMessage("null"), Headers: []Header{{Key: "x-mode", Value: "override"}, {Key: "Content-Type", Value: "application/problem+json"}}, Query: []QueryParam{{Key: "tag", Value: "new"}, {Key: "tag", Value: "newer"}}, PathParams: map[string]string{"id": "84"}, Auth: &Auth{Type: "noauth"}},
			{Name: "A"},
		},
	}}, Options{Auth: &Auth{Type: "bearer", Token: "{{token}}"}})
	if c.Auth == nil || c.Auth.Type != "bearer" {
		t.Fatal("missing collection auth")
	}
	items := c.Items[0].Items[0].Items
	if len(items) != 2 || !strings.HasSuffix(items[0].Name, " / A") || !strings.HasSuffix(items[1].Name, " / B") {
		t.Fatalf("%+v", items)
	}
	a, b := items[0].Request, items[1].Request
	if a.Auth != nil || b.Auth == nil || b.Auth.Type != "noauth" {
		t.Fatal("incorrect auth inheritance")
	}
	if a.URL.Variables[0].Value != "42" || b.URL.Variables[0].Value != "84" {
		t.Fatal("path values leaked across requests")
	}
	if !strings.Contains(a.Body.Raw, "default") || b.Body.Raw != "null" {
		t.Fatal("incorrect body inheritance")
	}
	if b.Description != "Route description\n\nVariant description" {
		t.Fatal(b.Description)
	}
	wantHeaders := []Header{{Key: "Accept", Value: "application/json"}, {Key: "Content-Type", Value: "application/problem+json"}, {Key: "X-Mode", Value: "override"}}
	if !reflect.DeepEqual(b.Headers, wantHeaders) {
		t.Fatalf("%+v", b.Headers)
	}
	wantQuery := []QueryParam{{Key: "limit", Value: "20"}, {Key: "tag", Value: "new"}, {Key: "tag", Value: "newer"}}
	if !reflect.DeepEqual(b.URL.Query, wantQuery) {
		t.Fatalf("%+v", b.URL.Query)
	}
	if b.URL.Raw != "{{base_url}}/users/:id?limit=20&tag=new&tag=newer" {
		t.Fatal(b.URL.Raw)
	}
}

func TestPostmanQueryEncodingAndVariables(t *testing.T) {
	c := decodeCollection(t, []Route{{Method: "GET", Path: "/search", Headers: []Header{{Key: "X-ID", Value: "{{request_id}}"}}, Query: []QueryParam{
		{Key: "q", Value: "a&b / café"}, {Key: "next", Value: "{{cursor}}"}, {Key: "ignored", Value: "hidden", Disabled: true}, {Key: "q", Value: "second"},
	}}}, Options{Variables: map[string]string{"request_id": "demo"}})
	req := c.Items[0].Request
	if req.URL.Raw != "{{base_url}}/search?next={{cursor}}&q=a%26b+%2F+caf%C3%A9&q=second" {
		t.Fatal(req.URL.Raw)
	}
	if len(req.URL.Query) != 4 || !req.URL.Query[0].Disabled {
		t.Fatalf("%+v", req.URL.Query)
	}
	values := map[string]string{}
	for _, v := range c.Variables {
		values[v.Key] = v.Value
	}
	if values["request_id"] != "demo" {
		t.Fatal(values)
	}
	if _, ok := values["cursor"]; !ok {
		t.Fatal("missing variable")
	}
	if _, ok := values["base_url"]; !ok {
		t.Fatal("missing base URL")
	}
}

func TestPostmanAuthTypesAndRouteOverride(t *testing.T) {
	tests := []Auth{{Type: "bearer", Token: "{{token}}"}, {Type: "basic", Username: "{{username}}", Password: "{{password}}"}, {Type: "apikey", Key: "X-API-Key", Value: "{{key}}", In: "header"}, {Type: "apikey", Key: "api_key", Value: "{{key}}", In: "query"}, {Type: "noauth"}}
	for _, auth := range tests {
		t.Run(auth.Type+auth.In, func(t *testing.T) {
			c := decodeCollection(t, []Route{{Method: "GET", Path: "/", Auth: &auth}}, Options{Auth: &Auth{Type: "bearer", Token: "collection"}})
			got := c.Items[0].Request.Auth
			if got == nil || got.Type != auth.Type {
				t.Fatalf("%+v", got)
			}
			switch auth.Type {
			case "bearer":
				if got.Bearer[0].Value != auth.Token {
					t.Fatal(got)
				}
			case "basic":
				if got.Basic[1].Key != "password" || got.Basic[1].Value != auth.Password {
					t.Fatal(got)
				}
			case "apikey":
				if got.APIKey[2].Key != "in" || got.APIKey[2].Value != auth.In {
					t.Fatal(got)
				}
			}
		})
	}
}

func TestPostmanGroupingAndDeterminism(t *testing.T) {
	routes := []Route{{Method: "GET", Path: "/users/{id}", PathParams: map[string]string{"id": "42"}, Examples: []RequestVariant{{Name: "Z"}, {Name: "A"}}}, {Method: "GET", Path: "/health"}, {Method: "GET", Path: "/orders/{id}", PathParams: map[string]string{"id": "84"}, Group: "Business/Orders"}, {Method: "GET", Path: "/"}}
	options := Options{GroupByPath: true, Variables: map[string]string{"z": "last", "a": "first"}}
	data, err := Postman(routes, options)
	if err != nil {
		t.Fatal(err)
	}
	routes[0], routes[3] = routes[3], routes[0]
	routes[3].Examples[0], routes[3].Examples[1] = routes[3].Examples[1], routes[3].Examples[0]
	again, err := Postman(routes, options)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("nondeterministic: %v", err)
	}
	var c collection
	json.Unmarshal(data, &c)
	if len(c.Items) != 4 || c.Items[0].Name != "Business" || c.Items[1].Name != "health" || c.Items[2].Name != "users" || c.Items[3].Request == nil {
		t.Fatalf("%s", data)
	}
	if len(options.Variables) != 2 {
		t.Fatal("mutated variables")
	}
}

func TestPostmanErrorsAreContextual(t *testing.T) {
	if _, err := Postman([]Route{{Method: "POST", Path: "/", Examples: []RequestVariant{{Name: "broken", Body: make(chan int)}}}}, Options{}); err == nil || !strings.Contains(err.Error(), `POST / example "broken"`) {
		t.Fatalf("%v", err)
	}
	for _, options := range []Options{{Auth: &Auth{Type: "unknown"}}, {Variables: map[string]string{"base_url": "bad"}}, {Variables: map[string]string{"invalid name": "bad"}}, {BaseURL: "https://example.com?"}} {
		if _, err := Postman(nil, options); err == nil {
			t.Fatalf("accepted %+v", options)
		}
	}
}

func TestPostmanEmbeddedParametersAndEscaping(t *testing.T) {
	routes := []Route{
		{Method: "GET", Path: "/files/{name}.{ext}", PathParams: map[string]string{"name": "my report", "ext": "csv"}},
		{Method: "GET", Path: "/users/{id}", PathParams: map[string]string{"id": "a/b"}},
		{Method: "GET", Path: "/other/{name}.{ext}", PathParams: map[string]string{"name": "other", "ext": "json"}},
	}
	c := decodeCollection(t, routes, Options{})
	values := map[string]string{}
	for _, v := range c.Variables {
		values[v.Key] = v.Value
	}
	resolve := func(raw string) string {
		return variableReference.ReplaceAllStringFunc(raw, func(match string) string { return values[match[2:len(match)-2]] })
	}
	if got := resolve(c.Items[0].Request.URL.Raw); got != "http://localhost:8080/files/my%20report.csv" {
		t.Fatal(got)
	}
	if got := resolve(c.Items[1].Request.URL.Raw); got != "http://localhost:8080/other/other.json" {
		t.Fatal(got)
	}
	if got := c.Items[2].Request.URL.Variables[0].Value; got != "a%2Fb" {
		t.Fatal(got)
	}
}

func TestPostmanAuthConflicts(t *testing.T) {
	for _, tt := range []struct {
		auth    Auth
		headers []Header
		query   []QueryParam
	}{
		{Auth{Type: "bearer", Token: "{{token}}"}, []Header{{Key: "authorization", Value: "manual"}}, nil},
		{Auth{Type: "basic", Username: "user"}, []Header{{Key: "Authorization", Value: "manual"}}, nil},
		{Auth{Type: "apikey", Key: "X-Key", Value: "key", In: "header"}, []Header{{Key: "x-key", Value: "manual"}}, nil},
		{Auth{Type: "apikey", Key: "api_key", Value: "key", In: "query"}, nil, []QueryParam{{Key: "api_key", Value: "manual"}}},
	} {
		route := Route{Method: "GET", Path: "/", Headers: tt.headers, Query: tt.query}
		if _, err := Postman([]Route{route}, Options{Auth: &tt.auth}); err == nil || !strings.Contains(err.Error(), "conflicts") {
			t.Fatalf("%+v %v", tt, err)
		}
		route.Auth = &Auth{Type: "noauth"}
		if _, err := Postman([]Route{route}, Options{Auth: &tt.auth}); err != nil {
			t.Fatal(err)
		}
	}
}
