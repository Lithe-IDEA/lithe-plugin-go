package workbench

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/1lck/Lithe-IDEA/desktop/internal/core"
)

// This opt-in integration test covers the run configuration types the Go Support archive
// declares, through the real installer and the Run/Debug Configurations dialog. It runs the
// user's own go, so it skips unless an archive is named.

const officialGoPluginID = "io.github.lithe-idea.go-support"

// officialGoRunTypeDraft opens the dialog, adds a configuration of kind as its + menu does and
// waits for the form.
func officialGoRunTypeDraft(t *testing.T, h *harness, p *project, kind core.RunTypeKey) *runConfigForm {
	t.Helper()
	d := &p.runConfigs.dialog
	h.app.post(func(a *App) {
		a.openRunConfigDialog(p, "")
		a.addRunConfigDraft(p, declaredKind(kind), nil)
	})
	var f *runConfigForm
	awaitStateWithin(t, h, 30*time.Second, "the form of "+kind.Type, func() bool {
		if draft := d.drafts[d.selected]; d.open && draft != nil && draft.form != nil {
			f = draft.form
		}
		return f != nil && f.ready() && f.fields != nil
	})
	if f.failure != "" {
		t.Fatalf("form failure %q", f.failure)
	}
	return f
}

// officialGoEditForm sets texts and flags on form f, as typing into its fields does, and waits
// until the form follows the edit.
func officialGoEditForm(t *testing.T, h *harness, p *project, f *runConfigForm, texts map[string]string, flags map[string]bool) {
	t.Helper()
	edited := false
	h.app.post(func(a *App) {
		for id, value := range texts {
			*f.texts[id] = value
		}
		for id, value := range flags {
			*f.flags[id] = value
		}
		a.runConfigFormEdited(p, f)
		edited = true
	})
	awaitStateWithin(t, h, 30*time.Second, "the edited form", func() bool { return edited && !f.refreshing })
}

// officialGoFieldShown reports whether form f shows the field id for the values being edited.
func officialGoFieldShown(f *runConfigForm, id string) bool {
	for _, field := range f.fields {
		if field.ID == id {
			return visibleRunConfigField(field, f.collect())
		}
	}
	return false
}

// officialGoValues prints the values being edited, for a failure.
func officialGoValues(f *runConfigForm) string {
	text, _ := json.Marshal(f.collect())
	return string(text)
}

// officialGoConditions prints when each field shows, for a failure.
func officialGoConditions(f *runConfigForm) string {
	conditions := map[string]*core.RunFieldCondition{}
	for _, field := range f.fields {
		if field.VisibleWhen != nil {
			conditions[field.ID] = field.VisibleWhen
		}
	}
	text, _ := json.Marshal(conditions)
	return string(text)
}

// officialGoFieldLabels maps the form's field IDs to their labels.
func officialGoFieldLabels(f *runConfigForm) map[string]core.RunEditorField {
	fields := map[string]core.RunEditorField{}
	for _, field := range f.fields {
		fields[field.ID] = field
	}
	return fields
}

// officialGoAwaitRun waits for the run of the configuration named label to end and returns its
// console, failing unless the program exited with 0.
func officialGoAwaitRun(t *testing.T, h *harness, p *project, label string) string {
	t.Helper()
	var found *runEntry
	defer func() {
		if found == nil {
			for _, entry := range p.build.runs {
				t.Logf("session %q running %v outcome %v summary %q console:\n%s", entry.label, entry.running, entry.outcome, entry.summary, entry.output.all())
			}
			t.Logf("configurations %+v failure %q notice %q", p.runConfigs.list, p.runConfigs.failure, lastNotice(h))
		}
	}()
	awaitStateWithin(t, h, 2*time.Minute, "the run of "+label, func() bool {
		for _, entry := range p.build.runs {
			if entry.label == label && !entry.running && !p.build.active() {
				found = entry
				return true
			}
		}
		return false
	})
	if found.outcome != runSucceeded || found.summary != h.app.text.Run.Exited(0) {
		t.Fatalf("run %s ended %v: %s; console:\n%s", label, found.outcome, found.summary, found.output.all())
	}
	return found.output.all()
}

