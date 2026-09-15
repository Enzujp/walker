package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiffCommand(t *testing.T) {
	previous := writeFixture(t, "previous.json", `[{"method":"GET","path":"/users/{id}"}]`)
	current := writeFixture(t, "current.json", `[{"method":"POST","path":"/users"}]`)
	for _, format := range []string{"text", "json"} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", format, fail), func(t *testing.T) {
				c := NewCommand()
				var out bytes.Buffer
				c.SetArgs([]string{"diff", previous, current, "--format", format, fmt.Sprintf("--fail-on-removed=%t", fail)})
				c.SetOut(&out)
				err := c.Execute()
				if fail != errors.Is(err, ErrRoutesRemoved) {
					t.Fatalf("%v", err)
				}
				wantCode := 0
				if fail {
					wantCode = 2
				}
				if ExitCode(err) != wantCode {
					t.Fatalf("exit code %d", ExitCode(err))
				}
				if format == "json" {
					var report struct{ Added, Removed []map[string]string }
					if err := json.Unmarshal(out.Bytes(), &report); err != nil || len(report.Added) != 1 || len(report.Removed) != 1 {
						t.Fatalf("%s %v", &out, err)
					}
				} else if out.String() != "ADDED    POST /users\nREMOVED  GET /users/{id}\n" {
					t.Fatal(out.String())
				}
			})
		}
	}
}

func TestDiffStdinAndNoChanges(t *testing.T) {
	file := writeFixture(t, "routes.json", `[{"method":"GET","path":"/"}]`)
	for _, args := range [][]string{{"diff", file, "-"}, {"diff", "-", file}} {
		c := NewCommand()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetArgs(args)
		c.SetIn(strings.NewReader(`[{"method":"get","path":"/","summary":"Changed metadata"}]`))
		if err := c.Execute(); err != nil || out.String() != "No endpoint changes.\n" {
			t.Fatalf("%s %v", &out, err)
		}
	}
}

func TestDiffErrors(t *testing.T) {
	good := writeFixture(t, "good.json", `[]`)
	bad := writeFixture(t, "bad.json", `[] []`)
	for _, args := range [][]string{{"diff"}, {"diff", "-", "-"}, {"diff", good, good, "--format", "yaml"}, {"diff", bad, good}, {"diff", good, bad}, {"diff", "missing.json", good}} {
		c := NewCommand()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetArgs(args)
		err := c.Execute()
		if err == nil || ExitCode(err) != 1 || out.Len() != 0 {
			t.Fatalf("%v: %s %v", args, &out, err)
		}
	}
	c := NewCommand()
	c.SetArgs([]string{"diff", good, good})
	c.SetOut(failingWriter{})
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("%v", err)
	}
	if ExitCode(fmt.Errorf("wrapped: %w", ErrRoutesRemoved)) != 2 {
		t.Fatal("wrapped policy error not recognized")
	}
}
