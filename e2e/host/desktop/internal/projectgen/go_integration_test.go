package projectgen

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/1lck/Lithe-IDEA/desktop/internal/core"
	"github.com/1lck/Lithe-IDEA/desktop/internal/toolchain"
)

// The Go plugin's project generator, run through Core's form and plan and this
// executor. It is an integration test: LITHE_PLUGIN_GO_DIR names the directory
// that holds the plugin's plugin.json. The checks that build the module, and the
// ones that find this machine's Go, need an installed go.
func goPluginDirectory(t *testing.T) (string, []byte) {
	t.Helper()
	directory := os.Getenv("LITHE_PLUGIN_GO_DIR")
	if directory == "" {
		t.Skip("LITHE_PLUGIN_GO_DIR is not set")
	}
	manifest, err := os.ReadFile(filepath.Join(directory, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	return directory, manifest
}

// goInstallation is one Go the plugin's tool definition found, with the
// provider the host records in a pin.
type goInstallation struct {
	core.ProjectToolInstallation
	Provider string
}

// localGoInstallations lists the Go installations the plugin's "go" tool
// definition finds on this machine, the way New Project lists them for the
// GOROOT field: discovery in an empty directory, so no project pin applies.
func localGoInstallations(t *testing.T, ctx context.Context, manifest []byte) []goInstallation {
	t.Helper()
	snapshot, err := toolchain.Load(ctx, core.NewClient(), t.TempDir(), ".", []json.RawMessage{manifest}, toolchain.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var found []goInstallation
	for _, tool := range snapshot.Tools {
		if tool.ID != "go" {
			continue
		}
		for _, installation := range tool.Installations {
			found = append(found, goInstallation{
				ProjectToolInstallation: core.ProjectToolInstallation{
					Home:       installation.Home,
					Executable: installation.Executable,
					Version:    installation.Version,
				},
				Provider: installation.Provider,
			})
		}
	}
	return found
}

func projectInstallations(found []goInstallation) []core.ProjectToolInstallation {
	installations := make([]core.ProjectToolInstallation, 0, len(found))
	for _, installation := range found {
		installations = append(installations, installation.ProjectToolInstallation)
	}
	return installations
}

// generateGoModule runs the go-module generator with input and returns the plan
// and the created project. savePins, when set, records the plan's tool pins.
func generateGoModule(t *testing.T, ctx context.Context, input core.ProjectGeneratorFormInput,
	savePins func(context.Context, string, core.ProjectGenerationPlan) error,
) (core.ProjectGenerationPlan, Result) {
	t.Helper()
	directory, manifest := goPluginDirectory(t)
	kernel := core.NewClient()
	form, err := kernel.ProjectGeneratorFormOf(ctx, manifest, "go-module", input)
	if err != nil || !form.Valid {
		t.Fatalf("form = %+v, %v", form, err)
	}
	plan, err := kernel.ProjectGenerationPlanOf(ctx, manifest, "go-module", directory, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) == 0 {
		t.Fatalf("plan = %+v, want the template steps", plan)
	}
	result := Run(ctx, Request{Parent: t.TempDir(), Name: input.ProjectName, Plan: plan, Kernel: kernel, SavePins: savePins})
	if result.Failure != nil {
		t.Fatalf("%v\n%s", result.Failure, result.Failure.Tail)
	}
	return plan, result
}

// goCommand runs the go executable in dir and returns what it printed.
func goCommand(t *testing.T, ctx context.Context, executable, dir string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = dir
	// The module needs nothing from the network, the build cache stays out of the
	// user's, and the chosen Go builds it instead of a downloaded toolchain.
	command.Env = append(os.Environ(), "GOPROXY=off", "GOTOOLCHAIN=local", "GOCACHE="+t.TempDir())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", executable, strings.Join(args, " "), err, output)
	}
	return string(output)
}

// goEnvironment returns the GOROOT and release of the go on PATH. It skips the
// test when there is none, or when that go is a development build without a
// release number.
func goEnvironment(t *testing.T, ctx context.Context) (root, version string) {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go on PATH")
	}
	output, err := exec.CommandContext(ctx, path, "env", "GOROOT", "GOVERSION").Output()
	if err != nil {
		t.Fatalf("go env: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 {
		t.Fatalf("go env printed %q", output)
	}
	release := regexp.MustCompile(`^go(\d+(?:\.\d+)*)$`).FindStringSubmatch(strings.TrimSpace(lines[1]))
	if release == nil {
		t.Skipf("go on PATH is %q, not a release", lines[1])
	}
	return strings.TrimSpace(lines[0]), release[1]
}

func samePath(t *testing.T, a, b string) bool {
	t.Helper()
	resolvedA, errA := filepath.EvalSymlinks(a)
	resolvedB, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && resolvedA == resolvedB
}

func TestGoPluginToolDefinitionFindsTheLocalGo(t *testing.T) {
	_, manifest := goPluginDirectory(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	root, version := goEnvironment(t, ctx)
	found := localGoInstallations(t, ctx, manifest)
	for _, installation := range found {
		if samePath(t, installation.Home, root) {
			if installation.Version != version {
				t.Fatalf("installation %+v, want version %s from go env GOVERSION", installation, version)
			}
			return
		}
	}
	t.Fatalf("installations = %+v, want the GOROOT %s of the go on PATH", found, root)
}

func TestGoPluginGeneratesAModuleWithSampleCode(t *testing.T) {
	_, manifest := goPluginDirectory(t)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	found := localGoInstallations(t, ctx, manifest)
	local := len(found) > 0
	if !local {
		// No Go on this machine: a listed installation still decides the go line.
		home := t.TempDir()
		found = []goInstallation{{ProjectToolInstallation: core.ProjectToolInstallation{
			Home: home, Executable: filepath.Join(home, "bin", "go"), Version: "1.22.3",
		}}}
	}
	kernel := core.NewClient()
	input := core.ProjectGeneratorFormInput{
		ProjectName: "Hello-Go",
		Tools:       map[string][]core.ProjectToolInstallation{"goroot": projectInstallations(found)},
	}
	form, err := kernel.ProjectGeneratorFormOf(ctx, manifest, "go-module", input)
	if err != nil {
		t.Fatal(err)
	}
	// Without a choice the GOROOT field holds the newest installation.
	var chosen goInstallation
	for _, installation := range found {
		if installation.Home == fieldState(form, "goroot").Value {
			chosen = installation
		}
	}
	if chosen.Home == "" {
		t.Fatalf("goroot = %+v, want one of %+v", fieldState(form, "goroot"), found)
	}
	// The project records the chosen GOROOT the way New Project does.
	savePins := func(ctx context.Context, root string, plan core.ProjectGenerationPlan) error {
		var edits []toolchain.Edit
		for _, pin := range plan.ToolPins {
			edits = append(edits, toolchain.Edit{Tool: pin.Tool, Pin: &core.ToolchainPin{Home: pin.Home, Version: chosen.Version, Provider: chosen.Provider}})
		}
		_, _, err := toolchain.SavePins(ctx, kernel, root, ".", edits, nil)
		return err
	}
	if !local {
		savePins = nil
	}
	plan, result := generateGoModule(t, ctx, input, savePins)
	goMod, err := os.ReadFile(filepath.Join(result.Path, "go.mod"))
	if want := "module hello-go\n\ngo " + chosen.Version + "\n"; err != nil || string(goMod) != want {
		t.Fatalf("go.mod = %q, %v; want %q: the lowercased project name and the chosen Go's version", goMod, err, want)
	}
	wantPins := []core.ProjectPlanToolPin{{Field: "goroot", Tool: "go", PinAs: "project", Home: chosen.Home, Executable: chosen.Executable}}
	if !slices.Equal(plan.ToolPins, wantPins) {
		t.Fatalf("tool pins = %+v, want %+v", plan.ToolPins, wantPins)
	}
	if _, err := os.Stat(filepath.Join(result.Path, "main.go")); err != nil {
		t.Fatalf("the sample program was not generated: %v", err)
	}
	ignore, err := os.ReadFile(filepath.Join(result.Path, ".gitignore"))
	if err != nil || !strings.Contains(string(ignore), "__debug_bin*") {
		t.Fatalf(".gitignore = %q, %v; want the dot- template renamed and Delve's binary ignored", ignore, err)
	}
	if _, err := os.Stat(filepath.Join(result.Path, "dot-gitignore")); !os.IsNotExist(err) {
		t.Fatalf("dot-gitignore was copied under its template name: %v", err)
	}
	if !slices.Equal(result.OpenFiles, []string{"main.go"}) {
		t.Fatalf("open files = %v, want main.go", result.OpenFiles)
	}
	if !local {
		return
	}
	// The build resolves the go tool to the pinned GOROOT, and that Go builds the module.
	selection, err := toolchain.Resolve(ctx, kernel, result.Path, ".", "go", "1.21", nil, []json.RawMessage{manifest}, toolchain.Options{})
	if err != nil || selection.Home == nil || !samePath(t, *selection.Home, chosen.Home) {
		t.Fatalf("selection = %+v, %v; want the pinned GOROOT %s", selection, err, chosen.Home)
	}
	goCommand(t, ctx, selection.Executable, result.Path, "vet", "./...")
	if output := goCommand(t, ctx, selection.Executable, result.Path, "run", "."); output != "Hello, Gopher!\n" {
		t.Fatalf("go run printed %q", output)
	}
}

func TestGoPluginGeneratesAnEmptyModuleWithoutAGoroot(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	// GOROOT is optional: with no Go found, a typed version still creates the module.
	plan, result := generateGoModule(t, ctx, core.ProjectGeneratorFormInput{
		ProjectName: "tool",
		Values: map[string]any{
			"addSampleCode": false,
			"modulePath":    "github.com/example/tool",
			"goVersion":     "1.23",
		},
		Edited: []string{"addSampleCode", "modulePath", "goVersion"},
	}, nil)
	goMod, err := os.ReadFile(filepath.Join(result.Path, "go.mod"))
	if err != nil || string(goMod) != "module github.com/example/tool\n\ngo 1.23\n" {
		t.Fatalf("go.mod = %q, %v", goMod, err)
	}
	if len(plan.ToolPins) != 0 {
		t.Fatalf("tool pins = %+v, want none without a GOROOT", plan.ToolPins)
	}
	if _, err := os.Stat(filepath.Join(result.Path, "main.go")); !os.IsNotExist(err) {
		t.Fatalf("main.go exists without sample code: %v", err)
	}
	if !slices.Equal(result.OpenFiles, []string{"go.mod"}) {
		t.Fatalf("open files = %v, want go.mod", result.OpenFiles)
	}
}

func TestGoPluginGoVersionFollowsTheGoroot(t *testing.T) {
	_, manifest := goPluginDirectory(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	kernel := core.NewClient()
	base := t.TempDir()
	install := func(name, version string) core.ProjectToolInstallation {
		home := filepath.Join(base, name)
		return core.ProjectToolInstallation{Home: home, Executable: filepath.Join(home, "bin", "go"), Version: version}
	}
	older, newer, tooOld := install("go1.22", "1.22.3"), install("go1.25", "1.25.0"), install("go1.20", "1.20.14")
	evaluate := func(input core.ProjectGeneratorFormInput) core.ProjectGeneratorForm {
		t.Helper()
		input.ProjectName = "hello"
		form, err := kernel.ProjectGeneratorFormOf(ctx, manifest, "go-module", input)
		if err != nil {
			t.Fatal(err)
		}
		return form
	}
	// The host sends an empty list, never null, for a tool it did not find.
	tools := func(installations ...core.ProjectToolInstallation) map[string][]core.ProjectToolInstallation {
		return map[string][]core.ProjectToolInstallation{"goroot": append([]core.ProjectToolInstallation{}, installations...)}
	}

	// No Go found: GOROOT stays empty without a problem, and the version must be typed.
	for _, found := range [][]core.ProjectToolInstallation{nil, {tooOld}} {
		form := evaluate(core.ProjectGeneratorFormInput{Tools: tools(found...)})
		goroot, goVersion := fieldState(form, "goroot"), fieldState(form, "goVersion")
		if form.Valid || goroot.Value != "" || goroot.Problem != "" || goVersion.Value != "" || goVersion.Problem != "required" {
			t.Fatalf("found %v: form = %+v; want an empty optional GOROOT and a required Go version", found, form)
		}
	}
	// The version starts as the newest Go's, then as the chosen one's.
	form := evaluate(core.ProjectGeneratorFormInput{Tools: tools(older, newer)})
	if !form.Valid || fieldState(form, "goroot").Value != newer.Home || fieldState(form, "goVersion").Value != "1.25.0" {
		t.Fatalf("form = %+v, want the newest GOROOT and its version", form)
	}
	form = evaluate(core.ProjectGeneratorFormInput{Values: map[string]any{"goroot": older.Home}, Tools: tools(older, newer)})
	if !form.Valid || fieldState(form, "goVersion").Value != "1.22.3" {
		t.Fatalf("form = %+v, want the chosen GOROOT's version", form)
	}
	// A typed version stays when the GOROOT changes.
	form = evaluate(core.ProjectGeneratorFormInput{
		Values: map[string]any{"goroot": older.Home, "goVersion": "1.21"},
		Edited: []string{"goVersion"},
		Tools:  tools(older, newer),
	})
	if !form.Valid || fieldState(form, "goVersion").Value != "1.21" {
		t.Fatalf("form = %+v, want the typed version kept", form)
	}
	// A Go older than the plugin's minimum can be chosen but is reported.
	form = evaluate(core.ProjectGeneratorFormInput{Values: map[string]any{"goroot": tooOld.Home}, Tools: tools(tooOld, newer)})
	if form.Valid || fieldState(form, "goroot").Problem == "" {
		t.Fatalf("form = %+v, want a problem on a GOROOT below Go 1.21", form)
	}
}

func TestGoPluginFormRejectsAModulePathWithSpaces(t *testing.T) {
	_, manifest := goPluginDirectory(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	kernel := core.NewClient()
	home := t.TempDir()
	tools := map[string][]core.ProjectToolInstallation{
		"goroot": {{Home: home, Executable: filepath.Join(home, "bin", "go"), Version: "1.24.2"}},
	}
	// The module path follows the project name until it is edited, and a space is
	// not valid in one.
	form, err := kernel.ProjectGeneratorFormOf(ctx, manifest, "go-module", core.ProjectGeneratorFormInput{ProjectName: "My App", Tools: tools})
	if err != nil {
		t.Fatal(err)
	}
	if form.Valid || fieldState(form, "modulePath").Problem == "" {
		t.Fatalf("form = %+v, want a problem on modulePath", form)
	}
	form, err = kernel.ProjectGeneratorFormOf(ctx, manifest, "go-module", core.ProjectGeneratorFormInput{
		ProjectName: "My App",
		Values:      map[string]any{"modulePath": "example.com/my-app"},
		Edited:      []string{"modulePath"},
		Tools:       tools,
	})
	if err != nil || !form.Valid {
		t.Fatalf("form = %+v, %v; want the edited module path accepted", form, err)
	}
	if goVersion := fieldState(form, "goVersion"); goVersion.Value != "1.24.2" {
		t.Fatalf("goVersion = %+v, want the GOROOT's version 1.24.2", goVersion)
	}
}

func fieldState(form core.ProjectGeneratorForm, id string) core.ProjectFieldState {
	for _, field := range form.Fields {
		if field.ID == id {
			return field
		}
	}
	return core.ProjectFieldState{}
}
