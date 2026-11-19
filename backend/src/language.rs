//! The Go language page's preview: what each gopls option does to a piece of
//! code, in four cards that follow the rows below them.
//!
//! The scene is as wide as the view the host reports, so a unit is a point:
//! the cards stand two to a row when two of the narrowest fit, else one above
//! the other, and widen with the window. A host that reports no width gets
//! the scene for the region in the smallest settings window, 613 points as
//! measured in the host. The code in the editor-like boxes is laid out on the
//! scene's own monospace grid, so the marks on it line up in every host font.

use crate::scene::{
    code, column, group, pair, path, pill, rect, runs, squiggle, text, text_width, token, Node,
    MONO_ADVANCE, STRONG,
};
use crate::settings::{Hover, Settings};
use crate::words::Words;
use serde_json::{json, Value};

pub const CANVAS_ID: &str = "gopls-preview";

/// The settings the cards show; a draft of any of them marks the preview.
pub const SETTINGS: [&str; 4] = ["hoverKind", "usePlaceholders", "gofumpt", "staticcheck"];

/// The scene's width for a host that reports none.
pub const DEFAULT_WIDTH: f64 = 610.0;
/// The narrowest card: the longest Staticcheck line and its squiggle end 256
/// units into the box, and the pill keeps room beside the longest title. A
/// view narrower than one such card is scaled down by the host.
pub const MIN_CARD_W: f64 = 292.0;
const HEADER: f64 = 28.0;
const GAP: f64 = 10.0;
/// A card's title row, the line under it that says what the value does, and
/// the editor-like box below them.
const TITLE_Y: f64 = 22.0;
const NOTE_Y: f64 = 41.0;
const BOX_X: f64 = 10.0;
const BOX_Y: f64 = 50.0;
/// The box heights of the hover and completion cards and of the gofumpt and
/// Staticcheck ones: the hover's full documentation needs the most.
const BOX_H: [f64; 2] = [124.0, 108.0];
const CARD_H: [f64; 2] = [BOX_Y + BOX_H[0] + BOX_X, BOX_Y + BOX_H[1] + BOX_X];

/// Where the four cards go in a scene of a given width.
pub struct Layout {
    pub width: f64,
    /// Two when two of the narrowest cards fit side by side, else one.
    pub columns: usize,
    card_w: f64,
}

impl Layout {
    pub fn new(width: Option<f64>) -> Self {
        let width = width.unwrap_or(DEFAULT_WIDTH).max(MIN_CARD_W);
        let columns = if width >= 2.0 * MIN_CARD_W + GAP { 2 } else { 1 };
        let card_w = (width - (columns - 1) as f64 * GAP) / columns as f64;
        Self {
            width,
            columns,
            card_w,
        }
    }

    fn box_w(&self) -> f64 {
        self.card_w - 2.0 * BOX_X
    }

    /// The top-left corner of each card, row after row, and the scene's
    /// height under the last row. The cards of a row are one height: the
    /// hover and completion ones, or the gofumpt and Staticcheck ones.
    fn places(&self) -> ([(f64, f64); 4], f64) {
        let mut places = [(0.0, 0.0); 4];
        let mut y = HEADER;
        for (index, place) in places.iter_mut().enumerate() {
            let column = index % self.columns;
            if index > 0 && column == 0 {
                y += CARD_H[(index - 1) / 2] + GAP;
            }
            *place = (column as f64 * (self.card_w + GAP), y);
        }
        (places, y + CARD_H[1])
    }
}

/// Code colors after the IDE themes Lithe's editor follows.
fn keyword() -> Value {
    pair("#0033B3", "#CF8E6D")
}
fn function() -> Value {
    pair("#00627A", "#56A8F5")
}
fn string() -> Value {
    pair("#067D17", "#6AAB73")
}
fn number() -> Value {
    pair("#1750EB", "#2AACB8")
}
fn plain() -> Value {
    token("$text")
}
fn editor() -> Value {
    pair("#FFFFFF", "#1E1F22")
}
fn popup() -> Value {
    pair("#F7F8FA", "#2B2D30")
}

