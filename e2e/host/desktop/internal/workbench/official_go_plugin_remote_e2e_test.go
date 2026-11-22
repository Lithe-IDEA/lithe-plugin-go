//go:build lithe_remote_e2e

package workbench

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/1lck/Lithe-IDEA/desktop/internal/core"
	"github.com/1lck/Lithe-IDEA/desktop/internal/i18n"
	"github.com/1lck/Lithe-IDEA/desktop/internal/remoteplaceholder"
	"github.com/1lck/Lithe-IDEA/desktop/internal/remoteserver"
	"github.com/vivaui/viva/app"
)

// Use a disposable Linux SSH host with go, gopls and dlv on the remote login PATH. Set
// LITHE_TEST_GO_SSH_HOST to its OpenSSH alias, LITHE_TEST_GO_PLUGIN_ARCHIVE to
// the local plugin ZIP and LITHE_REMOTE_SERVER_STAGE to the staged server builds.
// LITHE_TEST_GO_SSH_PROGRAM optionally names an absolute path to an SSH wrapper.
// LITHE_TEST_GO_SSH_INSTALL_ROOT optionally isolates the installed remote server.
// scripts/e2e-host.mjs --remote enables the build tag on the requested host revision.
func TestOfficialGoPluginUsesRemoteWorkspaceOverSSH(t *testing.T) {
	host := os.Getenv("LITHE_TEST_GO_SSH_HOST")
	if host == "" {
		t.Skip("set LITHE_TEST_GO_SSH_HOST to a disposable Linux OpenSSH target")
	}
	archive, stage := os.Getenv("LITHE_TEST_GO_PLUGIN_ARCHIVE"), os.Getenv("LITHE_REMOTE_SERVER_STAGE")
	if archive == "" || stage == "" {
		t.Fatal("set LITHE_TEST_GO_PLUGIN_ARCHIVE and LITHE_REMOTE_SERVER_STAGE")
	}
	bundle, err := remoteserver.Staged(stage)
	if err != nil {
		t.Fatal(err)
	}
	connection := remoteHost{ssh: core.SSHConnection{Host: host}}
	builds := make([]core.RemoteServerBuild, 0, len(bundle.Builds))
	for _, build := range bundle.Builds {
		builds = append(builds, core.RemoteServerBuild{Target: build.Target, Path: build.Path, SHA256: build.SHA256})
	}
	request := connection.request(builds, bundle.Version)
	request.InstallRoot = os.Getenv("LITHE_TEST_GO_SSH_INSTALL_ROOT")
	if ssh := os.Getenv("LITHE_TEST_GO_SSH_PROGRAM"); ssh != "" {
		data, err := json.Marshal(map[string]any{"version": 1, "sshProgram": ssh})
		if err != nil {
			t.Fatal(err)
		}
		preferences := string(data)
		request.Preferences = &preferences
	}
	client := core.NewClient()
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), time.Minute)
	connected, err := client.RemoteConnect(connectCtx, request)
	cancelConnect()
	if err != nil || connected.Status != core.RemoteProbeConnected || connected.Session == nil {
		t.Fatalf("SSH connection failed: status=%s err=%v diagnostics=%s", connected.Status, err, connected.Diagnostics)
	}
	session := connected.Session.ID
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := client.RemoteDisconnect(ctx, session, core.RemoteCloseEnd); err != nil {
			t.Errorf("disconnect SSH session: %v", err)
		}
	})
	platform := connected.Host.PluginPlatform()
	if platform == nil || platform.OS != "linux" {
		t.Fatalf("connected target has no Linux plugin platform: %+v", connected.Host)
	}
	var directory struct {
		Path string `json:"path"`
	}
	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelSetup()
	lease := map[string]string{"directoryId": "go-plugin-ssh-e2e"}
	if err := client.RemoteHostRequest(setupCtx, session, "fs.createRunDirectory", lease, &directory); err != nil || !path.IsAbs(directory.Path) {
		t.Fatalf("create SSH fixture directory: path=%q err=%v", directory.Path, err)
	}
	const modulePath = "example.com/lithe-remote-go"
	for name, content := range map[string]string{
		"go.mod": "module " + modulePath + "\n\ngo 1.21\n",
		"main.go": `package main

import "runtime"

func main() {
	if runtime.GOOS != "linux" {
		panic("expected Linux")
	}
	println("Lithe Go program on " + runtime.GOOS)
}
`,
		"main_test.go": `package main

import (
	"runtime"
	"testing"
)

func TestRunsOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Fatalf("expected Linux, got %s", runtime.GOOS)
	}
}
`,
	} {
		if _, saved, err := client.RemoteWriteFile(setupCtx, session, path.Join(directory.Path, name), []byte(content), nil); err != nil || !saved {
			t.Fatalf("write remote %s: saved=%v err=%v", name, saved, err)
		}
	}
	cancelSetup()

	h := newHarness(t, &testKernel{Client: client})
	installed := installOfficialPluginArchive(t, h, archive, officialGoPluginID)
	before := packageDigest(t, installed)
	h.app.reloadPluginLanguages()
	const connectionID = "go-plugin-ssh-e2e"
	anchor := filepath.Join(t.TempDir(), remoteplaceholder.AnchorName)
	if err := remoteplaceholder.Ensure(anchor); err != nil {
		t.Fatal(err)
	}
	root := remoteplaceholder.Root(anchor, connectionID, directory.Path)
	h.app.openRemoteProject(remoteTarget{
		connectionID: connectionID, name: "Linux SSH test", host: connection, session: *connected.Session,
		platform: platform, remoteRoot: directory.Path, root: root, trusted: true,
	})
	p := h.app.project
	if p == nil || p.remote == nil {
		t.Fatal("the SSH project did not open")
	}
	instance := awaitBuildModel(t, h, p, "go", time.Minute)
	module := modelProjectNamed(instance.Model, modulePath)
	if module == nil {
		t.Fatalf("the remote Go model has no %s module: %+v", modulePath, instance.Model.Projects)
	}
	runBuildTaskToEnd(t, h, p, module.ID, "build", 2*time.Minute)
	var application string
	for _, target := range instance.Model.Targets {
		if slices.Contains(target.Tags, "application") {
			if application != "" {
				t.Fatalf("expected a single application target: %+v", instance.Model.Targets)
			}
			application = target.ID
		}
	}
	if application == "" {
		t.Fatalf("the SSH module has no application target: %+v", instance.Model.Targets)
	}
	if output := runTargetEntryToEnd(t, h, p, instance, application, time.Minute); !strings.Contains(output, "Lithe Go program on linux") {
		t.Fatalf("remote Go program did not print its Linux result: %q", output)
	}
	t.Logf("Built and ran the Go module on Linux %s", platform.Arch)

	h.app.post(func(a *App) { a.runBuildProjectTests(p, instance, module.ID) })
	awaitStateWithin(t, h, 2*time.Minute, "the Go module test run over SSH", func() bool {
		return p.tests.last != nil && !p.tests.running
	})
	if output := p.tests.output.all(); !p.tests.verdict.Success ||
		!strings.Contains(output, "ok") || !strings.Contains(output, modulePath) || strings.Contains(output, "[no test files]") {
		t.Fatalf("the Linux-only test did not pass: verdict=%+v failure=%q output=%q", p.tests.verdict, p.tests.failed, output)
	}
	t.Log("The Build tool window reported the Linux-only test passed")

	file := filepath.Join(root, "main.go")
	h.app.openFileAt(p, file, 1)
	var d *document
	awaitStateWithin(t, h, time.Minute, "remote gopls to open main.go", func() bool {
		d = p.docs.byPath(file)
		return d != nil && d.sync.opened
	})
	debugOfficialGoProgramOverSSH(t, h, p, file, filepath.Join(root, "go.mod"))
	forwardCtx, cancelForward := context.WithTimeout(context.Background(), 10*time.Second)
	forwards, err := client.RemoteForwards(forwardCtx, session)
	cancelForward()
	if err != nil || len(forwards) != 0 {
		t.Fatalf("Delve left a port forward after its session ended: forwards=%+v err=%v", forwards, err)
	}

	if !d.view.GoToNavigation(4, len("func main() {"), false) {
		t.Fatal("could not place the caret in remote main.go")
	}
	input := d.view.TextInput(app.TextInputContext{ID: d.hit})
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextInsert, Text: "\n\tmissingOverSSH()"}); err != nil {
		t.Fatal(err)
	}
	awaitStateWithin(t, h, time.Minute, "a diagnostic from remote gopls", func() bool {
		return problemMessagesContain(h.app, "missingOverSSH") && diagnosticsForFile(h.app, file) > 0
	})
	t.Log("gopls returned a diagnostic for a local edit to the remote file")
	h.app.saveDocument(p, d.id)
	awaitStateWithin(t, h, 30*time.Second, "remote main.go to save", func() bool { return !d.dirty() })
	readCtx, cancelRead := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelRead()
	data, _, err := client.RemoteReadFile(readCtx, session, path.Join(directory.Path, "main.go"))
	if err != nil || !strings.Contains(string(data), "missingOverSSH()") {
		t.Fatalf("the saved source did not reach the remote file: %v", err)
	}

	h.app.closeProject()
	awaitStateWithin(t, h, 20*time.Second, "the SSH project to close or ask about running programs", func() bool {
		return h.app.project == nil || len(h.app.prompts.queue) > 0
	})
	if h.app.project != nil {
		prompt := h.app.prompts.queue[0]
		language := h.app.language()
		end := slices.Index(prompt.actions, i18n.Shared(language, "desktop.remote.close.end"))
		if prompt.title != i18n.Shared(language, "desktop.remote.close.title") || end < 0 {
			t.Fatalf("unexpected prompt when closing the SSH project: %q", prompt.title)
		}
		h.app.answerPrompt(&end)
	}
	awaitStateWithin(t, h, 20*time.Second, "the closed SSH project", func() bool {
		return h.app.project == nil && !h.app.closingProject
	})
	h.settle()
	if data, err := os.ReadFile(anchor); err != nil || len(data) != 0 {
		t.Fatalf("the remote placeholder changed: size=%d err=%v", len(data), err)
	}
	assertPackageUnchanged(t, installed, before)
}

