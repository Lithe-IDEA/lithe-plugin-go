//! The values the previews show, read from a view event.
//!
//! A preview follows the settings window as the user edits it, so a value the
//! user changed but has not applied (the draft) wins over the saved one, and a
//! setting with neither shows the default `plugin.json` declares.

use lithe_plugin_sdk::view::ViewEvent;
use serde_json::Value;

/// How much documentation a gopls hover shows: the `hoverKind` setting.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Hover {
    Full,
    Synopsis,
    SingleLine,
    Signature,
}

impl Hover {
    pub const ALL: [Hover; 4] = [
        Hover::Full,
        Hover::Synopsis,
        Hover::SingleLine,
        Hover::Signature,
    ];

    fn parse(value: &str) -> Option<Self> {
        Some(match value {
            "FullDocumentation" => Hover::Full,
            "SynopsisDocumentation" => Hover::Synopsis,
            "SingleLine" => Hover::SingleLine,
            "NoDocumentation" => Hover::Signature,
            _ => return None,
        })
    }

    pub fn value(self) -> &'static str {
        match self {
            Hover::Full => "FullDocumentation",
            Hover::Synopsis => "SynopsisDocumentation",
            Hover::SingleLine => "SingleLine",
            Hover::Signature => "NoDocumentation",
        }
    }
}

/// The plugin's settings as the settings window holds them now.
#[derive(Debug)]
pub struct Settings {
    pub hover: Hover,
    pub placeholders: bool,
    pub gofumpt: bool,
    pub staticcheck: bool,
    /// The `goExecutable` path; empty when it is not set.
    pub go_executable: String,
    /// The settings whose draft differs from the value in force.
    changed: Vec<&'static str>,
    /// Values of the wrong type or outside the declared options, shown as the
    /// default instead.
    pub unrecognized: u64,
}

impl Settings {
    pub fn read(event: &ViewEvent) -> Self {
        let mut reader = Reader {
            event,
            changed: Vec::new(),
            unrecognized: 0,
        };
        let hover = reader.read("hoverKind", Value::from("FullDocumentation"), |value| {
            value.as_str().and_then(Hover::parse)
        });
        let placeholders = reader.boolean("usePlaceholders");
        let gofumpt = reader.boolean("gofumpt");
        let staticcheck = reader.boolean("staticcheck");
        let go_executable = reader.read("goExecutable", Value::from(""), |value| {
            value.as_str().map(|path| path.trim().to_string())
        });
        Self {
            hover,
            placeholders,
            gofumpt,
            staticcheck,
            go_executable,
            changed: reader.changed,
            unrecognized: reader.unrecognized,
        }
    }

    /// Whether the user changed `id` without applying it yet.
    pub fn is_changed(&self, id: &str) -> bool {
        self.changed.contains(&id)
    }

    pub fn any_changed(&self, ids: &[&str]) -> bool {
        ids.iter().any(|id| self.is_changed(id))
    }
}

struct Reader<'a> {
    event: &'a ViewEvent,
    changed: Vec<&'static str>,
    unrecognized: u64,
}

impl Reader<'_> {
    fn read<T>(
        &mut self,
        id: &'static str,
        default: Value,
        parse: impl Fn(&Value) -> Option<T>,
    ) -> T {
        let saved = self.event.effective.get(id).unwrap_or(&default);
        if let Some(draft) = self.event.draft.get(id) {
            if draft != saved {
                self.changed.push(id);
            }
        }
        let shown = self.event.draft.get(id).unwrap_or(saved);
        match parse(shown) {
            Some(value) => value,
            None => {
                self.unrecognized += 1;
                parse(&default).expect("the declared default parses")
            }
        }
    }

    fn boolean(&mut self, id: &'static str) -> bool {
        self.read(id, Value::Bool(false), Value::as_bool)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    pub fn event(effective: Value, draft: Value) -> ViewEvent {
        ViewEvent::parse(&json!({
            "event": "settings",
            "instance": "i1",
            "surface": { "settingRegionID": "language" },
            "language": "en",
            "settings": { "effective": effective, "draft": draft },
        }))
        .unwrap()
    }

    #[test]
    fn a_setting_without_a_value_shows_its_default() {
        let settings = Settings::read(&event(json!({}), json!({})));
        assert_eq!(settings.hover, Hover::Full);
        assert!(!settings.placeholders && !settings.gofumpt && !settings.staticcheck);
        assert_eq!(settings.go_executable, "");
        assert_eq!(settings.unrecognized, 0);
        assert!(!settings.any_changed(&["hoverKind", "gofumpt", "goExecutable"]));
    }

    #[test]
    fn the_draft_wins_over_the_saved_value_and_counts_as_a_change() {
        let settings = Settings::read(&event(
            json!({ "gofumpt": true, "hoverKind": "SingleLine", "staticcheck": true }),
            json!({ "gofumpt": false, "staticcheck": true, "goExecutable": " /opt/go/bin/go " }),
        ));
        assert!(!settings.gofumpt);
        assert_eq!(settings.hover, Hover::SingleLine);
        assert_eq!(settings.go_executable, "/opt/go/bin/go");
        assert!(settings.is_changed("gofumpt"));
        assert!(settings.is_changed("goExecutable"));
        // A draft equal to the value in force changes nothing.
        assert!(!settings.is_changed("staticcheck"));
        assert!(!settings.is_changed("hoverKind"));
    }

    #[test]
    fn a_value_outside_the_declaration_shows_the_default_and_is_counted() {
        let settings = Settings::read(&event(
            json!({ "hoverKind": "Structured", "gofumpt": "yes" }),
            json!({}),
        ));
        assert_eq!(settings.hover, Hover::Full);
        assert!(!settings.gofumpt);
        assert_eq!(settings.unrecognized, 2);
    }

    #[test]
    fn every_hover_kind_reads_back_from_its_value() {
        for hover in Hover::ALL {
            let settings = Settings::read(&event(json!({ "hoverKind": hover.value() }), json!({})));
            assert_eq!(settings.hover, hover);
        }
    }
}
