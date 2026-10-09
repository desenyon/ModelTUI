package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestOfflineJSONList(t *testing.T) {
	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list", "--offline", "--cache-dir", t.TempDir(), "--format", "json", "--limit", "2"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Source    string            `json:"source"`
		Count     int               `json:"count"`
		Offerings []json.RawMessage `json:"offerings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, out.String())
	}
	if got.Source != "embedded snapshot" || got.Count != 2 || len(got.Offerings) != 2 {
		t.Fatalf("result: %+v", got)
	}
}

func TestInvalidCLIOptions(t *testing.T) {
	for _, args := range [][]string{{"list", "--format", "xml"}, {"list", "--sort", "oops"}, {"list", "--capability", "oops"}, {"list", "--limit", "-1"}, {"list", "extra"}, {"--timeout", "0s"}, {"version", "extra"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newRootCommand()
			cmd.SetArgs(args)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.ExecuteContext(context.Background()); err == nil {
				t.Fatal("accepted invalid arguments")
			}
		})
	}
}
