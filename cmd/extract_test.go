package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestExtractCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		input     string
		wantError bool
	}{
		{"demo", []string{"extract", "--demo"}, "", false},
		{"demo-json", []string{"extract", "--demo", "--format", "json"}, "", false},
		{"stdin", []string{"extract", "--input", "-"}, `[{"method":"get","path":"/"}]`, false},
		{"empty", []string{"extract", "--input", "-"}, `[]`, false},
		{"missing-source", []string{"extract"}, "", true},
		{"conflicting-source", []string{"extract", "--demo", "--input", "-"}, "", true},
		{"format", []string{"extract", "--demo", "-f", "xml"}, "", true},
		{"bad-json", []string{"extract", "-i", "-"}, `[`, true},
		{"null", []string{"extract", "-i", "-"}, `null`, true},
		{"unknown-field", []string{"extract", "-i", "-"}, `[{"method":"GET","path":"/","typo":1}]`, true},
		{"trailing-document", []string{"extract", "-i", "-"}, `[] []`, true},
		{"trailing-garbage", []string{"extract", "-i", "-"}, `[] !`, true},
		{"duplicate", []string{"extract", "-i", "-"}, `[{"method":"GET","path":"/"},{"method":"get","path":"/"}]`, true},
		{"missing-file", []string{"extract", "-i", "does-not-exist.json"}, "", true},
		{"extra-arg", []string{"extract", "--demo", "oops"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCommand()
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&out)
			c.SetIn(strings.NewReader(tt.input))
			c.SetArgs(tt.args)
			err := c.Execute()
			if (err != nil) != tt.wantError {
				t.Fatalf("err=%v output=%s", err, &out)
			}
			if !tt.wantError && !json.Valid(out.Bytes()) {
				t.Fatalf("stdout is not JSON: %s", &out)
			}
			if tt.wantError && out.Len() != 0 {
				t.Fatalf("partial output on error: %s", &out)
			}
		})
	}
}

func TestPreservesJSONNumbers(t *testing.T) {
	c := NewCommand()
	var out bytes.Buffer
	c.SetArgs([]string{"extract", "-i", "-", "-f", "json"})
	c.SetIn(strings.NewReader(`[{"method":"POST","path":"/","body":{"id":9007199254740993}}]`))
	c.SetOut(&out)
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "9007199254740993") {
		t.Fatalf("lost precision: %s", &out)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func TestOutputFailure(t *testing.T) {
	c := NewCommand()
	c.SetArgs([]string{"extract", "--demo"})
	c.SetOut(failingWriter{})
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("got %v", err)
	}
}
