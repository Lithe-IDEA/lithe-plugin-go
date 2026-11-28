package workbench

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/1lck/Lithe-IDEA/desktop/internal/core"
)

// This opt-in integration test installs the Go Support archive and opens a repository that
// holds several Go modules, as a user does. The backend starts with the workspace, finds each
// go.mod through the host's file search and marks the directory of every nested module as a
// source directory, which Core resolves into what the project tree draws. The root's own module
// is not marked, nor are the directories the go command skips. It needs neither go nor gopls.

func TestOfficialGoPluginMarksNestedModuleDirectories(t *testing.T) {
	archive := os.Getenv("LITHE_TEST_GO_PLUGIN_ARCHIVE")
	if archive == "" {
		t.Skip("set LITHE_TEST_GO_PLUGIN_ARCHIVE to a go-support-<version>-darwin.zip")
	}
	const id = "io.github.lithe-idea.go-support"
	h := newHarness(t, &testKernel{Client: core.NewClient()})
	installed := installOfficialPluginArchive(t, h, archive, id)
	t.Cleanup(h.app.plugins.Host.StopAll)
	// The window attaches itself to the plugin service as it starts; the file search and the
	// directory marks are host methods only a window answers.
	if err := h.app.plugins.AttachWorkbench(pluginWorkbench{app: h.app}); err != nil {
		t.Fatal(err)
	}
	before := packageDigest(t, installed)
	root := t.TempDir()
	module := func(path string) string { return "module example.com/repo" + path + "\n\ngo 1.21\n" }
	writePluginFixture(t, root, map[string]string{
		"go.mod":                        module(""),
		"main.go":                       "package main\n\nfunc main() {}\n",
		"tools/go.mod":                  module("/tools"),
		"services/api/go.mod":           module("/services/api"),
		"services/api/main.go":          "package main\n\nfunc main() {}\n",
		"vendor/example.com/dep/go.mod": "module example.com/dep\n\ngo 1.21\n",
		"testdata/fixture/go.mod":       "module example.com/fixture\n\ngo 1.21\n",
		"_old/go.mod":                   module("/_old"),
		"docs/go.mod.bak":               module("/docs"),
	})
	h.open(root)
	p := h.app.project
	if p == nil {
		t.Fatal("the repository did not open")
	}

	want := []core.PluginDirectoryMark{
		{PluginID: id, Path: "services/api", Role: "sources"},
		{PluginID: id, Path: "tools", Role: "sources"},
	}
	awaitStateWithin(t, h, time.Minute, "the nested module directories to be marked", func() bool {
		return slices.Equal(p.pluginDirectoryMarks(), want)
	})
	// Core resolves the plugin's mark into the row the project tree draws.
	awaitStateWithin(t, h, 30*time.Second, "the tools directory drawn as sources", func() bool {
		entry, ok := p.indexing.presentation["tools"]
		return ok && entry.Role == "sources" && entry.PluginID == id
	})
	for _, skipped := range []string{"vendor", "testdata", "_old", "docs"} {
		if entry := p.indexing.presentation[skipped]; entry.PluginID == id {
			t.Fatalf("%s is marked by the plugin: %+v", skipped, entry)
		}
	}

	// Opening the repository also detects and syncs each eligible Go Modules instance;
	// a running sync would make closing the project ask first.
	awaitStateWithin(t, h, 2*time.Minute, "the Go Modules syncs to end", func() bool {
		if !p.build.loaded || p.build.loading {
			return false
		}
		for _, run := range p.build.runs {
			if run.running {
				return false
			}
		}
		return true
	})
	var roots []string
	for _, instance := range p.build.instances {
		if instance.PluginID == id && instance.BuildSystemID == "go" {
			roots = append(roots, instance.Root)
		}
	}
	slices.Sort(roots)
	if !slices.Equal(roots, []string{".", "services/api", "tools"}) {
		t.Fatalf("Go Modules detected roots %q", roots)
	}
	h.app.closeProject()
	awaitStateWithin(t, h, 30*time.Second, "the closed project", func() bool { return h.app.project == nil })
	h.settle()
	assertPackageUnchanged(t, installed, before)
}
