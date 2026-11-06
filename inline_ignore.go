package actionlint

import (
	"bytes"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"
)

const inlineIgnoreMarker = "actionlint-ignore:"

var reInlineIgnore = regexp.MustCompile(`(^|\s)#\s*actionlint-ignore:([^#]*)`)

// inlineIgnores maps a line number to the rule names ignored on that line.
type inlineIgnores map[int][]string

func (ig inlineIgnores) match(err *Error) bool {
	for _, k := range ig[err.Line] {
		if k == err.Kind {
			return true
		}
	}
	return false
}

func (ig inlineIgnores) add(line int, kinds []string) {
	ig[line] = append(ig[line], kinds...)
}

type inlineIgnoreCollector struct {
	// Indentation of the parent of each block scalar, keyed by the line of its '|' or '>' indicator
	blocks   map[int]int
	comments []string
}

func (c *inlineIgnoreCollector) visit(n *yaml.Node, parentIndent int) {
	for _, s := range []string{n.HeadComment, n.LineComment, n.FootComment} {
		if s != "" {
			c.comments = append(c.comments, s)
		}
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			c.blocks[n.Line] = parentIndent
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			c.visit(k, k.Column-1)
			c.visit(v, k.Column-1)
		}
	case yaml.SequenceNode:
		for _, e := range n.Content {
			c.visit(e, n.Column-1)
		}
	default:
		for _, e := range n.Content {
			c.visit(e, parentIndent)
		}
	}
}

func indentOf(l string) int {
	return len(l) - len(strings.TrimLeft(l, " \t"))
}

func isBlank(l string) bool {
	return strings.TrimSpace(l) == ""
}

func isCommentLine(l string) bool {
	return strings.HasPrefix(strings.TrimSpace(l), "#")
}

// parseInlineIgnores collects `# actionlint-ignore: rule1, rule2` comments. A comment at the end of
// a line applies to that line. A comment on its own line applies to the next non-comment line. When
// the target line starts a block scalar, the whole block is covered. Lines inside block scalars are
// script content, not YAML comments, so directives there are ignored.
func parseInlineIgnores(src []byte, root *yaml.Node) inlineIgnores {
	if root == nil || !bytes.Contains(src, []byte(inlineIgnoreMarker)) {
		return nil
	}

	c := &inlineIgnoreCollector{blocks: map[int]int{}}
	c.visit(root, -1)

	lines := strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}

	// Resolve each block scalar to its last content line and mark its content lines
	inBlock := make([]bool, len(lines)+2)
	blockEnd := map[int]int{}
	for start, parent := range c.blocks {
		end := start
		for l := start + 1; l <= len(lines); l++ {
			text := lines[l-1]
			if isBlank(text) {
				continue
			}
			if indentOf(text) <= parent {
				break
			}
			inBlock[l] = true
			end = l
		}
		blockEnd[start] = end
	}

	ig := inlineIgnores{}
	for i, text := range lines {
		line := i + 1
		if inBlock[line] || !strings.Contains(text, inlineIgnoreMarker) {
			continue
		}
		m := reInlineIgnore.FindStringSubmatch(text)
		if m == nil || !c.isComment(m[0]) {
			continue
		}
		kinds := strings.FieldsFunc(m[2], func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
		if len(kinds) == 0 {
			continue
		}

		target := line
		if isCommentLine(text) {
			target++
			for target <= len(lines) && isCommentLine(lines[target-1]) {
				target++
			}
			if target > len(lines) {
				continue
			}
		}

		end := target
		if e, ok := blockEnd[target]; ok {
			end = e
		}
		for l := target; l <= end; l++ {
			ig.add(l, kinds)
		}
	}
	return ig
}

// isComment checks the directive was really parsed as a YAML comment, not as a part of a quoted string.
func (c *inlineIgnoreCollector) isComment(directive string) bool {
	d := strings.TrimSpace(directive)
	for _, s := range c.comments {
		if strings.Contains(s, d) {
			return true
		}
	}
	return false
}
