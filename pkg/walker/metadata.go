package walker

import (
	"fmt"
	"maps"
	"net/textproto"
	"slices"
	"strings"
	"unicode"
)

// Header is an explicit request header. Header names are case-insensitive.
type Header struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// QueryParam is a query parameter. Repeated keys retain their declared order.
// Disabled parameters remain visible in Postman but are not sent.
type QueryParam struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
}

// Auth describes bearer, basic, apikey, or noauth authentication.
// A nil Auth inherits collection authentication. Use noauth to opt out explicitly.
// Credentials are exported verbatim: prefer variables such as {{token}}.
type Auth struct {
	Type     string `json:"type"`
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Key      string `json:"key,omitempty"`
	Value    string `json:"value,omitempty"`
	In       string `json:"in,omitempty"`
}

// RequestVariant is a named request, not a saved Postman response.
// Non-nil Body overrides the route body (use json.RawMessage("null") for JSON null).
// Headers and query parameters merge by key; path parameters merge by name.
// An example replaces all route query values for any key it supplies.
// Auth inherits from the route unless explicitly set.
type RequestVariant struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Body        any               `json:"body,omitempty"`
	Headers     []Header          `json:"headers,omitempty"`
	Query       []QueryParam      `json:"query,omitempty"`
	PathParams  map[string]string `json:"path_params,omitempty"`
	Auth        *Auth             `json:"auth,omitempty"`
}

func validateAuth(a *Auth) error {
	if a == nil {
		return nil
	}
	extra := *a
	extra.Type = ""
	switch a.Type {
	case "noauth":
	case "bearer":
		if a.Token == "" {
			return fmt.Errorf("bearer auth requires token")
		}
		extra.Token = ""
	case "basic":
		if a.Username == "" {
			return fmt.Errorf("basic auth requires username")
		}
		extra.Username, extra.Password = "", ""
	case "apikey":
		if a.Key == "" || a.Value == "" || (a.In != "header" && a.In != "query") {
			return fmt.Errorf("apikey auth requires key, value, and in (header or query)")
		}
		if a.In == "header" {
			if err := validateHeaders([]Header{{Key: a.Key, Value: a.Value}}); err != nil {
				return err
			}
		}
		extra.Key, extra.Value, extra.In = "", "", ""
	default:
		return fmt.Errorf("unsupported auth type %q: use bearer, basic, apikey, or noauth", a.Type)
	}
	if extra != (Auth{}) {
		return fmt.Errorf("auth contains fields incompatible with type %q", a.Type)
	}
	for _, value := range []string{a.Token, a.Username, a.Password, a.Key, a.Value} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("auth values must not contain line breaks")
		}
	}
	return nil
}

func isToken(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", r))
	}) < 0
}

func validateHeaders(headers []Header) error {
	seen := map[string]bool{}
	for _, h := range headers {
		if !isToken(h.Key) {
			return fmt.Errorf("invalid header name %q", h.Key)
		}
		if strings.IndexFunc(h.Value, func(r rune) bool { return r == 127 || (r < 32 && r != '\t') }) >= 0 {
			return fmt.Errorf("header %q contains a control character", h.Key)
		}
		key := strings.ToLower(h.Key)
		if seen[key] {
			return fmt.Errorf("duplicate header %q", h.Key)
		}
		seen[key] = true
	}
	return nil
}

