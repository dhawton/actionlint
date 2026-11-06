package actionlint

import (
	"strings"
	"testing"
)

func TestRuleActionRequireCommitHash(t *testing.T) {
	sha := "8e5e7e5ab8b370d6c329ec480221332ada57f0ab"
	tests := []struct {
		what string
		uses string
		err  bool
	}{
		{"full sha", "actions/checkout@" + sha, false},
		{"full sha with path", "github/codeql-action/init@" + sha, false},
		{"short sha", "actions/checkout@8e5e7e5", true},
		{"41 chars", "actions/checkout@" + sha + "0", true},
		{"upper case sha", "actions/checkout@" + strings.ToUpper(sha), true},
		{"non-hex 40 chars", "actions/checkout@" + strings.Repeat("g", 40), true},
		{"tag", "actions/checkout@v4", true},
		{"tag with path", "github/codeql-action/init@v3", true},
		{"branch", "actions/checkout@main", true},
		{"branch with slash", "actions/checkout@release/v1", true},
		{"local action", "./path/to/action", false},
		{"self repository action", "$/path/to/action", false},
		{"docker image tag", "docker://alpine:3.20", false},
		{"docker image without tag", "docker://alpine", false},
		{"docker image digest", "docker://alpine@sha256:" + strings.Repeat("a", 64), false},
		{"expression", "actions/checkout@${{ env.REF }}", false},
	}

	for _, tc := range tests {
		for _, enabled := range []bool{true, false} {
			name := tc.what
			if !enabled {
				name += " disabled"
			}
			t.Run(name, func(t *testing.T) {
				r := NewRuleAction(NewLocalActionsCache(nil, nil))
				r.SetConfig(&Config{RequireCommitHash: enabled})
				s := &Step{
					Exec: &ExecAction{
						Uses: &String{Value: tc.uses, Pos: &Pos{}},
					},
				}
				if err := r.VisitStep(s); err != nil {
					t.Fatal(err)
				}
				var errs []*Error
				for _, e := range r.Errs() {
					if strings.Contains(e.Message, "full length commit SHA") {
						errs = append(errs, e)
					}
				}
				want := tc.err && enabled
				if want && len(errs) != 1 {
					t.Fatalf("wanted one error but got %v", errs)
				}
				if !want && len(errs) > 0 {
					t.Fatalf("wanted no error but got %v", errs)
				}
			})
		}
	}
}

func TestRuleActionRequireCommitHashNoConfig(t *testing.T) {
	r := NewRuleAction(NewLocalActionsCache(nil, nil))
	s := &Step{
		Exec: &ExecAction{
			Uses: &String{Value: "actions/checkout@main", Pos: &Pos{}},
		},
	}
	if err := r.VisitStep(s); err != nil {
		t.Fatal(err)
	}
	if errs := r.Errs(); len(errs) > 0 {
		t.Fatal(errs)
	}
}
