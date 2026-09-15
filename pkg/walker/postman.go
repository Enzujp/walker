package walker

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// Options controls collection defaults. Explicit route/example metadata wins.
type Options struct {
	Name      string            `json:"name,omitempty"`
	BaseURL   string            `json:"base_url,omitempty"`
	Auth      *Auth             `json:"auth,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	// GroupByPath groups otherwise ungrouped routes by their first static segment.
	GroupByPath bool `json:"group_by_path,omitempty"`
}

type collection struct {
	Info      collectionInfo `json:"info"`
	Items     []item         `json:"item"`
	Variables []variable     `json:"variable"`
	Auth      *postmanAuth   `json:"auth,omitempty"`
}
type collectionInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}
type variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
}
type item struct {
	Name    string   `json:"name"`
	Request *request `json:"request,omitempty"`
	Items   []item   `json:"item,omitempty"`
}
type request struct {
	Method      string       `json:"method"`
	Description string       `json:"description,omitempty"`
	Headers     []Header     `json:"header"`
	URL         postmanURL   `json:"url"`
	Body        *body        `json:"body,omitempty"`
	Auth        *postmanAuth `json:"auth,omitempty"`
}
type postmanURL struct {
	Raw       string       `json:"raw"`
	Host      []string     `json:"host"`
	Path      []string     `json:"path"`
	Query     []QueryParam `json:"query,omitempty"`
	Variables []variable   `json:"variable,omitempty"`
}
type body struct {
	Mode    string      `json:"mode"`
	Raw     string      `json:"raw"`
	Options bodyOptions `json:"options"`
}
type bodyOptions struct {
	Raw rawOptions `json:"raw"`
}
type rawOptions struct {
	Language string `json:"language"`
}
type postmanAuth struct {
	Type   string     `json:"type"`
	Bearer []variable `json:"bearer,omitempty"`
	Basic  []variable `json:"basic,omitempty"`
	APIKey []variable `json:"apikey,omitempty"`
}

var variableReference = regexp.MustCompile(`\{\{([A-Za-z_][A-Za-z0-9_.-]*)\}\}`)
var variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// Postman produces a deterministic Collection v2.1. Named examples become
// separate requests. Authentication is inherited from the collection unless
// overridden, and path values are scoped to each request's URL variables.
func Postman(routes []Route, options Options) ([]byte, error) {
	routes, err := Normalize(routes)
	if err != nil {
		return nil, err
	}
	if options.Name == "" {
		options.Name = "Walker API"
	}
	if options.BaseURL == "" {
		options.BaseURL = "http://localhost:8080"
	}
	parsed, err := url.Parse(options.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.User != nil {
		return nil, fmt.Errorf("base URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	if err := validateAuth(options.Auth); err != nil {
		return nil, fmt.Errorf("collection auth: %w", err)
	}
	values := maps.Clone(options.Variables)
	if values == nil {
		values = map[string]string{}
	}
	for _, key := range sortedKeys(values) {
		if !variableName.MatchString(key) || (key == "base_url" || strings.HasPrefix(key, "walker_path_")) {
			return nil, fmt.Errorf("invalid or reserved collection variable %q", key)
		}
	}
	values["base_url"] = strings.TrimRight(options.BaseURL, "/")
	c := collection{
		Info:  collectionInfo{Name: options.Name, Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
		Items: []item{}, Auth: makeAuth(options.Auth),
	}
	tree := folder{children: map[string]*folder{}}
	for _, route := range routes {
		group := route.Group
		if group == "" && options.GroupByPath {
			first, _, _ := strings.Cut(strings.TrimPrefix(route.Path, "/"), "/")
			if first != "" && !strings.ContainsAny(first, "{}*") {
				group = first
			}
		}
		variants := slices.Clone(route.Examples)
		if len(variants) == 0 {
			variants = []RequestVariant{{}}
		}
		slices.SortFunc(variants, func(a, b RequestVariant) int { return strings.Compare(a.Name, b.Name) })
		for _, variant := range variants {
			req, generated, err := makeRequest(route, variant, options.Auth)
			if err != nil {
				return nil, fmt.Errorf("%s %s example %q: %w", route.Method, route.Path, variant.Name, err)
			}
			maps.Copy(values, generated)
			title := route.Method + " " + route.Path
			if route.Summary != "" {
				title += " — " + route.Summary
			}
			if variant.Name != "" {
				title += " / " + variant.Name
			}
			tree.add(group, item{Name: title, Request: req})
		}
	}
	c.Items = tree.items()
	// Create empty collection variables for placeholders actually used by requests
	// or authentication. Values are supplied by the caller or later in Postman.
	encoded, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	for _, match := range variableReference.FindAllSubmatch(encoded, -1) {
		key := string(match[1])
		if _, ok := values[key]; !ok {
			values[key] = ""
		}
	}
	c.Variables = []variable{{Key: "base_url", Value: values["base_url"], Type: "string"}}
	for _, key := range sortedKeys(values) {
		if key != "base_url" {
			c.Variables = append(c.Variables, variable{Key: key, Value: values[key], Type: "string"})
		}
	}
	return json.MarshalIndent(c, "", "  ")
}

func makeRequest(route Route, variant RequestVariant, collectionAuth *Auth) (*request, map[string]string, error) {
	headers := mergeHeaders(route.Headers, variant.Headers)
	query := mergeQuery(route.Query, variant.Query)
	params := maps.Clone(route.PathParams)
	if params == nil {
		params = map[string]string{}
	}
	maps.Copy(params, variant.PathParams)
	auth := route.Auth
	if variant.Auth != nil {
		auth = variant.Auth
	}
	effectiveAuth := auth
	if effectiveAuth == nil {
		effectiveAuth = collectionAuth
	}
	if err := validateAuthConflicts(effectiveAuth, headers, query); err != nil {
		return nil, nil, err
	}
	description := route.Description
	if variant.Description != "" {
		if description != "" {
			description += "\n\n"
		}
		description += variant.Description
	}
	req := &request{Method: route.Method, Description: description, Headers: headers, Auth: makeAuth(auth)}
	path, names, err := postmanPath(route.Path)
	if err != nil {
		return nil, nil, err
	}
	generated := map[string]string{}
	seen := map[string]bool{}
	for _, key := range names {
		name := strings.TrimPrefix(key, "path_")
		if !variableName.MatchString(name) {
			return nil, nil, fmt.Errorf("unsupported path parameter name %q", name)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		value, ok := params[name]
		if !ok {
			value = "example"
		}
		marker := "{{" + key + "}}"
		// Native Postman path variables only resolve at the start of a segment.
		// Use them for whole segments; embedded Chi parameters get a unique
		// collection variable for this endpoint/variant, avoiding cross-request leaks.
		wholeSegments := !strings.Contains(name, ".")
		for _, segment := range strings.Split(path, "/") {
			if strings.Contains(segment, marker) && segment != marker {
				wholeSegments = false
			}
		}
		if wholeSegments {
			path = strings.ReplaceAll(path, marker, ":"+name)
			req.URL.Variables = append(req.URL.Variables, variable{Key: name, Value: escapePath(value), Type: "string"})
		} else {
			digest := sha256.Sum256([]byte(route.Method + "\x00" + route.Path + "\x00" + variant.Name + "\x00" + name))
			scoped := fmt.Sprintf("walker_path_%x", digest[:16])
			path = strings.ReplaceAll(path, marker, "{{"+scoped+"}}")
			generated[scoped] = escapePath(value)
		}
	}

	req.URL.Host = []string{"{{base_url}}"}
	req.URL.Path = strings.Split(strings.TrimPrefix(path, "/"), "/")
	req.URL.Raw = "{{base_url}}" + path
	req.URL.Query = query
	var pairs []string
	for _, q := range query {
		if !q.Disabled {
			pairs = append(pairs, escapeQuery(q.Key)+"="+escapeQuery(q.Value))
		}
	}
	if len(pairs) != 0 {
		req.URL.Raw += "?" + strings.Join(pairs, "&")
	}
	value := route.Body
	if variant.Body != nil {
		value = variant.Body
	}
	if value != nil {
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("body: %w", err)
		}
		req.Body = &body{Mode: "raw", Raw: string(raw), Options: bodyOptions{Raw: rawOptions{Language: "json"}}}
		hasContentType := false
		for _, h := range headers {
			if strings.EqualFold(h.Key, "Content-Type") {
				hasContentType = true
			}
		}
		if !hasContentType {
			req.Headers = mergeHeaders(headers, []Header{{Key: "Content-Type", Value: "application/json"}})
		}
	}
	return req, generated, nil
}

// Preserve Postman placeholders while escaping literal query text.
func escapeQuery(value string) string { return escapeVariables(value, url.QueryEscape) }
func escapePath(value string) string  { return escapeVariables(value, url.PathEscape) }
func escapeVariables(value string, escape func(string) string) string {
	var b strings.Builder
	last := 0
	for _, loc := range variableReference.FindAllStringIndex(value, -1) {
		b.WriteString(escape(value[last:loc[0]]))
		b.WriteString(value[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(escape(value[last:]))
	return b.String()
}

func makeAuth(a *Auth) *postmanAuth {
	if a == nil {
		return nil
	}
	attribute := func(key, value string) variable { return variable{Key: key, Value: value, Type: "string"} }
	result := &postmanAuth{Type: a.Type}
	switch a.Type {
	case "bearer":
		result.Bearer = []variable{attribute("token", a.Token)}
	case "basic":
		result.Basic = []variable{attribute("username", a.Username), attribute("password", a.Password)}
	case "apikey":
		result.APIKey = []variable{attribute("key", a.Key), attribute("value", a.Value), attribute("in", a.In)}
	}
	return result
}

type folder struct {
	requests []item
	children map[string]*folder
}

func (f *folder) add(group string, request item) {
	if group == "" {
		f.requests = append(f.requests, request)
		return
	}
	head, tail, _ := strings.Cut(group, "/")
	child := f.children[head]
	if child == nil {
		child = &folder{children: map[string]*folder{}}
		f.children[head] = child
	}
	child.add(tail, request)
}
func (f *folder) items() []item {
	result := make([]item, 0, len(f.children)+len(f.requests))
	for _, name := range sortedKeys(f.children) {
		result = append(result, item{Name: name, Items: f.children[name].items()})
	}
	return append(result, f.requests...)
}

func postmanPath(path string) (string, []string, error) {
	var out strings.Builder
	var params []string
	for i := 0; i < len(path); {
		if path[i] == '?' || path[i] == '#' {
			return "", nil, fmt.Errorf("route path must not include a query or fragment")
		}
		if path[i] == '}' {
			return "", nil, fmt.Errorf("unexpected closing route parameter")
		}
		if path[i] != '{' {
			out.WriteByte(path[i])
			i++
			continue
		}
		start := i + 1
		depth := 1
		regex, escaped, characterClass := false, false, false
		i++
		for i < len(path) && depth > 0 {
			c := path[i]
			switch {
			case escaped:
				escaped = false
			case regex && c == '\\':
				escaped = true
			case regex && characterClass:
				if c == ']' {
					characterClass = false
				}
			case regex && c == '[':
				characterClass = true
			case c == ':':
				regex = true
			case c == '{':
				depth++
			case c == '}':
				depth--
			}
			i++
		}
		if depth != 0 {
			return "", nil, fmt.Errorf("unclosed route parameter")
		}
		name, _, _ := strings.Cut(path[start:i-1], ":")
		if !variableName.MatchString(name) {
			return "", nil, fmt.Errorf("invalid route parameter")
		}
		// Prefix avoids collisions with the base_url variable.
		key := "path_" + name
		params = append(params, key)
		out.WriteString("{{" + key + "}}")
	}
	return out.String(), params, nil
}
