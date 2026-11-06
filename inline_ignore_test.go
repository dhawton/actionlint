package actionlint

import (
	"reflect"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestParseInlineIgnores(t *testing.T) {
	tests := []struct {
		what string
		src  string
		want inlineIgnores
	}{
		{
			what: "no directive",
			src:  "on: push # comment\n",
			want: nil,
		},
		{
			what: "same line",
			src:  "on: push # actionlint-ignore: events\n",
			want: inlineIgnores{1: {"events"}},
		},
		{
			what: "previous line",
			src:  "# actionlint-ignore: events\non: push\n",
			want: inlineIgnores{2: {"events"}},
		},
		{
			what: "multiple rules with and without spaces",
			src:  "on: push # actionlint-ignore: events,expression , syntax-check\n",
			want: inlineIgnores{1: {"events", "expression", "syntax-check"}},
		},
		{
			what: "trailing comment after rule names",
			src:  "on: push # actionlint-ignore: events # reason\n",
			want: inlineIgnores{1: {"events"}},
		},
		{
			what: "no rule name",
			src:  "on: push # actionlint-ignore:\n",
			want: inlineIgnores{},
		},
		{
			what: "CRLF",
			src:  "# actionlint-ignore: events\r\non: push\r\n",
			want: inlineIgnores{2: {"events"}},
		},
		{
			what: "directive at end of file",
			src:  "on: push\n# actionlint-ignore: events\n",
			want: inlineIgnores{},
		},
		{
			what: "quoted string",
			src:  "x: '# actionlint-ignore: events'\n",
			want: inlineIgnores{},
		},
		{
			what: "block scalar in mapping",
			src:  "a:\n  b: | # actionlint-ignore: expression\n    foo\n\n    bar\n  c: d\n",
			want: inlineIgnores{2: {"expression"}, 3: {"expression"}, 4: {"expression"}, 5: {"expression"}},
		},
		{
			what: "block scalar in mapping in sequence",
			src:  "a:\n  - b: |\n      # actionlint-ignore: expression\n    c: d # actionlint-ignore: events\n",
			want: inlineIgnores{4: {"events"}},
		},
		{
			what: "block scalar as sequence item",
			src:  "a:\n# actionlint-ignore: expression\n- |\n  foo\n- b\n",
			want: inlineIgnores{3: {"expression"}, 4: {"expression"}},
		},
		{
			what: "multiple directives for the same line",
			src:  "# actionlint-ignore: events\n# actionlint-ignore: expression\non: push # actionlint-ignore: syntax-check\n",
			want: inlineIgnores{3: {"events", "expression", "syntax-check"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.what, func(t *testing.T) {
			var n yaml.Node
			if err := yaml.Unmarshal([]byte(tc.src), &n); err != nil {
				t.Fatal(err)
			}
			have := parseInlineIgnores([]byte(tc.src), &n)
			if !reflect.DeepEqual(tc.want, have) {
				t.Fatalf("wanted %v but have %v", tc.want, have)
			}
		})
	}
}

func TestParseInlineIgnoresBrokenYAML(t *testing.T) {
	if have := parseInlineIgnores([]byte("on: push # actionlint-ignore: events\n"), nil); have != nil {
		t.Fatalf("wanted nil but have %v", have)
	}
}
