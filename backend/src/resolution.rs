//! The Go build page's diagram: the order in which Lithe looks for the `go`
//! program, and which step the `Go executable` row below it decides.
//!
//! The steps follow `toolDefinitions[go].invocation.executables` in
//! `plugin.json`: a set `goExecutable` is the only candidate tried, and an
//! unset one leaves the toolchain's Go, then the automatic lookup. The view
//! sees settings only, so it shows the order, not the program found.
//!
//! The region sits in the build system page's detail pane, 344 points wide
//! in the smallest settings window as measured in the host, so the steps
//! stand one above the other, numbered on a rail, and a unit is a point.

use crate::scene::{group, pair, path, pill, rect, tail, text, text_width, token, Node, CAPTION};
use crate::settings::Settings;
use crate::words::Words;
use serde_json::{json, Value};

pub const CANVAS_ID: &str = "go-resolution";

pub const WIDTH: f64 = 340.0;
const HEADER: f64 = 28.0;
/// The middle of the rail of numbered dots left of the steps.
const RAIL: f64 = 11.0;
const DOT: f64 = 20.0;
const STEP_X: f64 = 30.0;
const STEP_W: f64 = WIDTH - STEP_X;
const STEP_H: f64 = 64.0;
const STEP_GAP: f64 = 10.0;
/// The middle of a step's title row, where its dot sits.
const TITLE_MIDDLE: f64 = 17.0;
/// The top of the row of what the program is used for.
const USES_Y: f64 = HEADER + 3.0 * (STEP_H + STEP_GAP) + 4.0;
pub const HEIGHT: f64 = USES_Y + 22.0;

/// How many characters of the set path a step shows; the end, which names
/// the program, is kept.
const PATH_ROOM: usize = 42;

/// A step's look: the step in force, a step left out, or one in the order.
#[derive(Clone, Copy, PartialEq)]
enum Look {
    InUse,
    Dimmed,
    Plain,
}

/// A step's chip, by its meaning.
#[derive(Clone, Copy)]
enum Chip {
    Success,
    Accent,
    Neutral,
}

