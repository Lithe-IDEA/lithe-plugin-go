package workbench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/1lck/Lithe-IDEA/desktop/internal/buildprocess"
	"github.com/1lck/Lithe-IDEA/desktop/internal/core"
)

// goDebugSample is main.go of the debugged module. Zero-based line 6 is the
// Printf call, where main's local name already holds "Gopher".
const goDebugSample = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tname := \"Gopher\"\n\tfmt.Printf(\"Hello, %s!\\n\", name)\n}\n"

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

// TestOfficialGoPluginDebugsAModuleWithDelve covers the Delve adapter of the Go
// Support archive: the Debug window starts the user's dlv dap, the host reads the
// address Delve announces and connects, a breakpoint in main.go stops main.main
// with its local variable, Continue runs the program to its end and the session
// ends by itself, and Delve removes the program it built for the session.
func TestOfficialGoPluginDebugsAModuleWithDelve(t *testing.T) {
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
	writePluginFixture(t, root, map[string]string{
		"go.mod":  "module example.com/hello\n\ngo 1.21\n",
		"main.go": goDebugSample,
	})
	p := openHostedPanel(t, h, root)
	if declaration := p.debug.adapters[0].Declaration; declaration.Transport != "tcpAnnounced" || declaration.LaunchKind != "systemCommand" {
		t.Fatalf("declaration = %+v, want Delve started as dlv dap and announcing its address", declaration)
	}
	// Opening main.go starts gopls, whose tasks run until the project closes, so from
	// here on every step waits for its own state instead of for all background work.
	source := filepath.Join(root, "main.go")
	h.app.openFileAtPosition(p, source, 6, 0)
	awaitStateWithin(t, h, 30*time.Second, "main.go with the caret on the Printf line", func() bool {
		d := p.selectedDocument(p.docs.group)
		if d == nil || !samePath(d.path, source) {
			return false
		}
		line, _ := d.view.Caret()
		return line == 6
	})
	clickNamedWithoutSettling(t, h, h.shared("native.viewBreakpoints"))
	awaitStateWithin(t, h, 10*time.Second, "the Breakpoints dialog", func() bool { return p.panels.breakpointsOpen })
	clickNamedWithoutSettling(t, h, h.app.text.Debug.ToggleBreakpoint)
	awaitStateWithin(t, h, 10*time.Second, "the breakpoint on main.go:7", func() bool {
		for path, lines := range p.debug.breakpoints {
			if samePath(path, source) && len(lines) == 1 && lines[0].line == 7 {
				return true
			}
		}
		return false
	})
	clickNamedWithoutSettling(t, h, h.shared("ui.done"))
	awaitStateWithin(t, h, 10*time.Second, "the closed Breakpoints dialog", func() bool { return !p.panels.breakpointsOpen })

	clickNamedWithoutSettling(t, h, h.app.text.Debug.Start)
	// Delve builds the package before it runs it, so the first stop waits for a build.
	awaitStateWithin(t, h, 2*time.Minute, "Delve to stop at the breakpoint", func() bool {
		if p.debug.failure != "" {
			t.Fatalf("debug start failed: %s\noutput: %q", p.debug.failure, p.debug.output)
		}
		return p.debug.state == "paused" && len(p.debug.frames) > 0
	})
	top := p.debug.frames[0]
	if top.Name != "main.main" || top.Line != 7 || !samePath(top.SourcePath, source) {
		t.Fatalf("top frame = %+v, want main.main at main.go:7", top)
	}
	// The locals open by themselves once the stop's scopes arrive.
	awaitStateWithin(t, h, time.Minute, "main's local name in Variables", func() bool {
		for _, scope := range p.debug.vars.scopes {
			if scope.container == nil {
				continue
			}
			for _, node := range scope.container.nodes {
				if node.name == "name" && node.value == `"Gopher"` {
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
	awaitStateWithin(t, h, time.Minute, "the program to end its session", func() bool {
		return p.debug.activeSession() == nil && p.debug.state == "terminated"
	})
	if output := strings.Join(p.debug.output, "\n"); !strings.Contains(output, "Hello, Gopher!") {
		t.Fatalf("the console does not show the program's output: %q", output)
	}
	// The ended session closes Delve's connection and asks Delve to stop within the stop
	// time; Delve then deletes the program it built, so nothing is left in the module.
	awaitStateWithin(t, h, 30*time.Second, "Delve to remove its debug binary", func() bool {
		return len(debugBinaries(t, root)) == 0
	})

	h.app.closeProject()
	awaitStateWithin(t, h, 30*time.Second, "the closed project", func() bool { return h.app.project == nil })
	h.settle()
	assertPackageUnchanged(t, installed, before)
}
