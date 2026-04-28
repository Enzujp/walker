package generator

import (
	"encoding/json"
	"fmt"
	"github.com/enzujp/walker/internal"
)

type PostmanCollector struct {
	Info Info   `json:"info"`
	Item []Item `json:"item"`
}

type Info struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

type Item struct {
	Name    string  `json:"name"`
	Request Request `json:"request"`
}

type Request struct {
	Method string `json:"method"`
	Header []any  `json:"header"`
	Body   *Body  `json:"body,omitempty"`
	URL    string `json:"url"`
}

type Body struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

func GeneratePostmanCollection(routes []internal.RouteMeta) ([]byte, error) {
	var items []Item

	for _, route := range routes {
		item := Item{
			Name: fmt.Sprintf("%s %s", route.Method, route.Path),
			Request: Request{
				Method: route.Method,
				URL:    "{{base_url}}" + route.Path,
				Header: []any{},
			},
		}

		if route.Req != nil {
			bodyBytes, _ := GenerateJSONExample(route.Req)
			item.Request.Body = &Body{
				Mode: "raw",
				Raw:  string(bodyBytes),
			}
		}

		items = append(items, item)
	}

	// Initiate Collection
	collection := PostmanCollector{
		Info: Info{
			Name:   "Auto Generated API",
			Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		Item: items,
	}

	return json.MarshalIndent(collection, "", " ")
}
