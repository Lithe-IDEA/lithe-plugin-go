package projectgen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/1lck/Lithe-IDEA/desktop/internal/core"
)

// The Go plugin's project generator, run through Core's form and plan and this
// executor. It is an integration test: LITHE_PLUGIN_GO_DIR names the directory
// that holds the plugin's plugin.json, and the build check needs an installed go.
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

// generateGoModule runs the go-module generator for name with the given field
// values and returns the created project.
func generateGoModule(t *testing.T, ctx context.Context, name string, values map[string]any, edited []string) Result {
	t.Helper()
	directory, manifest := goPluginDirectory(t)
	kernel := core.NewClient()
	input := core.ProjectGeneratorFormInput{ProjectName: name, Values: values, Edited: edited}
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
	result := Run(ctx, Request{Parent: t.TempDir(), Name: name, Plan: plan, Kernel: kernel})
	if result.Failure != nil {
		t.Fatalf("%v\n%s", result.Failure, result.Failure.Tail)
	}
	return result
}

// goCommand runs go in dir and returns what it printed.
func goCommand(t *testing.T, ctx context.Context, dir string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = dir
	// The module needs nothing from the network, and the build cache stays out of the user's.
	command.Env = append(os.Environ(), "GOPROXY=off", "GOCACHE="+t.TempDir())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func TestGoPluginGeneratesAModuleWithSampleCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	result := generateGoModule(t, ctx, "Hello-Go", nil, nil)
	goMod, err := os.ReadFile(filepath.Join(result.Path, "go.mod"))
	if err != nil || string(goMod) != "module hello-go\n\ngo 1.21\n" {
		t.Fatalf("go.mod = %q, %v; want the lowercased project name and the plugin's minimum Go", goMod, err)
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
	if _, err := exec.LookPath("go"); err != nil {
		return
	}
	goCommand(t, ctx, result.Path, "vet", "./...")
	if output := goCommand(t, ctx, result.Path, "run", "."); output != "Hello, Gopher!\n" {
		t.Fatalf("go run printed %q", output)
	}
}

func TestGoPluginGeneratesAnEmptyModule(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	result := generateGoModule(t, ctx, "tool", map[string]any{
		"addSampleCode": false,
		"modulePath":    "github.com/example/tool",
		"goVersion":     "1.23",
	}, []string{"addSampleCode", "modulePath", "goVersion"})
	goMod, err := os.ReadFile(filepath.Join(result.Path, "go.mod"))
	if err != nil || string(goMod) != "module github.com/example/tool\n\ngo 1.23\n" {
		t.Fatalf("go.mod = %q, %v", goMod, err)
	}
	if _, err := os.Stat(filepath.Join(result.Path, "main.go")); !os.IsNotExist(err) {
		t.Fatalf("main.go exists without sample code: %v", err)
	}
	if !slices.Equal(result.OpenFiles, []string{"go.mod"}) {
		t.Fatalf("open files = %v, want go.mod", result.OpenFiles)
	}
}

func TestGoPluginFormRejectsAModulePathWithSpaces(t *testing.T) {
	_, manifest := goPluginDirectory(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	kernel := core.NewClient()
	// The module path follows the project name until it is edited, and a space is
	// not valid in one.
	form, err := kernel.ProjectGeneratorFormOf(ctx, manifest, "go-module", core.ProjectGeneratorFormInput{ProjectName: "My App"})
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
	})
	if err != nil || !form.Valid {
		t.Fatalf("form = %+v, %v; want the edited module path accepted", form, err)
	}
	if goVersion := fieldState(form, "goVersion"); goVersion.Value != "1.21" {
		t.Fatalf("goVersion = %+v, want the default 1.21", goVersion)
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
