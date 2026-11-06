package actionlint

import "strings"

// RuleParallelSteps is a rule to check background and parallel steps. 'wait' and 'cancel' must refer to
// a preceding background step, and steps in a 'parallel' group must be plain 'run' or 'uses' steps.
// https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idstepsbackground
type RuleParallelSteps struct {
	RuleBase
	// seen maps lower-cased IDs of preceding top-level steps to whether the step is a background step.
	seen map[string]bool
	// grouped holds lower-cased IDs of steps inside 'parallel' groups.
	grouped map[string]struct{}
	// inGroup holds steps inside 'parallel' groups.
	inGroup map[*Step]struct{}
}

// NewRuleParallelSteps creates a new RuleParallelSteps instance.
func NewRuleParallelSteps() *RuleParallelSteps {
	return &RuleParallelSteps{
		RuleBase: RuleBase{
			name: "parallel-steps",
			desc: "Checks step IDs referenced by \"wait\" and \"cancel\", and steps not allowed in \"parallel\" groups",
		},
	}
}

// VisitJobPre is callback when visiting Job node before visiting its children.
func (rule *RuleParallelSteps) VisitJobPre(n *Job) error {
	rule.seen = map[string]bool{}
	rule.grouped = map[string]struct{}{}
	rule.inGroup = map[*Step]struct{}{}
	return nil
}

// VisitJobPost is callback when visiting Job node after visiting its children.
func (rule *RuleParallelSteps) VisitJobPost(n *Job) error {
	rule.seen = nil
	rule.grouped = nil
	rule.inGroup = nil
	return nil
}

// VisitStep is callback when visiting Step node.
func (rule *RuleParallelSteps) VisitStep(n *Step) error {
	if p, ok := n.Exec.(*ExecParallel); ok {
		for _, s := range p.Steps {
			rule.inGroup[s] = struct{}{}
		}
	}

	if _, ok := rule.inGroup[n]; ok {
		rule.checkStepInGroup(n)
		if n.ID != nil && !n.ID.ContainsExpression() {
			rule.grouped[strings.ToLower(n.ID.Value)] = struct{}{}
		}
		return nil
	}

	switch e := n.Exec.(type) {
	case *ExecWait:
		for _, id := range e.Wait {
			rule.checkRef(id, "wait")
		}
	case *ExecCancel:
		rule.checkRef(e.Cancel, "cancel")
	}

	if n.ID != nil && !n.ID.ContainsExpression() {
		// Expression at "background" is already reported by the parser. Treat it as background to avoid cascading errors
		rule.seen[strings.ToLower(n.ID.Value)] = n.Background != nil && (n.Background.Value || n.Background.Expression != nil)
	}

	return nil
}

func (rule *RuleParallelSteps) checkStepInGroup(s *Step) {
	if s.Background != nil {
		rule.Error(s.Background.Pos, "\"background\" is not allowed for steps in \"parallel\" group. all steps in the group already run in the background")
	}
	switch e := s.Exec.(type) {
	case *ExecWait:
		k := "wait"
		if e.WaitAll {
			k = "wait-all"
		}
		rule.Errorf(s.Pos, "%q step is not allowed in \"parallel\" group", k)
	case *ExecCancel:
		rule.Error(s.Pos, "\"cancel\" step is not allowed in \"parallel\" group")
	case *ExecParallel:
		rule.Error(s.Pos, "\"parallel\" group cannot be nested in another \"parallel\" group")
	}
}

func (rule *RuleParallelSteps) checkRef(id *String, key string) {
	if id == nil || id.Value == "" || id.ContainsExpression() {
		return
	}
	k := strings.ToLower(id.Value)
	bg, ok := rule.seen[k]
	if !ok {
		if _, ok := rule.grouped[k]; ok {
			rule.Errorf(id.Pos, "step %q at %q is in \"parallel\" group. steps in \"parallel\" group cannot be referenced by \"wait\" or \"cancel\"", id.Value, key)
			return
		}
		rule.Errorf(id.Pos, "step %q at %q is not defined before this step. %q must refer to a preceding background step", id.Value, key, key)
		return
	}
	if !bg {
		rule.Errorf(id.Pos, "step %q at %q is not a background step. %q must refer to a step with \"background: true\"", id.Value, key, key)
	}
}
