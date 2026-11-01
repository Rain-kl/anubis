package main

import (
	"flag"
	"testing"

	"github.com/facebookgo/flagenv"
)

func TestPasswordHintFlag(t *testing.T) {
	for _, tt := range []struct {
		name string
		env  string
		args []string
		want string
	}{
		{name: "default"},
		{name: "environment", env: "What is two plus two?", want: "What is two plus two?"},
		{name: "command line", args: []string{"--password-hint", "Your question?"}, want: "Your question?"},
		{name: "command line overrides environment", env: "Old question?", args: []string{"--password-hint", "New question?"}, want: "New question?"},
		{name: "command line clears environment", env: "Your question?", args: []string{"--password-hint="}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PASSWORD_HINT", tt.env)
			previous := *passwordHint
			t.Cleanup(func() { *passwordHint = previous })
			registered := flag.Lookup("password-hint")
			if registered == nil {
				t.Fatal("password-hint flag is not registered")
			}
			if err := registered.Value.Set(registered.DefValue); err != nil {
				t.Fatal(err)
			}
			set := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			set.Var(registered.Value, registered.Name, registered.Usage)
			if err := flagenv.ParseSet("", set); err != nil {
				t.Fatal(err)
			}
			if err := set.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			if *passwordHint != tt.want {
				t.Errorf("password hint = %q, want %q", *passwordHint, tt.want)
			}
		})
	}
}
