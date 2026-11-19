package workbench

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/1lck/Lithe-IDEA/desktop/internal/core"
	"github.com/1lck/Lithe-IDEA/desktop/internal/i18n"
	"github.com/1lck/Lithe-IDEA/desktop/internal/pluginservice"
)

// This opt-in integration test installs the released Go Support archive and opens its two
// settings regions as the settings window does: through the window's own settings source,
// which starts the packaged backend, sends the region's view events and has Core check every
// answer. Every event carries the region's width, as the window measures it, and a resize
// to a wider window draws the scene again at the new width. It needs neither go nor gopls,
// since the backend reads settings only.

// goRegion is one of the plugin's settings regions, its width in the smallest settings window
// and the words of its title and label in each language.
type goRegion struct {
	id    string
	width float64
	words map[i18n.Language][]string
}

var goRegions = []goRegion{
	{id: "language", width: 610, words: map[i18n.Language][]string{
		i18n.English: {"Preview", "Preview of the gopls options", "Hover documentation", "gofumpt formatting"},
		i18n.Chinese: {"效果预览", "gopls 选项的效果预览", "悬停文档", "gofumpt 格式化"},
	}},
	{id: "toolchain", width: 340, words: map[i18n.Language][]string{
		i18n.English: {"How Lithe finds go", "The order in which Lithe looks for the go program"},
		i18n.Chinese: {"Lithe 按这个顺序找 go", "Lithe 查找 go 程序的顺序"},
	}},
}

// drawnCanvas is the one node a region's view answers with, as Core checked it.
type drawnCanvas struct {
	Type    string            `json:"type"`
	ID      string            `json:"id"`
	Label   string            `json:"label"`
	Height  float64           `json:"height"`
	ViewBox []float64         `json:"viewBox"`
	Pan     json.RawMessage   `json:"pan"`
	On      []string          `json:"on"`
	Nodes   []json.RawMessage `json:"nodes"`
}

// sceneWords are the words of every text node of a scene, nested ones included, a run joined
// into its line. Code drawn a character to a node gives single characters.
func sceneWords(t *testing.T, scene []json.RawMessage) string {
	t.Helper()
	var words []string
	var walk func(nodes []json.RawMessage)
	walk = func(nodes []json.RawMessage) {
		for _, raw := range nodes {
			var node struct {
				Text     string                  `json:"text"`
				Runs     []struct{ Text string } `json:"runs"`
				Children []json.RawMessage       `json:"children"`
			}
			if err := json.Unmarshal(raw, &node); err != nil {
				t.Fatalf("a scene node is not an object: %s", raw)
			}
			line := node.Text
			for _, run := range node.Runs {
				line += run.Text
			}
			if line != "" {
				words = append(words, line)
			}
			walk(node.Children)
		}
	}
	walk(scene)
	return strings.Join(words, "\n")
}

// drawRegion sends one view event to region and returns the canvas it drew.
func drawRegion(t *testing.T, ctx context.Context, sources pluginSettingsSources, editor pluginservice.SettingsEditor, region string, event map[string]any) drawnCanvas {
	t.Helper()
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	drawn, changed, err := sources.View(ctx, editor, region, "settings.view", encoded, nil, false)
	if err != nil {
		t.Fatalf("%s region, %s event: %v", region, event["event"], err)
	}
	if !changed {
		t.Fatalf("%s region, %s event: the view drew nothing new", region, event["event"])
	}
	var nodes []drawnCanvas
	if err := json.Unmarshal(drawn, &nodes); err != nil {
		t.Fatalf("%s region: Core returned %s: %v", region, drawn, err)
	}
	if len(nodes) != 1 || nodes[0].Type != "canvas" {
		t.Fatalf("%s region: want one canvas, got %s", region, drawn)
	}
	return nodes[0]
}

// drawnAt checks that a region's canvas is drawn for a view width points wide: its view box
// is that wide, it shows whole at one unit to a point, and it listens for resize.
func drawnAt(t *testing.T, region string, canvas drawnCanvas, width float64) {
	t.Helper()
	if len(canvas.ViewBox) != 4 || canvas.ViewBox[2] != width || canvas.ViewBox[3] < 1 {
		t.Fatalf("%s region: view box %v, want one %g wide", region, canvas.ViewBox, width)
	}
	if canvas.Height < canvas.ViewBox[3] || len(canvas.Pan) > 0 {
		t.Fatalf("%s region: height %g, pan %s; the scene must show whole at one unit to a point", region, canvas.Height, canvas.Pan)
	}
	if !slices.Contains(canvas.On, "resize") {
		t.Fatalf("%s region: on %v, want resize so the scene follows the window", region, canvas.On)
	}
}

