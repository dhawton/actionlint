package actionlint

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkParseWorkflow(b *testing.B) {
	type bench struct {
		name  string
		input []byte
	}

	loadBench := func(name string) bench {
		i, err := os.ReadFile(filepath.Join("testdata", "bench", name+".yaml"))
		if err != nil {
			b.Fatal(err)
		}
		return bench{name, i}
	}

	for _, bc := range []bench{
		loadBench("minimal"),
		loadBench("small"),
		loadBench("large"),
	} {
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, errs := Parse(bc.input); len(errs) > 0 {
					b.Fatal(errs)
				}
			}
		})
	}
}

func BenchmarkParseTestData(b *testing.B) {
	type bench struct {
		name   string
		inputs [][]byte
	}

	loadBench := func(name string) bench {
		inputs := [][]byte{}
		_, fs, err := testFindAllWorkflowsInDir(name)
		if err != nil {
			b.Fatal(err)
		}
		for _, f := range fs {
			bs, err := os.ReadFile(f)
			if err != nil {
				b.Fatal(err)
			}
			inputs = append(inputs, bs)
		}
		return bench{name, inputs}
	}

	for _, bc := range []bench{
		loadBench("examples"),
		loadBench("ok"),
		loadBench("err"),
	} {
		b.Run(bc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, in := range bc.inputs {
					// Note: Some workflows may cause parse error
					Parse(in)
				}
			}
		})
	}
}

func TestParseParallelSteps(t *testing.T) {
	src := `on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - id: a
        run: echo a
        background: true
      - wait: [a, b]
      - wait-all:
      - cancel: a
      - parallel:
          - run: echo b
          - uses: actions/checkout@v5
`
	w, errs := Parse([]byte(src))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	steps := w.Jobs["test"].Steps
	if len(steps) != 5 {
		t.Fatalf("wanted 5 steps but got %d", len(steps))
	}

	if b := steps[0].Background; b == nil || !b.Value {
		t.Errorf("background is not parsed: %#v", b)
	}

	wait, ok := steps[1].Exec.(*ExecWait)
	if !ok || wait.WaitAll || len(wait.Wait) != 2 || wait.Wait[0].Value != "a" || wait.Wait[1].Value != "b" {
		t.Errorf("wait step is not parsed correctly: %#v", steps[1].Exec)
	}

	waitAll, ok := steps[2].Exec.(*ExecWait)
	if !ok || !waitAll.WaitAll || waitAll.Wait != nil {
		t.Errorf("wait-all step is not parsed correctly: %#v", steps[2].Exec)
	}

	cancel, ok := steps[3].Exec.(*ExecCancel)
	if !ok || cancel.Cancel == nil || cancel.Cancel.Value != "a" {
		t.Errorf("cancel step is not parsed correctly: %#v", steps[3].Exec)
	}

	par, ok := steps[4].Exec.(*ExecParallel)
	if !ok || len(par.Steps) != 2 {
		t.Fatalf("parallel step is not parsed correctly: %#v", steps[4].Exec)
	}
	if _, ok := par.Steps[0].Exec.(*ExecRun); !ok {
		t.Errorf("first step in parallel should run a script: %#v", par.Steps[0].Exec)
	}
	if _, ok := par.Steps[1].Exec.(*ExecAction); !ok {
		t.Errorf("second step in parallel should run an action: %#v", par.Steps[1].Exec)
	}
}

func TestParseContainerPortsAndVolumes(t *testing.T) {
	src := `on: push
jobs:
  test:
    runs-on: ubuntu-latest
    container:
      image: node:18
      ports: [8080]
      volumes: [/foo:/bar]
    services:
      redis:
        image: redis
        volumes: [/redis:/data]
        ports: [6379]
    steps:
      - run: echo
`
	w, errs := Parse([]byte(src))
	if len(errs) > 0 {
		t.Fatal(errs)
	}

	j := w.Jobs["test"]
	for name, tc := range map[string]struct {
		c            *Container
		port, volume string
	}{
		"container": {j.Container, "8080", "/foo:/bar"},
		"service":   {j.Services.Value["redis"].Container, "6379", "/redis:/data"},
	} {
		if len(tc.c.Ports) != 1 || tc.c.Ports[0].Value != tc.port {
			t.Errorf("%s: unexpected ports: %v", name, tc.c.Ports)
		}
		if len(tc.c.Volumes) != 1 || tc.c.Volumes[0].Value != tc.volume {
			t.Errorf("%s: unexpected volumes: %v", name, tc.c.Volumes)
		}
	}
}