struct Step<'a> {
    number: u8,
    title: &'a str,
    lines: [Line<'a>; 2],
    chip: (&'a str, Chip),
    look: Look,
}

enum Line<'a> {
    Words(&'a str, Value),
    Path(String),
}

pub fn canvas(settings: &Settings, words: &Words) -> Value {
    let changed = settings.is_changed("goExecutable");
    let set = !settings.go_executable.is_empty();
    let muted = || token("$textMuted");
    let secondary = || token("$textSecondary");
    let steps = [
        Step {
            number: 1,
            title: words.step_setting,
            lines: if set {
                [
                    Line::Path(tail(&settings.go_executable, PATH_ROOM)),
                    Line::Words(words.step_setting_strict, secondary()),
                ]
            } else {
                [
                    Line::Words(words.step_setting_unset, muted()),
                    Line::Words(words.step_setting_only, secondary()),
                ]
            },
            chip: if set {
                (words.in_use, Chip::Success)
            } else {
                (words.skipped, Chip::Neutral)
            },
            look: if set { Look::InUse } else { Look::Dimmed },
        },
        Step {
            number: 2,
            title: words.step_toolchain,
            lines: [
                Line::Words(words.step_toolchain_detail[0], secondary()),
                Line::Words(words.step_toolchain_detail[1], muted()),
            ],
            chip: if set {
                (words.not_used, Chip::Neutral)
            } else {
                (words.first, Chip::Accent)
            },
            look: if set { Look::Dimmed } else { Look::Plain },
        },
        Step {
            number: 3,
            title: words.step_automatic,
            lines: [
                Line::Words(words.step_automatic_detail[0], secondary()),
                Line::Words(words.step_automatic_detail[1], secondary()),
            ],
            chip: if set {
                (words.not_used, Chip::Neutral)
            } else {
                (words.last, Chip::Neutral)
            },
            look: if set { Look::Dimmed } else { Look::Plain },
        },
    ];

    let mut nodes = vec![text(0.0, 17.0, words.resolution_title, "strong")];
    if changed {
        nodes.push(
            text(WIDTH, 17.0, words.includes_draft, "caption")
                .align("end")
                .color(token("$accent")),
        );
    }
    for (index, step) in steps.iter().enumerate() {
        let y = HEADER + index as f64 * (STEP_H + STEP_GAP);
        if index > 0 {
            // The rail joins the dots; past a step in force the order stops,
            // and the rail into the next step fades with it.
            let above = y - STEP_GAP - STEP_H + TITLE_MIDDLE + DOT / 2.0 + 4.0;
            let below = y + TITLE_MIDDLE - DOT / 2.0 - 4.0;
            let rail = path(format!("M{RAIL} {above} V{below}"))
                .stroke(token("$line"), 2.0)
                .set("cap", "round");
            nodes.push(if step.look == Look::Dimmed {
                rail.opacity(0.5)
            } else {
                rail
            });
        }
        nodes.push(draw(y, step));
    }
    nodes.extend(uses(words).0);
    json!({
        "type": "canvas",
        "id": CANVAS_ID,
        "label": words.resolution_label,
        "height": HEIGHT.ceil() as u64,
        "viewBox": [0, 0, WIDTH, HEIGHT],
        "nodes": nodes.into_iter().map(Value::from).collect::<Vec<_>>(),
    })
}

/// A step at `y`: its numbered dot on the rail and its card.
fn draw(y: f64, step: &Step) -> Node {
    let in_use = step.look == Look::InUse;
    let mut frame = rect(0.0, 0.0, STEP_W, STEP_H).radius(8.0);
    frame = if in_use {
        frame.fill(token("$selection")).stroke(token("$accent"), 1.5)
    } else {
        frame.fill(token("$surface")).stroke(token("$line"), 1.0)
    };
    let (dot, number) = if in_use {
        (token("$accent"), Value::from("#FFFFFF"))
    } else {
        (token("$badge"), token("$textSecondary"))
    };
    let (fill, ink) = match step.chip.1 {
        Chip::Success => (pair("#E6F4EA", "#253A2B"), token("$success")),
        Chip::Accent => (token("$selection"), token("$accent")),
        Chip::Neutral => (token("$badge"), token("$textSecondary")),
    };
    let (_, chip_w) = pill(0.0, 0.0, step.chip.0, fill.clone(), ink.clone());
    let chip_x = STEP_W - 10.0 - chip_w;
    let (chip, _) = pill(chip_x, TITLE_MIDDLE - 10.0, step.chip.0, fill, ink);
    let mut card = vec![
        frame,
        text(12.0, TITLE_MIDDLE + 4.5, step.title, "strong").max_width(chip_x - 12.0 - 10.0),
        chip,
    ];
    for (line, y) in step.lines.iter().zip([40.0, 55.0]) {
        card.push(match line {
            Line::Words(words, color) => text(12.0, y, words, "caption")
                .color(color.clone())
                .max_width(STEP_W - 24.0),
            Line::Path(value) => text(12.0, y, value, "monoSmall")
                .color(token("$text"))
                .max_width(STEP_W - 24.0),
        });
    }
    let step_group = group(
        0.0,
        y,
        vec![
            rect(RAIL - DOT / 2.0, TITLE_MIDDLE - DOT / 2.0, DOT, DOT)
                .radius(DOT / 2.0)
                .fill(dot),
            text(RAIL, TITLE_MIDDLE + 4.0, &step.number.to_string(), "caption")
                .align("center")
                .color(number),
            group(STEP_X, 0.0, card),
        ],
    );
    if step.look == Look::Dimmed {
        step_group.opacity(0.55)
    } else {
        step_group
    }
}

/// What Lithe runs the program for, after the steps, and where the row ends.
fn uses(words: &Words) -> (Vec<Node>, f64) {
    let mut nodes =
        vec![text(0.0, USES_Y + 14.0, words.used_for, "caption").color(token("$textMuted"))];
    let mut x = text_width(words.used_for, CAPTION) + 8.0;
    let mut end = x;
    for word in words.uses {
        let (chip, width) = pill(x, USES_Y, word, token("$badge"), token("$textSecondary"));
        nodes.push(chip);
        end = x + width;
        x = end + 6.0;
    }
    (nodes, end)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::words;

    #[test]
    fn the_row_of_uses_fits_the_width_in_both_languages() {
        for language in ["en", "zh-Hans"] {
            let (_, end) = uses(words::for_language(language));
            assert!(end <= WIDTH, "{language}: {end}");
        }
    }
}
