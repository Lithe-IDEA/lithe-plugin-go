package workbench

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/1lck/Lithe-IDEA/desktop/internal/buildprocess"
	"github.com/1lck/Lithe-IDEA/desktop/internal/core"
	"github.com/1lck/Lithe-IDEA/desktop/internal/pluginsettings"
)

// goDebugWorkspace lays out a workspace like a course folder: the root has no
// go.mod and holds several projects side by side. lab-2 is a module whose main
// package reads a file by a path relative to its own directory and whose
// cmd/tool is a second main package; scratch has no go.mod at all and spreads
// its main package over two files.
var goDebugWorkspace = map[string]string{
	"lab-1/go.mod":  "module example.com/lab1\n\ngo 1.21\n",
	"lab-1/main.go": "package main\n\nfunc main() {}\n",
	"lab-2/go.mod":  "module example.com/lab2\n\ngo 1.21\n",
	"lab-2/main.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n\n" +
		"func main() {\n\tdata, err := os.ReadFile(\"examples/greeting.txt\")\n\tif err != nil {\n\t\tpanic(err)\n\t}\n" +
		"\tname := strings.TrimSpace(string(data))\n\tfmt.Printf(\"Hello, %s!\\n\", name)\n}\n",
	"lab-2/examples/greeting.txt": "Gopher\n",
	"lab-2/cmd/tool/main.go":      "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tname := \"Tool\"\n\tfmt.Printf(\"Hello, %s!\\n\", name)\n}\n",
	"scratch/main.go":             "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tname := greeting()\n\tfmt.Printf(\"Hello, %s!\\n\", name)\n}\n",
	"scratch/greeting.go":         "package main\n\nfunc greeting() string { return \"Loose\" }\n",
}

// TestOfficialGoPluginDebugsWithConfiguredDelve uses Go 1.22.12 with Delve
// 1.27.2 on PATH and the absolute path of Delve 1.23.1 outside PATH.
func TestOfficialGoPluginDebugsWithConfiguredDelve(t *testing.T) {
	compatible := os.Getenv("LITHE_TEST_COMPATIBLE_DELVE")
	archive := os.Getenv("LITHE_TEST_GO_PLUGIN_ARCHIVE")
	if compatible == "" || archive == "" {
		t.Skip("set LITHE_TEST_GO_PLUGIN_ARCHIVE and LITHE_TEST_COMPATIBLE_DELVE")
	}
	t.Setenv("GOTOOLCHAIN", "go1.22.12")
	versionCtx, cancelVersion := context.WithTimeout(context.Background(), time.Minute)
	defer cancelVersion()
	for _, check := range []struct {
		program, want string
	}{
		{"go", "go version go1.22.12 "},
		{"dlv", "Version: 1.27.2"},
		{compatible, "Version: 1.23.1"},
	} {
		if out, err := exec.CommandContext(versionCtx, check.program, "version").CombinedOutput(); err != nil || !strings.Contains(string(out), check.want) {
			t.Fatalf("%s version = %s, %v; want %s", check.program, out, err, check.want)
		}
	}

	h := newHarness(t, &testKernel{Client: core.NewClient()})
	previousGrace := buildprocess.StopGrace()
	t.Cleanup(func() { buildprocess.SetStopGrace(previousGrace) })
	h.app.refreshStopGrace()
	awaitStateWithin(t, h, 20*time.Second, "Core's stop time", func() bool { return buildprocess.StopGrace() > 0 })
	installed := installOfficialPluginArchive(t, h, archive, "io.github.lithe-idea.go-support")
	before := packageDigest(t, installed)
	h.app.reloadPluginLanguages()
	root := t.TempDir()
	writePluginFixture(t, root, map[string]string{
		"go.mod":  "module example.com/delve\n\ngo 1.22.12\n",
		"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tname := \"Gopher\"\n\tfmt.Printf(\"Hello, %s!\\n\", name)\n}\n",
	})
	p := openHostedPanel(t, h, root)
	source := filepath.Join(root, "main.go")
	h.app.openFileAtPosition(p, source, 6, 0)
	awaitStateWithin(t, h, 30*time.Second, "main.go to open", func() bool {
		doc := p.selectedDocument(p.docs.group)
		return doc != nil && samePath(doc.path, source)
	})
	defer h.app.stopDebug(p)
	clickNamedWithoutSettling(t, h, h.app.text.Debug.Start)
	awaitStateWithin(t, h, time.Minute, "the Delve compatibility failure", func() bool {
		return strings.Contains(p.debug.failure+"\n"+strings.Join(p.debug.output, "\n"),
			"Go version go1.22.12 is too old for this version of Delve")
	})
	h.app.stopDebug(p)
	awaitStateWithin(t, h, 30*time.Second, "the failed session to stop", func() bool {
		return p.debug.activeSession() == nil && len(debugBinaries(t, root)) == 0
	})

	settings := pluginsettings.New(h.app.kernel, h.store.Dir())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snapshot, err := settings.Load(ctx, p.debug.adapters[0].Manifest, root)
	if err != nil {
		t.Fatal(err)
	}
	value, err := json.Marshal(compatible)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := settings.Save(ctx, snapshot, "global", []core.PluginSettingEdit{{ID: "delveExecutable", Value: value}}, nil)
	if err != nil || !saved.Committed {
		t.Fatalf("save global Delve setting: %+v, %v", saved, err)
	}
	debugGoProgram(t, h, p, root, goDebugProgram{source: "main.go", line: 7, local: `"Gopher"`})
	h.app.closeProject()
	awaitStateWithin(t, h, 30*time.Second, "the closed project", func() bool { return h.app.project == nil })
	h.settle()
	assertPackageUnchanged(t, installed, before)
}