/// One card: the row it follows and what that row's value does.
struct Card<'a> {
    title: &'a str,
    /// The value shown in the pill.
    value: &'a str,
    /// False for a switched-off option, whose pill is neutral.
    lit: bool,
    note: &'a str,
    /// Whether the value is a draft not applied yet.
    changed: bool,
    preview: Vec<Node>,
}

/// The preview for a view `width` points wide, or for the smallest settings
/// window when the host reports no width. A host that reports it sends
/// `resize` with the new width, and the scene is drawn again for it.
pub fn canvas(settings: &Settings, words: &Words, width: Option<f64>) -> Value {
    let layout = Layout::new(width);
    let box_w = layout.box_w();
    let changed = settings.any_changed(&SETTINGS);
    let mut nodes = vec![
        text(0.0, 17.0, words.preview_title, "strong"),
        text(
            layout.width,
            17.0,
            if changed {
                words.includes_draft
            } else {
                words.follows_options
            },
            "caption",
        )
        .align("end")
        .color(token(if changed { "$accent" } else { "$textMuted" }))
        .max_width(layout.width - text_width(words.preview_title, STRONG) - 16.0),
    ];
    let hover = Hover::ALL
        .iter()
        .position(|hover| *hover == settings.hover)
        .expect("every hover kind has words");
    let switch = |on: bool, note_on: &'static str, note_off: &'static str| {
        (
            if on { words.on } else { words.off },
            if on { note_on } else { note_off },
        )
    };
    let (placeholders, placeholders_note) =
        switch(settings.placeholders, words.placeholders_on, words.placeholders_off);
    let (gofumpt, gofumpt_note) = switch(settings.gofumpt, words.gofumpt_on, words.gofumpt_off);
    let (staticcheck, staticcheck_note) =
        switch(settings.staticcheck, words.staticcheck_on, words.staticcheck_off);
    let cards = [
        Card {
            title: words.hover_title,
            value: words.hover_options[hover],
            lit: true,
            note: words.hover_notes[hover],
            changed: settings.is_changed("hoverKind"),
            preview: hover_preview(settings.hover, box_w),
        },
        Card {
            title: words.completion_title,
            value: placeholders,
            lit: settings.placeholders,
            note: placeholders_note,
            changed: settings.is_changed("usePlaceholders"),
            preview: completion_preview(settings.placeholders, words, box_w),
        },
        Card {
            title: words.format_title,
            value: gofumpt,
            lit: settings.gofumpt,
            note: gofumpt_note,
            changed: settings.is_changed("gofumpt"),
            preview: format_preview(settings.gofumpt, box_w),
        },
        Card {
            title: words.staticcheck_title,
            value: staticcheck,
            lit: settings.staticcheck,
            note: staticcheck_note,
            changed: settings.is_changed("staticcheck"),
            preview: staticcheck_preview(settings.staticcheck, box_w),
        },
    ];
    let (places, height) = layout.places();
    for (index, (card, (x, y))) in cards.into_iter().zip(places).enumerate() {
        nodes.push(group(x, y, draw(card, index / 2, layout.card_w)));
    }
    let mut canvas = json!({
        "type": "canvas",
        "id": CANVAS_ID,
        "label": words.preview_label,
        "height": height.ceil() as u64,
        "viewBox": [0, 0, layout.width, height],
        "nodes": nodes.into_iter().map(Value::from).collect::<Vec<_>>(),
    });
    if width.is_some() {
        canvas["on"] = json!(["resize"]);
    }
    canvas
}

