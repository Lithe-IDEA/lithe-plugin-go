//! Building blocks of the canvas scenes, shaped as `plugin-ui.schema.json`
//! describes them.
//!
//! Coordinates are view-box units. A canvas without `pan` shows its whole
//! view box, scaled to the view's width but never past its `height`, from
//! the top-left corner. Each scene is drawn for the width the host reports
//! and is as tall as its view box, so one unit is one point, and it lists
//! `resize` to be drawn again when the window's width changes. A host that
//! reports no width gets the scene for the smallest settings window, which a
//! wider window shows whole at the same size.

use serde_json::{json, Map, Value};

/// The pixel sizes of the host's `caption` and `strong` text styles at
/// scale 1.
pub const CAPTION: f64 = 11.0;
pub const STRONG: f64 = 13.0;

/// How far one character of `monoSmall` code advances in the scenes.
///
/// A host draws monospace text in its own font, rounded to its screen's
/// pixels, so one character takes 7 points with SF Mono on macOS and 6 with
/// Consolas on Windows. Code that a selection, a caret, a field or a squiggle
/// marks is therefore laid out by [`code`], one character to a node on this
/// grid, which is SF Mono's: the marks then sit on the right characters on
/// every host, and on macOS the code looks as the host itself draws it.
pub const MONO_ADVANCE: f64 = 7.0;

/// The left edge of column `column` of code that starts at `x`.
pub fn column(x: f64, column: usize) -> f64 {
    x + column as f64 * MONO_ADVANCE
}

/// One line of `monoSmall` code from `x`, its baseline at `y`, in pieces of
/// their own colors: a character to a node on the [`MONO_ADVANCE`] grid. A
/// space takes its column and no node.
pub fn code(x: f64, y: f64, pieces: &[(&str, Value)]) -> Vec<Node> {
    let mut nodes = Vec::new();
    let mut index = 0;
    for (piece, color) in pieces {
        for character in piece.chars() {
            if !character.is_whitespace() {
                nodes.push(
                    text(column(x, index), y, &character.to_string(), "monoSmall")
                        .color(color.clone()),
                );
            }
            index += 1;
        }
    }
    nodes
}

/// One canvas node.
pub struct Node(Map<String, Value>);

impl Node {
    fn of(kind: &str) -> Self {
        let mut node = Map::new();
        node.insert("type".into(), kind.into());
        Self(node)
    }

    pub fn set(mut self, key: &str, value: impl Into<Value>) -> Self {
        self.0.insert(key.into(), value.into());
        self
    }

    /// A number field, rounded to a tenth so the scene stays short.
    pub fn at(self, key: &str, value: f64) -> Self {
        self.set(key, round(value))
    }

    pub fn fill(self, color: impl Into<Value>) -> Self {
        self.set("fill", color)
    }

    pub fn stroke(self, color: impl Into<Value>, width: f64) -> Self {
        self.set("stroke", color).at("width", width)
    }

    pub fn radius(self, radius: f64) -> Self {
        self.at("radius", radius)
    }

    pub fn opacity(self, opacity: f64) -> Self {
        self.at("opacity", opacity)
    }

    pub fn color(self, color: impl Into<Value>) -> Self {
        self.set("color", color)
    }

    pub fn max_width(self, width: f64) -> Self {
        self.at("maxWidth", width)
    }

    pub fn align(self, align: &str) -> Self {
        self.set("align", align)
    }
}

impl From<Node> for Value {
    fn from(node: Node) -> Self {
        Value::Object(node.0)
    }
}

fn round(value: f64) -> Value {
    let tenths = (value * 10.0).round();
    if tenths % 10.0 == 0.0 {
        json!((tenths / 10.0) as i64)
    } else {
        json!(tenths / 10.0)
    }
}

pub fn rect(x: f64, y: f64, w: f64, h: f64) -> Node {
    Node::of("rect").at("x", x).at("y", y).at("w", w).at("h", h)
}

/// One line of text with its baseline at `y`.
pub fn text(x: f64, y: f64, value: &str, style: &str) -> Node {
    Node::of("text")
        .at("x", x)
        .at("y", y)
        .set("text", value)
        .set("style", style)
}

/// One line of text in pieces of their own colors.
pub fn runs(x: f64, y: f64, pieces: &[(&str, Value)], style: &str) -> Node {
    let runs: Vec<Value> = pieces
        .iter()
        .map(|(text, color)| json!({ "text": text, "color": color }))
        .collect();
    Node::of("text")
        .at("x", x)
        .at("y", y)
        .set("runs", runs)
        .set("style", style)
}

pub fn path(d: String) -> Node {
    Node::of("path").set("d", d)
}