// goDebugProgram is one program of goDebugWorkspace the test debugs.
type goDebugProgram struct {
	// source is the file, relative to the workspace, the breakpoint goes in.
	source string
	// line is the one-based line of the Printf call, where main's local name
	// already holds its value.
	line int
	// local is that value as Variables shows it.
	local string
}

// samePath reports whether two paths name the same file, so a path Delve
// reports through a symbolic link (macOS's /var is /private/var) still matches.
func samePath(a, b string) bool {
	resolvedA, errA := filepath.EvalSymlinks(a)
	resolvedB, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && resolvedA == resolvedB
}

// debugBinaries lists the programs Delve built in dir for a debug session.
func debugBinaries(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "__debug_bin*"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

// clickNamedWithoutSettling clicks the named control of the current frame and paints
// the result once. The harness's clickNamed waits for every background task, which
// never happens while a language server runs, so the caller awaits the click's effect.
func clickNamedWithoutSettling(t *testing.T, h *harness, name string) {
	t.Helper()
	if err := h.driver.Pump(); err != nil {
		t.Fatal(err)
	}
	point := center(h.pageHit(name).Bounds)
	if err := h.driver.Click(point.X, point.Y); err != nil {
		t.Fatal(err)
	}
	if err := h.driver.Pump(); err != nil {
		t.Fatal(err)
	}
}

// TestOfficialGoPluginDebugsEachProjectOfAWorkspaceWithDelve covers the Delve
// adapter of the Go Support archive in a workspace whose root has no go.mod:
// the Debug window starts the user's dlv dap, the host reads the address Delve
// announces and connects, and for a module's main package, a second main
// package of that module and a directory without go.mod, a breakpoint stops
// main.main with its local variable, the program runs in its own directory,
// Continue runs it to its end, the session ends by itself, and Delve removes
// the program it built. The temporary directory is under macOS's /var, a
// symbolic link, which is where go build once read the wrong directory.
func TestOfficialGoPluginDebugsEachProjectOfAWorkspaceWithDelve(t *testing.T) {
	archive := os.Getenv("LITHE_TEST_GO_PLUGIN_ARCHIVE")
	if archive == "" {
		t.Skip("set LITHE_TEST_GO_PLUGIN_ARCHIVE to a go-support-<version>-darwin.zip")
	}
	for _, command := range []string{"go", "gopls", "dlv"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Fatalf("%s is not on PATH: %v", command, err)
		}
	}
	h := newHarness(t, &testKernel{Client: core.NewClient()})
	// Lithe asks Core for the stop time when its window opens, which the harness never
	// does. Without it a stopped process is killed at once, and a killed Delve cannot
	// remove the program it built.
	previousGrace := buildprocess.StopGrace()
	t.Cleanup(func() { buildprocess.SetStopGrace(previousGrace) })
	h.app.refreshStopGrace()
	awaitStateWithin(t, h, 20*time.Second, "Core's stop time", func() bool { return buildprocess.StopGrace() > 0 })
	installed := installOfficialPluginArchive(t, h, archive, "io.github.lithe-idea.go-support")
	before := packageDigest(t, installed)
	h.app.reloadPluginLanguages()
	root := t.TempDir()
	writePluginFixture(t, root, goDebugWorkspace)
	p := openHostedPanel(t, h, root)
	if declaration := p.debug.adapters[0].Declaration; declaration.Transport != "tcpAnnounced" || declaration.LaunchKind != "systemCommand" {
		t.Fatalf("declaration = %+v, want Delve started as dlv dap and announcing its address", declaration)
	}
	for _, program := range []goDebugProgram{
		{source: "lab-2/main.go", line: 15, local: `"Gopher"`},
		{source: "lab-2/cmd/tool/main.go", line: 7, local: `"Tool"`},
		{source: "scratch/main.go", line: 7, local: `"Loose"`},
	} {
		debugGoProgram(t, h, p, root, program)
	}
	if binaries := debugBinaries(t, root); len(binaries) != 0 {
		t.Fatalf("Delve built in the workspace root: %v", binaries)
	}

	h.app.closeProject()
	awaitStateWithin(t, h, 30*time.Second, "the closed project", func() bool { return h.app.project == nil })
	h.settle()
	assertPackageUnchanged(t, installed, before)
}