/// A card `card_w` wide whose box is `BOX_H[kind]` tall: its title, a pill
/// with the value shown and, when that value is a draft, an accent outline
/// and a dot in front of the pill; the line saying what the value does; then
/// the preview in an editor-like box. A lit pill writes in `$text`: accent ink
/// on the selection fill falls to about 2:1 in Lithe's dark theme.
fn draw(card: Card, kind: usize, card_w: f64) -> Vec<Node> {
    let (fill, ink) = if card.lit {
        (token("$selection"), token("$text"))
    } else {
        (token("$badge"), token("$textSecondary"))
    };
    let (_, width) = pill(0.0, 0.0, card.value, fill.clone(), ink.clone());
    let pill_x = card_w - 12.0 - width;
    let (pill, _) = pill(pill_x, 8.0, card.value, fill, ink);
    let frame = rect(0.0, 0.0, card_w, CARD_H[kind])
        .radius(8.0)
        .fill(token("$surface"));
    let mut nodes = vec![
        if card.changed {
            frame.stroke(token("$accent"), 1.2)
        } else {
            frame.stroke(token("$line"), 1.0)
        },
        text(12.0, TITLE_Y, card.title, "strong").max_width(pill_x - 12.0 - 18.0),
        pill,
        text(12.0, NOTE_Y, card.note, "caption")
            .color(token("$textSecondary"))
            .max_width(card_w - 24.0),
        rect(BOX_X, BOX_Y, card_w - 2.0 * BOX_X, BOX_H[kind])
            .radius(6.0)
            .fill(editor())
            .stroke(token("$line"), 1.0),
        group(BOX_X, BOX_Y, card.preview),
    ];
    if card.changed {
        nodes.push(
            rect(pill_x - 11.0, 15.0, 6.0, 6.0)
                .radius(3.0)
                .fill(token("$accent")),
        );
    }
    nodes
}

/// A hover over `strings.Split` in a box `box_w` wide, with as much
/// documentation as `hover` shows. The documentation is Go's own, which gopls
/// shows as it is, wrapped to the popup.
fn hover_preview(hover: Hover, box_w: f64) -> Vec<Node> {
    const POPUP_X: f64 = 4.0;
    const INSET: f64 = 9.0;
    let popup_w = box_w - 2.0 * POPUP_X;
    let text_w = popup_w - 2.0 * INSET;
    let code_x = 10.0;
    let signature = || {
        runs(
            INSET,
            15.0,
            &[
                ("func ", keyword()),
                ("Split", function()),
                ("(s, sep ", plain()),
                ("string", keyword()),
                (") []", plain()),
                ("string", keyword()),
            ],
            "monoSmall",
        )
    };
    let doc = |y: f64, line: &str| {
        text(INSET, y, line, "caption")
            .color(token("$textSecondary"))
            .max_width(text_w)
    };
    let divider = || rect(INSET, 21.0, text_w, 1.0).fill(token("$line"));
    let synopsis = || {
        [
            doc(34.0, "Split slices s into all substrings separated"),
            doc(47.0, "by sep and returns a slice of the substrings"),
            doc(60.0, "between those separators."),
        ]
    };
    let (height, mut body) = match hover {
        Hover::Full => {
            let mut body = vec![signature(), divider()];
            body.extend(synopsis());
            body.push(doc(73.0, "If s does not contain sep and sep is not…"));
            body.push(
                text(INSET, 87.0, "pkg.go.dev/strings#Split", "caption").color(token("$accent")),
            );
            (94.0, body)
        }
        Hover::Synopsis => {
            let mut body = vec![signature(), divider()];
            body.extend(synopsis());
            (68.0, body)
        }
        Hover::SingleLine => (
            22.0,
            vec![text(
                INSET,
                15.0,
                "func strings.Split(s string, sep string) []string",
                "caption",
            )
            .max_width(text_w)],
        ),
        Hover::Signature => (22.0, vec![signature()]),
    };
    let mut frame = vec![rect(0.0, 0.0, popup_w, height)
        .radius(6.0)
        .fill(popup())
        .stroke(token("$line"), 1.0)
        .set("shadow", 1)];
    frame.append(&mut body);
    let mut nodes = vec![rect(column(code_x, 17) - 1.0, 6.0, 5.0 * MONO_ADVANCE + 2.0, 15.0)
        .radius(2.0)
        .fill(token("$selection"))];
    nodes.extend(code(
        code_x,
        17.0,
        &[
            ("parts := strings.", plain()),
            ("Split", function()),
            ("(line, ", plain()),
            ("\",\"", string()),
            (")", plain()),
        ],
    ));
    nodes.push(group(POPUP_X, 24.0, frame));
    nodes
}