func debugOfficialGoProgramOverSSH(t *testing.T, h *harness, p *project, source, moduleFile string) {
	t.Helper()
	const line = 9
	h.app.openToolWindow(itemDebug)
	awaitStateWithin(t, h, 30*time.Second, "the Go debug adapter list", func() bool { return p.debug.loaded })
	if len(p.debug.adapters) != 1 || p.debug.adapters[0].PluginID != officialGoPluginID {
		t.Fatalf("debug adapters = %+v, want the Go adapter; failure: %s", p.debug.adapters, p.debug.failure)
	}
	h.app.toggleBreakpointAt(p, source, line)
	awaitStateWithin(t, h, 10*time.Second, "the remote Go breakpoint", func() bool {
		breakpoints := p.debug.breakpoints[source]
		return len(breakpoints) == 1 && breakpoints[0].line == line
	})
	clickNamedWithoutSettling(t, h, h.app.text.Debug.Start)
	var session *debugSession
	awaitStateWithin(t, h, 2*time.Minute, "Delve to hit the remote Go breakpoint", func() bool {
		if p.debug.failure != "" {
			t.Fatalf("remote Delve failed: %s\noutput: %q", p.debug.failure, p.debug.output)
		}
		session = p.debug.activeSession()
		return session != nil && p.debug.state == "paused" && len(p.debug.frames) != 0
	})
	if top := p.debug.frames[0]; top.Name != "main.main" || top.Line != line || top.SourcePath != source {
		t.Fatalf("top frame = %+v, want main.main at %s:%d", top, source, line)
	}
	h.app.openFileAt(p, moduleFile, 1)
	// Loading the thread picker shifts the stack rows. Wait for both the editor
	// and the picker before locating the row to click.
	awaitStateWithin(t, h, 20*time.Second, "go.mod and the debug thread picker to appear", func() bool {
		d := p.selectedDocument(p.docs.group)
		return d != nil && d.path == moduleFile && len(p.debug.threads) > 0
	})
	clickNamedWithoutSettling(t, h, "main.main")
	awaitStateWithin(t, h, 20*time.Second, "the stack frame to select the remote source line", func() bool {
		d := p.selectedDocument(p.docs.group)
		if d == nil || d.path != source {
			return false
		}
		caret, _ := d.view.Caret()
		return caret == line-1
	})
	clickNamedWithoutSettling(t, h, h.app.text.Debug.Continue)
	awaitStateWithin(t, h, time.Minute, "the remote debug session to terminate and release its resources", func() bool {
		select {
		case <-session.cleanupDone:
			return p.debug.activeSession() == nil && p.debug.state == "terminated"
		default:
			return false
		}
	})
	if session.cleanupErr != nil {
		t.Fatalf("releasing the remote debug session: %v", session.cleanupErr)
	}
	t.Log("Delve hit a Linux breakpoint, opened the remote source from the call stack and released the session")
}
