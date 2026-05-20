package main

import (
	"reflect"
	"testing"

	"github.com/startvibecoding/vibecoding/internal/acp"
)

func TestRootPrintAcceptsMessageArgument(t *testing.T) {
	var gotArgs []string
	var gotOpts runOptions

	cmd := newRootCommand(
		func(args []string, opts runOptions) error {
			gotArgs = args
			gotOpts = opts
			return nil
		},
		func(acp.RunOptions) error {
			t.Fatal("unexpected ACP command execution")
			return nil
		},
	)
	cmd.SetArgs([]string{"-P", "review"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if !gotOpts.print {
		t.Fatal("expected print mode to be enabled")
	}
	if want := []string{"review"}; !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args = %#v, want %#v", gotArgs, want)
	}
}

func TestRootStillDispatchesACPSubcommand(t *testing.T) {
	var calledACP bool

	cmd := newRootCommand(
		func([]string, runOptions) error {
			t.Fatal("unexpected root command execution")
			return nil
		},
		func(opts acp.RunOptions) error {
			calledACP = true
			if opts.Model != "test-model" {
				t.Fatalf("model = %q, want test-model", opts.Model)
			}
			return nil
		},
	)
	cmd.SetArgs([]string{"acp", "-m", "test-model"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if !calledACP {
		t.Fatal("expected ACP command execution")
	}
}