/// Completing `strings.Spl` in a box `box_w` wide, with and without
/// parameter placeholders.
fn completion_preview(placeholders: bool, words: &Words, box_w: f64) -> Vec<Node> {
    let code_x = 10.0;
    let label = |y: f64, words: &str| {
        text(code_x, y, words, "caption")
            .color(token("$textMuted"))
            .max_width(box_w - 2.0 * code_x)
    };
    let caret = |x: f64, y: f64| rect(x, y - 10.5, 1.2, 13.0).fill(plain());
    let list_x = column(code_x, 9);
    let list_w = box_w - list_x - 10.0;
    let mut nodes = vec![label(17.0, words.before)];
    nodes.extend(code(code_x, 35.0, &[("parts := strings.Spl", plain())]));
    nodes.extend([
        caret(column(code_x, 20), 35.0),
        group(
            list_x,
            42.0,
            vec![
                rect(0.0, 0.0, list_w, 22.0)
                    .radius(4.0)
                    .fill(popup())
                    .stroke(token("$line"), 1.0)
                    .set("shadow", 1),
                rect(2.0, 2.0, list_w - 4.0, 18.0)
                    .radius(3.0)
                    .fill(token("$selection")),
                rect(7.0, 6.0, 10.0, 10.0).radius(2.0).fill(function()),
                text(12.0, 14.5, "f", "caption").align("center").color("#FFFFFF"),
                runs(
                    23.0,
                    15.0,
                    &[("Split", plain()), ("(s, sep string)", token("$textMuted"))],
                    "monoSmall",
                )
                .max_width(list_w - 30.0),
            ],
        ),
        label(86.0, words.after),
    ]);
    if placeholders {
        // The placeholders are boxed as the editor boxes a snippet's fields;
        // the first one is selected.
        for (start, length, selected) in [(23, 1, true), (26, 3, false)] {
            let mut field = rect(
                column(code_x, start) - 1.0,
                92.0,
                length as f64 * MONO_ADVANCE + 2.0,
                16.0,
            )
            .radius(3.0)
            .stroke(token("$accent"), 1.0);
            if selected {
                field = field.fill(token("$selection"));
            }
            nodes.push(field);
        }
        nodes.extend(code(
            code_x,
            104.0,
            &[
                ("parts := strings.", plain()),
                ("Split", function()),
                ("(s, sep)", plain()),
            ],
        ));
    } else {
        nodes.extend(code(
            code_x,
            104.0,
            &[
                ("parts := strings.", plain()),
                ("Split", function()),
                ("()", plain()),
            ],
        ));
        nodes.push(caret(column(code_x, 23), 104.0));
    }
    nodes
}

/// The first code line's baseline in the gofumpt and Staticcheck boxes, and
/// the left edge of the code after the line numbers.
const CODE_TOP: f64 = 22.0;
const CODE_X: f64 = 32.0;
const LINE: f64 = 16.0;

/// Code lines with line numbers in a gutter, `LINE` units apart.
fn numbered(lines: Vec<Vec<(&str, Value)>>) -> Vec<Node> {
    let mut nodes = Vec::new();
    for (index, line) in lines.iter().enumerate() {
        let y = CODE_TOP + index as f64 * LINE;
        nodes.push(
            text(CODE_X - 10.0, y, &(index + 1).to_string(), "monoSmall")
                .align("end")
                .color(token("$textMuted")),
        );
        nodes.extend(code(CODE_X, y, line));
    }
    nodes
}

