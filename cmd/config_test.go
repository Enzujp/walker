package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractOptionsConfigAndOverrides(t *testing.T) {
	config := writeFixture(t, "config.json", `{"name":"Configured","base_url":"https://example.com","auth":{"type":"basic","username":"{{user}}","password":"{{password}}"},"variables":{"user":"Ada"},"group_by_path":true}`)
	manifest := writeFixture(t, "routes.json", `[{"method":"GET","path":"/users/{id}","path_params":{"id":"42"}}]`)
	c := NewCommand()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"extract", "-i", manifest, "--config", config, "--name", "Override", "--base-url", "https://override.example", "--bearer-token", "{{token}}", "--variable", "token=test=value", "--group-by-path=false"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Info     struct{ Name string }
		Auth     struct{ Type string }
		Item     []struct{ Request any }
		Variable []struct{ Key, Value string }
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Info.Name != "Override" || got.Auth.Type != "bearer" || got.Item[0].Request == nil {
		t.Fatalf("%s", &out)
	}
	values := map[string]string{}
	for _, v := range got.Variable {
		values[v.Key] = v.Value
	}
	if values["token"] != "test=value" || values["user"] != "Ada" || values["base_url"] != "https://override.example" {
		t.Fatal(values)
	}
	// With no explicit override, config values survive CLI defaults.
	c = NewCommand()
	out.Reset()
	c.SetOut(&out)
	c.SetArgs([]string{"extract", "-i", manifest, "--config", config})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"name": "Configured"`) || !strings.Contains(out.String(), `"type": "basic"`) {
		t.Fatal(out.String())
	}
}

func TestExtractConfigAndMetadataErrors(t *testing.T) {
	for _, contents := range []string{`null`, `[]`, `{} {}`, `{"unknown":1}`, `{"auth":{"type":"bearer","typo":"x"}}`, `{"auth":{"type":"nope"}}`} {
		path := writeFixture(t, "config.json", contents)
		c := NewCommand()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetArgs([]string{"extract", "--demo", "--config", path})
		if err := c.Execute(); err == nil || out.Len() != 0 {
			t.Fatalf("%s: %s %v", contents, &out, err)
		}
	}
	for _, args := range [][]string{
		{"extract", "--demo", "--config", "missing.json"},
		{"extract", "--demo", "--variable", "missing-equals"},
		{"extract", "--demo", "--variable", "x=1", "--variable", "x=2"},
		{"extract", "--demo", "--variable", "base_url=x"},
		{"extract", "--demo", "--bearer-token", ""},
		{"extract", "--demo", "--format", "json", "--group-by-path"},
	} {
		c := NewCommand()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetArgs(args)
		if err := c.Execute(); err == nil || out.Len() != 0 {
			t.Fatalf("%v: %s %v", args, &out, err)
		}
	}
	for _, input := range []string{
		`[{"method":"GET","path":"/","examples":[{"name":"one","typo":true}]}]`,
		`[{"method":"GET","path":"/","headers":[{"key":"Accept","typo":true}]}]`,
		`[{"method":"GET","path":"/","path_params":{"id":"42"}}]`,
	} {
		c := NewCommand()
		c.SetArgs([]string{"extract", "-i", "-"})
		c.SetIn(strings.NewReader(input))
		if err := c.Execute(); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
