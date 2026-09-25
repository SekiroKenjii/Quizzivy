//go:build integration

package wordconvert_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/platform/wordconvert"
)

func TestDockerConverterEnforcesOfflineUnprivilegedReadOnlyRuntime(t *testing.T) {
	_, root := configured(t, time.Minute)
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "args.jsonl")
	t.Setenv("WORD_TEST_DOCKER", docker)
	t.Setenv("WORD_TEST_CAPTURE", capture)
	wrapper := filepath.Join(t.TempDir(), "docker-wrapper")
	script := "#!" + python + "\nimport os,sys,json\nwith open(os.environ['WORD_TEST_CAPTURE'],'a') as f: f.write(json.dumps(sys.argv[1:])+'\\n')\nos.execv(os.environ['WORD_TEST_DOCKER'],[os.environ['WORD_TEST_DOCKER']]+sys.argv[1:])\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	converter, err := wordconvert.New(root, wrapper, os.Getenv("TEST_WORD_CONVERTER_IMAGE"), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	result, err := converter.Convert(context.Background(), bytes.NewReader(fixture(t)), "docx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = result.Close() }()
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var captured []string
		if err := json.Unmarshal(line, &captured); err != nil {
			t.Fatal(err)
		}
		if len(captured) > 0 && captured[0] == "run" {
			args = captured
			break
		}
	}
	if len(args) < 3 {
		t.Fatal("runtime invocation not captured")
	}
	cidPosition := slices.Index(args, "--cidfile")
	if cidPosition < 0 {
		t.Fatal("container identity not journalled")
	}
	args[cidPosition+1] = filepath.Join(t.TempDir(), "probe-container-id")
	namePosition := slices.Index(args, "--name")
	if namePosition < 0 {
		t.Fatal("container was not tracked")
	}
	name := args[namePosition+1]
	t.Cleanup(func() { _ = exec.Command(docker, "rm", "--force", name).Run() })
	probe := `import os,socket,pathlib
assert os.getuid()!=0
status=pathlib.Path('/proc/self/status').read_text()
assert 'NoNewPrivs:\t1' in status
assert int(next(line.split()[1] for line in status.splitlines() if line.startswith('CapEff:')),16)==0
assert len(pathlib.Path('/proc/net/route').read_text().splitlines())==1
for _,name in socket.if_nameindex():
 if name!='lo':
  assert int(pathlib.Path('/sys/class/net',name,'flags').read_text(),16)&1==0
try:
 socket.create_connection(('1.1.1.1',53),timeout=0.2)
 raise AssertionError('network reachable')
except OSError: pass
for target in ['/etc/quizzivy-write-probe','/input/source.docx']:
 try:
  with open(target,'wb') as f: f.write(b'changed')
  raise AssertionError('protected path writable')
 except OSError: pass
assert not pathlib.Path('/var/run/docker.sock').exists()
assert int(pathlib.Path('/sys/fs/cgroup/memory.max').read_text())==512*1024*1024
assert int(pathlib.Path('/sys/fs/cgroup/pids.max').read_text())==64
assert pathlib.Path('/sys/fs/cgroup/cpu.max').read_text().split()==['100000','100000']
pathlib.Path('/work/private-write-probe').write_text('allowed')
pathlib.Path('/tmp/private-write-probe').write_text('disk-backed')
assert all(line.split()[2]!='tmpfs' for line in pathlib.Path('/proc/mounts').read_text().splitlines() if line.split()[1] in ('/work','/tmp'))
print('isolation verified')`
	probeArgs := append(slices.Clone(args[:len(args)-2]), "--entrypoint", "python3", args[len(args)-2], "-c", probe)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, docker, probeArgs...).CombinedOutput()
	if err != nil || !strings.Contains(string(output), "isolation verified") {
		t.Fatalf("runtime isolation failed: %v %s", err, output)
	}
}