/// The same function as gofmt and as gofumpt format it, in a box `box_w`
/// wide; what gofumpt changes is tinted.
fn format_preview(gofumpt: bool, box_w: f64) -> Vec<Node> {
    let signature = vec![
        ("func ", keyword()),
        ("save", function()),
        ("(p ", plain()),
        ("string", keyword()),
        (") ", plain()),
        ("error", keyword()),
        (" {", plain()),
    ];
    let body = |mode: &'static str| {
        vec![
            ("    ", plain()),
            ("return ", keyword()),
            ("os.Chmod(p, ", plain()),
            (mode, number()),
            (")", plain()),
        ]
    };
    let close = vec![("}", plain())];
    if !gofumpt {
        return numbered(vec![signature, vec![], body("0644"), close]);
    }
    let mut nodes = vec![
        // The line the blank one closed up into, and the new octal literal.
        rect(0.0, CODE_TOP + LINE - 12.0, box_w, LINE)
            .fill(token("$success"))
            .opacity(0.08),
        rect(column(CODE_X, 23) - 2.0, CODE_TOP + LINE - 12.0, 5.0 * MONO_ADVANCE + 4.0, LINE)
            .radius(3.0)
            .fill(token("$success"))
            .opacity(0.2),
    ];
    nodes.extend(numbered(vec![signature, body("0o644"), close]));
    nodes
}

/// A comparison staticcheck's SA6005 reports and gopls's default analyses
/// do not, in a box `box_w` wide. The message is staticcheck's own.
fn staticcheck_preview(staticcheck: bool, box_w: f64) -> Vec<Node> {
    let mut nodes = numbered(vec![
        vec![
            ("func ", keyword()),
            ("same", function()),
            ("(a, b ", plain()),
            ("string", keyword()),
            (") ", plain()),
            ("bool", keyword()),
            (" {", plain()),
        ],
        vec![
            ("    ", plain()),
            ("return ", keyword()),
            ("strings.", plain()),
            ("ToLower", function()),
            ("(a) ==", plain()),
        ],
        vec![
            ("        strings.", plain()),
            ("ToLower", function()),
            ("(b)", plain()),
        ],
        vec![("}", plain())],
    ]);
    if !staticcheck {
        return nodes;
    }
    for (line, start, end) in [(1.0, 11, 32), (2.0, 8, 26)] {
        nodes.push(
            path(squiggle(
                column(CODE_X, start),
                column(CODE_X, end),
                CODE_TOP + line * LINE + 3.0,
            ))
            .stroke(token("$warning"), 1.0),
        );
    }
    let bar_w = box_w - 12.0;
    nodes.push(group(
        6.0,
        CODE_TOP + 3.0 * LINE + 10.0,
        vec![
            rect(0.0, 0.0, bar_w, 22.0)
                .radius(5.0)
                .fill(pair("#FFF4DE", "#3B3124")),
            path("M13 5 L19 16 H7 Z".into()).fill(token("$warning")),
            path("M13 8.6 V12 M13 14 V14.1".into())
                .stroke("#FFFFFF", 1.4)
                .set("cap", "round"),
            runs(
                25.0,
                15.0,
                &[
                    ("SA6005", token("$text")),
                    (": should use strings.EqualFold instead", token("$textSecondary")),
                ],
                "caption",
            )
            .max_width(bar_w - 31.0),
        ],
    ));
    nodes
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::scene::CAPTION;
    use crate::words;

    #[test]
    fn every_title_keeps_its_room_beside_its_pill_in_the_narrowest_card() {
        for language in ["en", "zh-Hans"] {
            let words = words::for_language(language);
            let hover = words
                .hover_options
                .iter()
                .map(|value| (words.hover_title, *value));
            let switches = [words.completion_title, words.format_title, words.staticcheck_title]
                .into_iter()
                .flat_map(|title| [(title, words.on), (title, words.off)]);
            for (title, value) in hover.chain(switches) {
                // As `draw` places them: the pill 12 units from the card's
                // right edge, the title from 12 units and 18 short of the pill.
                let pill_x = MIN_CARD_W - 12.0 - (text_width(value, CAPTION) + 16.0);
                let room = pill_x - 12.0 - 18.0;
                let title_w = text_width(title, STRONG);
                assert!(title_w <= room, "{language}: {title} ({title_w}) beside {value} ({room})");
            }
        }
    }
}