func validateMetadata(r Route) error {
	if strings.ContainsAny(r.Summary, "\r\n") {
		return fmt.Errorf("summary must be one line")
	}
	if r.Group != "" {
		for _, part := range strings.Split(r.Group, "/") {
			if strings.TrimSpace(part) != part || part == "" || part == "." || part == ".." || strings.IndexFunc(part, unicode.IsControl) >= 0 {
				return fmt.Errorf("group must contain nonempty slash-separated folder names")
			}
		}
	}
	_, params, err := postmanPath(r.Path)
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for _, key := range params {
		names[strings.TrimPrefix(key, "path_")] = true
	}
	check := func(headers []Header, query []QueryParam, values map[string]string, auth *Auth) error {
		if err := validateHeaders(headers); err != nil {
			return err
		}
		for _, q := range query {
			if q.Key == "" || strings.IndexFunc(q.Key, unicode.IsControl) >= 0 || strings.IndexFunc(q.Value, unicode.IsControl) >= 0 {
				return fmt.Errorf("query parameters require a key and must not contain control characters")
			}
		}
		for _, name := range sortedKeys(values) {
			if !names[name] {
				return fmt.Errorf("path parameter %q does not exist in route", name)
			}
			if strings.IndexFunc(values[name], unicode.IsControl) >= 0 {
				return fmt.Errorf("path parameter %q contains a control character", name)
			}
		}
		return validateAuth(auth)
	}
	if err := check(r.Headers, r.Query, r.PathParams, r.Auth); err != nil {
		return err
	}
	examples := map[string]bool{}
	for _, e := range r.Examples {
		if strings.TrimSpace(e.Name) == "" || strings.IndexFunc(e.Name, unicode.IsControl) >= 0 {
			return fmt.Errorf("example name must be nonempty and contain no control characters")
		}
		if examples[e.Name] {
			return fmt.Errorf("duplicate example name %q", e.Name)
		}
		examples[e.Name] = true
		if err := check(e.Headers, e.Query, e.PathParams, e.Auth); err != nil {
			return fmt.Errorf("example %q: %w", e.Name, err)
		}
	}
	return nil
}

func cloneAuth(a *Auth) *Auth {
	if a == nil {
		return nil
	}
	copy := *a
	return &copy
}
func cloneRoute(r Route) Route {
	r.Headers = slices.Clone(r.Headers)
	r.Query = slices.Clone(r.Query)
	r.PathParams = maps.Clone(r.PathParams)
	r.Auth = cloneAuth(r.Auth)
	r.Examples = slices.Clone(r.Examples)
	for i := range r.Examples {
		e := &r.Examples[i]
		e.Headers = slices.Clone(e.Headers)
		e.Query = slices.Clone(e.Query)
		e.PathParams = maps.Clone(e.PathParams)
		e.Auth = cloneAuth(e.Auth)
	}
	return r
}

func mergeHeaders(base, override []Header) []Header {
	byKey := map[string]Header{}
	for _, list := range [][]Header{base, override} {
		for _, h := range list {
			h.Key = textproto.CanonicalMIMEHeaderKey(h.Key)
			byKey[strings.ToLower(h.Key)] = h
		}
	}
	result := make([]Header, 0, len(byKey))
	for _, key := range sortedKeys(byKey) {
		result = append(result, byKey[key])
	}
	return result
}

func mergeQuery(base, override []QueryParam) []QueryParam {
	replaced := map[string]bool{}
	for _, q := range override {
		replaced[q.Key] = true
	}
	result := make([]QueryParam, 0, len(base)+len(override))
	for _, q := range base {
		if !replaced[q.Key] {
			result = append(result, q)
		}
	}
	result = append(result, override...)
	slices.SortStableFunc(result, func(a, b QueryParam) int { return strings.Compare(a.Key, b.Key) })
	return result
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Conflicting helper/manual credentials are ambiguous in Postman's runtime.
func validateAuthConflicts(auth *Auth, headers []Header, query []QueryParam) error {
	if auth == nil || auth.Type == "noauth" {
		return nil
	}
	headerName := ""
	switch auth.Type {
	case "basic", "bearer":
		headerName = "Authorization"
	case "apikey":
		if auth.In == "header" {
			headerName = auth.Key
		} else {
			for _, q := range query {
				if q.Key == auth.Key && !q.Disabled {
					return fmt.Errorf("query parameter %q conflicts with auth helper", q.Key)
				}
			}
		}
	}
	if headerName != "" {
		for _, h := range headers {
			if strings.EqualFold(h.Key, headerName) {
				return fmt.Errorf("header %q conflicts with auth helper; use noauth for manual credentials", h.Key)
			}
		}
	}
	return nil
}