func TestOfficialGoPluginDrawsItsSettingsRegions(t *testing.T) {
	archive := os.Getenv("LITHE_TEST_GO_PLUGIN_ARCHIVE")
	if archive == "" {
		t.Skip("set LITHE_TEST_GO_PLUGIN_ARCHIVE to a go-support-<version>-darwin.zip")
	}
	const id = "io.github.lithe-idea.go-support"
	h := newHarness(t, &testKernel{Client: core.NewClient()})
	installed := installOfficialPluginArchive(t, h, archive, id)
	t.Cleanup(h.app.plugins.Host.StopAll)
	before := packageDigest(t, installed)
	sources := pluginSettingsSources{app: h.app}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	instance := 0
	for _, language := range []i18n.Language{i18n.English, i18n.Chinese} {
		editor, err := sources.Load(ctx, id, language)
		if err != nil {
			t.Fatalf("load the settings page in %s: %v", language.Code(), err)
		}
		for _, region := range goRegions {
			instance++
			// The window's open event carries no settings when nothing is drafted; the
			// host adds the saved ones, read from the installed plugin.
			opened := drawRegion(t, ctx, sources, editor, region.id, map[string]any{
				"event": "open", "instance": "e2e-" + strconv.Itoa(instance), "language": language.Code(),
				"width": region.width,
			})
			drawnAt(t, region.id, opened, region.width)
			words := sceneWords(t, opened.Nodes)
			for _, want := range region.words[language] {
				if !strings.Contains(words, want) && opened.Label != want {
					t.Fatalf("%s region in %s: %q is not drawn; the scene says:\n%s", region.id, language.Code(), want, words)
				}
			}
			draftNote := map[i18n.Language]string{i18n.English: "Includes changes not applied yet", i18n.Chinese: "含未应用的修改"}[language]
			if strings.Contains(words, draftNote) {
				t.Fatalf("%s region: the saved settings are drawn as a draft", region.id)
			}

			// A draft row of the window shows in the preview before it is applied.
			draft := map[string]any{"gofumpt": true}
			if region.id == "toolchain" {
				draft = map[string]any{"goExecutable": "/opt/go/bin/go"}
			}
			drafted := drawRegion(t, ctx, sources, editor, region.id, map[string]any{
				"event": "settings", "instance": "e2e-" + strconv.Itoa(instance), "language": language.Code(),
				"width": region.width, "settings": map[string]any{"effective": map[string]any{}, "draft": draft},
			})
			drawnAt(t, region.id, drafted, region.width)
			draftWords := sceneWords(t, drafted.Nodes)
			if !strings.Contains(draftWords, draftNote) {
				t.Fatalf("%s region in %s: the draft %v is not marked; the scene says:\n%s", region.id, language.Code(), draft, draftWords)
			}
			inUse := map[i18n.Language]string{i18n.English: "In use", i18n.Chinese: "使用中"}[language]
			if region.id == "toolchain" && (strings.Contains(words, inUse) || !strings.Contains(draftWords, inUse)) {
				t.Fatalf("toolchain region in %s: only a set go executable is in use; saved:\n%s\ndrafted:\n%s", language.Code(), words, draftWords)
			}

			// The window widens: the host sends resize with the new width, and the plugin
			// lays the same scene out again at it, the draft still marked.
			const wider = 760
			resized := drawRegion(t, ctx, sources, editor, region.id, map[string]any{
				"event": "resize", "instance": "e2e-" + strconv.Itoa(instance), "language": language.Code(),
				"width": wider, "settings": map[string]any{"effective": map[string]any{}, "draft": draft},
			})
			drawnAt(t, region.id, resized, wider)
			if resizedWords := sceneWords(t, resized.Nodes); !strings.Contains(resizedWords, draftNote) {
				t.Fatalf("%s region in %s: the resized scene lost the draft; it says:\n%s", region.id, language.Code(), resizedWords)
			}
		}
	}
	if h.app.plugins.Host.Running(id) == nil {
		t.Fatal("the backend is not running after drawing the regions")
	}
	h.app.plugins.Host.StopAll()
	assertPackageUnchanged(t, installed, before)
}