// debugGoProgram debugs one program of the open workspace from a breakpoint on
// the Printf line of its source to its end.
func debugGoProgram(t *testing.T, h *harness, p *project, root string, program goDebugProgram) {
	t.Helper()
	source := filepath.Join(root, filepath.FromSlash(program.source))
	// Opening a Go file starts gopls, whose tasks run until the project closes, so
	// every step waits for its own state instead of for all background work.
	h.app.openFileAtPosition(p, source, program.line-1, 0)
	awaitStateWithin(t, h, 30*time.Second, program.source+" with the caret on the Printf line", func() bool {
		d := p.selectedDocument(p.docs.group)
		if d == nil || !samePath(d.path, source) {
			return false
		}
		line, _ := d.view.Caret()
		return line == program.line-1
	})
	clickNamedWithoutSettling(t, h, h.shared("native.viewBreakpoints"))
	awaitStateWithin(t, h, 10*time.Second, "the Breakpoints dialog", func() bool { return p.panels.breakpointsOpen })
	clickNamedWithoutSettling(t, h, h.app.text.Debug.ToggleBreakpoint)
	awaitStateWithin(t, h, 10*time.Second, "the breakpoint in "+program.source, func() bool {
		for path, lines := range p.debug.breakpoints {
			if samePath(path, source) && len(lines) == 1 && lines[0].line == int64(program.line) {
				return true
			}
		}
		return false
	})
	clickNamedWithoutSettling(t, h, h.shared("ui.done"))
	awaitStateWithin(t, h, 10*time.Second, "the closed Breakpoints dialog", func() bool { return !p.panels.breakpointsOpen })

	clickNamedWithoutSettling(t, h, h.app.text.Debug.Start)
	// Delve builds the package before it runs it, so the first stop waits for a build.
	awaitStateWithin(t, h, 2*time.Minute, "Delve to stop in "+program.source, func() bool {
		if p.debug.failure != "" {
			t.Fatalf("debugging %s failed: %s\noutput: %q", program.source, p.debug.failure, p.debug.output)
		}
		return p.debug.state == "paused" && len(p.debug.frames) > 0
	})
	top := p.debug.frames[0]
	if top.Name != "main.main" || top.Line != int64(program.line) || !samePath(top.SourcePath, source) {
		t.Fatalf("top frame = %+v, want main.main at %s:%d", top, program.source, program.line)
	}
	// The locals open by themselves once the stop's scopes arrive.
	awaitStateWithin(t, h, time.Minute, "main's local name in Variables", func() bool {
		for _, scope := range p.debug.vars.scopes {
			if scope.container == nil {
				continue
			}
			for _, node := range scope.container.nodes {
				if node.name == "name" && node.value == program.local {
					return true
				}
			}
		}
		return false
	})
	// The announcement is how the host found Delve, not console output.
	if output := strings.Join(p.debug.output, "\n"); strings.Contains(output, "DAP server listening at") {
		t.Fatalf("the console shows Delve's announcement: %q", output)
	}

	clickNamedWithoutSettling(t, h, h.app.text.Debug.Continue)
	awaitStateWithin(t, h, time.Minute, program.source+" to end its session", func() bool {
		return p.debug.activeSession() == nil && p.debug.state == "terminated"
	})
	// lab-2 prints what it read from examples/greeting.txt, which it finds only
	// when it runs in its own directory.
	greeting := "Hello, " + strings.Trim(program.local, `"`) + "!"
	if output := strings.Join(p.debug.output, "\n"); !strings.Contains(output, greeting) {
		t.Fatalf("the console does not show %q: %q", greeting, output)
	}
	// The ended session closes Delve's connection and asks Delve to stop within the stop
	// time; Delve then deletes the program it built in the package's directory.
	awaitStateWithin(t, h, 30*time.Second, "Delve to remove its debug binary", func() bool {
		return len(debugBinaries(t, filepath.Dir(source))) == 0
	})
}