/// Children placed relative to `(x, y)`.
pub fn group(x: f64, y: f64, children: Vec<Node>) -> Node {
    let children: Vec<Value> = children.into_iter().map(Value::from).collect();
    Node::of("group")
        .at("x", x)
        .at("y", y)
        .set("children", children)
}

/// A color that follows the light and the dark theme.
pub fn pair(light: &str, dark: &str) -> Value {
    json!({ "light": light, "dark": dark })
}

pub fn token(name: &str) -> Value {
    Value::from(name)
}

/// The width text of `size` pixels takes in the host's interface font,
/// estimated, since only the host measures text. Wide characters take a whole
/// em; the estimate errs wide so a pill around the text never clips it.
pub fn text_width(value: &str, size: f64) -> f64 {
    let ems: f64 = value
        .chars()
        .map(|character| match character {
            ' ' => 0.3,
            '\u{2E80}'..='\u{9FFF}' | '\u{F900}'..='\u{FAFF}' | '\u{FF00}'..='\u{FFEF}' => 1.0,
            'A'..='Z' | '0'..='9' | 'm' | 'w' => 0.66,
            'i' | 'j' | 'l' | '.' | ',' | ':' | ';' | '\'' | '|' | '!' => 0.3,
            _ => 0.55,
        })
        .sum();
    ems * size
}

/// A rounded label: its background and its words, centered.
pub fn pill(x: f64, y: f64, words: &str, fill: Value, ink: Value) -> (Node, f64) {
    let width = text_width(words, CAPTION) + 16.0;
    let node = group(
        x,
        y,
        vec![
            rect(0.0, 0.0, width, 20.0).radius(10.0).fill(fill),
            text(width / 2.0, 14.0, words, "caption")
                .align("center")
                .color(ink),
        ],
    );
    (node, width)
}

/// A wavy underline from `x0` to `x1`, its middle at `y`.
pub fn squiggle(x0: f64, x1: f64, y: f64) -> String {
    let mut d = format!("M{} {}", fmt(x0), fmt(y));
    let mut x = x0;
    let mut up = true;
    while x < x1 {
        x = (x + 2.0).min(x1);
        let dy = if up { -1.5 } else { 1.5 };
        d.push_str(&format!(" L{} {}", fmt(x), fmt(y + dy)));
        up = !up;
    }
    d
}

fn fmt(value: f64) -> String {
    round(value).to_string()
}

/// The end of `value` in at most `room` characters, with an ellipsis in
/// front when it is cut: for a path, the end names the program.
pub fn tail(value: &str, room: usize) -> String {
    let count = value.chars().count();
    if count <= room {
        return value.to_string();
    }
    let kept: String = value.chars().skip(count - room + 1).collect();
    format!("…{kept}")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn numbers_are_rounded_to_tenths_and_whole_numbers_stay_integers() {
        let node = Value::from(rect(1.0, 2.25, 3.04, 10.0 / 3.0));
        assert_eq!(node["x"], json!(1));
        assert_eq!(node["y"], json!(2.3));
        assert_eq!(node["w"], json!(3));
        assert_eq!(node["h"], json!(3.3));
    }

    #[test]
    fn wide_characters_take_a_whole_em() {
        assert_eq!(text_width("开启", 10.0), 20.0);
        assert!(text_width("On", 10.0) < text_width("开启", 10.0));
    }

    #[test]
    fn a_long_path_keeps_its_end() {
        assert_eq!(tail("/usr/local/go/bin/go", 30), "/usr/local/go/bin/go");
        let cut = tail("/Users/someone/sdk/go1.23.4/bin/go", 12);
        assert_eq!(cut.chars().count(), 12);
        assert!(cut.starts_with('…') && cut.ends_with("/bin/go"));
    }

    #[test]
    fn code_puts_each_character_on_its_column_and_skips_spaces() {
        let nodes: Vec<Value> = code(10.0, 5.0, &[("a b", token("$text")), ("c", token("$accent"))])
            .into_iter()
            .map(Value::from)
            .collect();
        let placed: Vec<(&str, f64)> = nodes
            .iter()
            .map(|node| (node["text"].as_str().unwrap(), node["x"].as_f64().unwrap()))
            .collect();
        assert_eq!(placed, [("a", 10.0), ("b", 24.0), ("c", 31.0)]);
        assert_eq!(nodes[2]["color"], json!("$accent"));
    }

    #[test]
    fn a_squiggle_ends_where_it_is_asked_to() {
        let d = squiggle(10.0, 15.0, 20.0);
        assert!(d.starts_with("M10 20"));
        assert!(d.ends_with("L15 21.5") || d.ends_with("L15 18.5"), "{d}");
    }
}