// TestOfficialGoPluginRunsGoBuildAndGoTestConfigurations covers the Go Build and Go Test kinds:
// both list under the package's Chinese names, a Go Build of a package with program arguments
// and a Go Test of a directory with a pattern are created in the dialog, saved to the plugin's
// run document, and run the user's go with the arguments the kinds make of the fields.
func TestOfficialGoPluginRunsGoBuildAndGoTestConfigurations(t *testing.T) {
	archive := os.Getenv("LITHE_TEST_GO_PLUGIN_ARCHIVE")
	if archive == "" {
		t.Skip("set LITHE_TEST_GO_PLUGIN_ARCHIVE to a go-support-<version>-darwin.zip")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("go is not on PATH: %v", err)
	}
	h := newHarness(t, &testKernel{Client: core.NewClient()})
	installed := installOfficialPluginArchive(t, h, archive, officialGoPluginID)
	before := packageDigest(t, installed)
	h.app.reloadPluginLanguages()
	root := t.TempDir()
	writePluginFixture(t, root, map[string]string{
		"go.mod":           "module example.com/hello\n\ngo 1.21\n",
		"cmd/echo/main.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n\nfunc main() { fmt.Println(\"echo:\", strings.Join(os.Args[1:], \"|\")) }\n",
		"greet/greet.go":   "package greet\n\nfunc Hello(name string) string { return \"Hello, \" + name + \"!\" }\n",
		"greet/greet_test.go": "package greet\n\nimport \"testing\"\n\n" +
			"func TestGreet(t *testing.T) { if Hello(\"Go\") != \"Hello, Go!\" { t.Fatal(\"wrong greeting\") } }\n\n" +
			"func TestOther(t *testing.T) { t.Fatal(\"the pattern must leave this test out\") }\n",
	})
	h.open(root)
	p := h.app.project
	if p == nil {
		t.Fatal("the module did not open")
	}
	h.app.loadBuildSystems(p)
	awaitBuildModel(t, h, p, "go", time.Minute)
	build := core.RunTypeKey{PluginID: officialGoPluginID, BuildSystemID: "go", Type: "build"}
	test := core.RunTypeKey{PluginID: officialGoPluginID, BuildSystemID: "go", Type: "test"}
	s := &p.runConfigs
	h.app.post(func(a *App) { a.loadTitleRunConfigs(p) })
	awaitStateWithin(t, h, time.Minute, "the Go kinds", func() bool {
		return p.runConfigsSettled() && !s.typesLoading && s.types != nil && p.hasRunTypeInstance(build)
	})
	for key, want := range map[core.RunTypeKey]string{build: "Go 构建", test: "Go 测试"} {
		if kind := s.runTypeKind(key); kind == nil || kind.DisplayName != want {
			t.Fatalf("kind %s = %+v, want %q among %+v", key.Type, kind, want, s.types)
		}
	}

	// Go Build: the run kind starts on a package; its path and the program arguments run
	// `go run ./cmd/echo one two`.
	f := officialGoRunTypeDraft(t, h, p, build)
	fields := officialGoFieldLabels(f)
	if kind := fields["field:runKind"]; kind.Label != "运行种类" || kind.OptionLabels["package"] != "软件包" || kind.OptionLabels["file"] != "文件" {
		t.Fatalf("run kind field = %+v", kind)
	}
	if pkg := fields["field:package"]; pkg.Label != "软件包路径" || !pkg.Required {
		t.Fatalf("package field = %+v", pkg)
	}
	if !officialGoFieldShown(f, "field:package") || officialGoFieldShown(f, "field:file") || officialGoFieldShown(f, "field:directory") {
		t.Fatalf("a package run shows other fields than the package path: values %s, conditions %s", officialGoValues(f), officialGoConditions(f))
	}
	officialGoEditForm(t, h, p, f, map[string]string{"name": "Echo", "field:package": "./cmd/echo", "programArguments": "one two"},
		map[string]bool{"storeAsProjectFile": true})
	applyDialog(t, h, p, runConfigDialogRun)
	if s.dialog.open {
		t.Fatalf("the dialog stayed open: errors %v failure %q / %q", f.errors, f.failure, s.dialog.failure)
	}
	if output := officialGoAwaitRun(t, h, p, "Echo"); !strings.Contains(output, "echo: one|two") {
		t.Fatalf("go run did not pass the program arguments: %q", output)
	}

	// Go Test of a directory: the directory field replaces the package path, and the pattern
	// leaves the failing TestOther out of `go test -v -run ^TestGreet$ <root>/...`.
	f = officialGoRunTypeDraft(t, h, p, test)
	officialGoEditForm(t, h, p, f, map[string]string{"name": "Greet tests", "field:testKind": "directory"}, nil)
	fields = officialGoFieldLabels(f)
	if directory := fields["field:directory"]; directory.Label != "目录" || directory.Kind != runFieldDirectory {
		t.Fatalf("directory field = %+v", directory)
	}
	if !officialGoFieldShown(f, "field:directory") || officialGoFieldShown(f, "field:package") {
		t.Fatalf("a directory test shows the package path instead of the directory: values %s, conditions %s", officialGoValues(f), officialGoConditions(f))
	}
	if pattern := fields["field:pattern"]; pattern.Label != "模式" {
		t.Fatalf("pattern field = %+v", pattern)
	}
	officialGoEditForm(t, h, p, f, map[string]string{
		"field:directory": root, "field:pattern": "^TestGreet$", "field:goToolArguments": "-v -count=1",
	}, map[string]bool{"storeAsProjectFile": true})
	applyDialog(t, h, p, runConfigDialogRun)
	if s.dialog.open {
		t.Fatalf("the dialog stayed open: errors %v failure %q / %q", f.errors, f.failure, s.dialog.failure)
	}
	output := officialGoAwaitRun(t, h, p, "Greet tests")
	if !strings.Contains(output, "--- PASS: TestGreet") || strings.Contains(output, "TestOther") {
		t.Fatalf("go test did not run only TestGreet: %q", output)
	}

	// Both are project configurations of the plugin's kinds, kept in its run document.
	data, err := os.ReadFile(filepath.Join(root, ".lithe", "run", "plugin", officialGoPluginID+".toml"))
	if err != nil || !strings.Contains(string(data), "Echo") || !strings.Contains(string(data), "Greet tests") {
		t.Fatalf("plugin run document = %s, %v", data, err)
	}
	h.app.closeProject()
	awaitStateWithin(t, h, 30*time.Second, "the closed project", func() bool { return h.app.project == nil })
	h.settle()
	assertPackageUnchanged(t, installed, before)
}
