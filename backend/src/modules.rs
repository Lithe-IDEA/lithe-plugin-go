//! Experimental: marks the directory of each Go module below a workspace's
//! root as a source directory (`workspace/setDirectoryMarks`), so a
//! repository that holds several modules shows each one in the project tree.
//!
//! The modules are found with `workspace/findFiles` when the backend starts
//! and whenever the host's workspace contexts change; a `go.mod` created
//! later is marked at the next change. The root's own module is not marked:
//! the host does not mark a workspace's root. The marks live in the host's
//! memory for this backend's session, and the host drops them when the
//! backend stops or the window closes.

use std::collections::{BTreeSet, HashMap, HashSet};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Mutex, MutexGuard};
use std::thread;
use std::time::Duration;

use lithe_plugin_sdk::{BackendHandle, Host, SdkError, WorkspaceContext};
use serde_json::{json, Value};

/// The host method that sets the marks; a host from before it has none.
const SET_DIRECTORY_MARKS: &str = "lithe/host/workspace/setDirectoryMarks";

/// The operation background work reports degradations under.
const NOTIFICATION: &str = "notification";

/// The role every module directory gets: a built-in one, so the host shows
/// it in the interface language.
const ROLE: &str = "sources";

/// The page size `findFiles` accepts at most. It returns at most 200 results
/// across its pages, below the 500 marks one call may set.
const PAGE: u64 = 200;

/// How long the first sync waits for the session, a step at a time: host
/// requests fail until the host has answered `initialize`.
const FIRST_SYNC_INTERVAL: Duration = Duration::from_millis(250);
const FIRST_SYNC_ATTEMPTS: u32 = 40;

/// What the backend last set for a context: its generation and the module
/// directories, sorted.
type Sent = (u64, Vec<String>);

pub struct ModuleMarks {
    handle: BackendHandle,
    /// Held for the whole of a sync, so syncs from the first one and from
    /// context changes take turns.
    sent: Mutex<HashMap<String, Sent>>,
    /// Whether the backend already said on stderr that the host cannot set
    /// marks.
    noted_unsupported: AtomicBool,
}

impl ModuleMarks {
    pub fn new(handle: BackendHandle) -> Self {
        Self {
            handle,
            sent: Mutex::new(HashMap::new()),
            noted_unsupported: AtomicBool::new(false),
        }
    }

    /// Runs the first sync on a thread of its own once the session is
    /// ready. Retries only while the contexts cannot be listed; when they
    /// never can, records `go.modules.contextsUnavailable`.
    pub fn sync_after_initialize(self: &std::sync::Arc<Self>) {
        let marks = std::sync::Arc::clone(self);
        let spawned = thread::Builder::new()
            .name("go-module-marks".into())
            .spawn(move || {
                for attempt in 1..=FIRST_SYNC_ATTEMPTS {
                    thread::sleep(FIRST_SYNC_INTERVAL);
                    match marks.sync() {
                        Ok(()) => return,
                        Err(_) if attempt < FIRST_SYNC_ATTEMPTS => continue,
                        Err(_) => marks.report("go.modules.contextsUnavailable", 1),
                    }
                }
            });
        if spawned.is_err() {
            self.report("go.modules.contextsUnavailable", 1);
        }
    }

    /// Syncs after the host's contexts changed; when they cannot be listed,
    /// records `go.modules.contextsUnavailable`.
    pub fn sync_on_change(&self) {
        if self.sync().is_err() {
            self.report("go.modules.contextsUnavailable", 1);
        }
    }

    /// Lists the contexts again and sets the marks of each that changed.
    /// An error means the contexts could not be listed; failures in one
    /// context are recorded and the others still sync.
    pub fn sync(&self) -> Result<(), SdkError> {
        let mut sent = lock(&self.sent);
        let host = self.handle.host();
        let listed = host.workspace().list_contexts()?;
        // Asked after listing: before the session is ready the SDK answers
        // false instead of failing.
        if !host.supports(SET_DIRECTORY_MARKS)? {
            if !self.noted_unsupported.swap(true, Ordering::Relaxed) {
                eprintln!(
                    "go-support: this Lithe cannot mark directories; Go modules are not marked"
                );
            }
            return Ok(());
        }
        let contexts = listed["contexts"]
            .as_array()
            .map(Vec::as_slice)
            .unwrap_or_default();
        let mut listed_ids = HashSet::new();
        for context in contexts {
            let Ok(reference) = WorkspaceContext::from_value(context) else {
                continue;
            };
            listed_ids.insert(reference.context_id().to_string());
            let Some(root_id) = single_local_root(context) else {
                continue;
            };
            if let Err(failure) = self.sync_context(&host, &reference, root_id, &mut sent) {
                if let Some(code) = failure.code() {
                    self.report(code, failure.count());
                }
            }
        }
        sent.retain(|context_id, _| listed_ids.contains(context_id));
        Ok(())
    }

    fn sync_context(
        &self,
        host: &Host,
        reference: &WorkspaceContext,
        root_id: &str,
        sent: &mut HashMap<String, Sent>,
    ) -> Result<(), Failure> {
        let found = find_go_mods(host, reference, root_id).map_err(Failure::search)?;
        let dirs = module_dirs(found.paths.iter().map(String::as_str));
        if found.truncated {
            self.report("go.modules.truncated", dirs.len() as u64);
        }
        let generation = reference.generation();
        let previous = sent.get(reference.context_id());
        if !needs_update(previous, generation, &dirs) {
            return Ok(());
        }
        let marks: Vec<Value> = dirs
            .iter()
            .map(|path| json!({ "path": path, "role": ROLE }))
            .collect();
        host.workspace()
            .set_directory_marks(reference, json!({ "marks": marks }))
            .map_err(|error| Failure::mark(error, dirs.len()))?;
        sent.insert(reference.context_id().to_string(), (generation, dirs));
        Ok(())
    }

    fn report(&self, code: &str, count: u64) {
        let _ = self
            .handle
            .diagnostics()
            .degraded(NOTIFICATION, code, Some(count));
    }
}

/// A context failure and what it is recorded as.
enum Failure {
    /// The context went away or was replaced while it synced; the change
    /// that replaced it syncs again. Not recorded.
    Expected,
    Recorded(&'static str, u64),
}

impl Failure {
    fn search(error: SdkError) -> Self {
        Self::of(error.code(), "go.modules.searchFailed", 1)
    }

    fn mark(error: SdkError, count: usize) -> Self {
        Self::of(error.code(), "go.modules.markFailed", count as u64)
    }

    /// `host_code` is the host's error code; `code` what the plugin records.
    fn of(host_code: &str, code: &'static str, count: u64) -> Self {
        match host_code {
            // The user did not grant the permission, or the context is gone:
            // both are states the user chose, not faults.
            "plugin.host.contextExpired" | "plugin.host.permissionDenied" => Self::Expected,
            _ => Self::Recorded(code, count),
        }
    }

    fn code(&self) -> Option<&'static str> {
        match self {
            Self::Expected => None,
            Self::Recorded(code, _) => Some(code),
        }
    }

    fn count(&self) -> u64 {
        match self {
            Self::Expected => 0,
            Self::Recorded(_, count) => *count,
        }
    }
}

/// The root of a local context that has exactly one: a mark's path is
/// relative to the context's root and names no root of its own.
fn single_local_root(context: &Value) -> Option<&str> {
    if context["authority"]["kind"].as_str() != Some("local") {
        return None;
    }
    match context["folders"].as_array()?.as_slice() {
        [folder] => folder["rootID"].as_str(),
        _ => None,
    }
}

/// The `go.mod` paths `findFiles` found under a root, and whether it left
/// some out.
struct Found {
    paths: Vec<String>,
    truncated: bool,
}

fn find_go_mods(
    host: &Host,
    reference: &WorkspaceContext,
    root_id: &str,
) -> Result<Found, SdkError> {
    let mut found = Found {
        paths: Vec::new(),
        truncated: false,
    };
    let mut cursor = Value::Null;
    loop {
        let mut params = json!({
            "resource": { "rootID": root_id, "path": "" },
            "query": "go.mod",
            "fileMask": "go.mod",
            "limit": PAGE,
        });
        if !cursor.is_null() {
            params["cursor"] = cursor;
        }
        let page = host.workspace().find_files(reference, params)?;
        found.truncated |= page["isTruncated"].as_bool().unwrap_or(false);
        if let Some(matches) = page["matches"].as_array() {
            found.paths.extend(
                matches
                    .iter()
                    .filter_map(|hit| hit["path"].as_str())
                    .map(str::to_string),
            );
        }
        cursor = page["nextCursor"].clone();
        if !cursor.is_string() {
            return Ok(found);
        }
    }
}

/// The directories of the modules below the root, sorted, from root-relative
/// file paths: each `go.mod`'s parent, except the root itself and the
/// directories the go command skips (`testdata`, `vendor`, and names that
/// start with `.` or `_`).
fn module_dirs<'a>(paths: impl IntoIterator<Item = &'a str>) -> Vec<String> {
    let mut dirs = BTreeSet::new();
    for path in paths {
        let Some((dir, "go.mod")) = path.rsplit_once('/') else {
            continue;
        };
        if dir.is_empty() || !dir.split('/').all(go_builds_into) {
            continue;
        }
        dirs.insert(dir.to_string());
    }
    dirs.into_iter().collect()
}

/// Whether the go command looks inside a directory of this name.
fn go_builds_into(segment: &str) -> bool {
    !segment.is_empty()
        && segment != "testdata"
        && segment != "vendor"
        && !segment.starts_with('.')
        && !segment.starts_with('_')
}

/// Whether the context needs a new call: a generation or modules other than
/// those last set. A context never set needs one only when it has modules;
/// one set before needs one to clear its marks, too.
fn needs_update(previous: Option<&Sent>, generation: u64, dirs: &[String]) -> bool {
    match previous {
        None => !dirs.is_empty(),
        Some((sent_generation, sent_dirs)) => *sent_generation != generation || sent_dirs != dirs,
    }
}

fn lock<T>(mutex: &Mutex<T>) -> MutexGuard<'_, T> {
    mutex
        .lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn dirs(paths: &[&str]) -> Vec<String> {
        module_dirs(paths.iter().copied())
    }

    #[test]
    fn each_nested_go_mod_marks_its_directory_and_the_root_one_none() {
        assert_eq!(
            dirs(&[
                "go.mod",
                "tools/go.mod",
                "services/api/go.mod",
                "services/api/main.go"
            ]),
            ["services/api", "tools"]
        );
    }

    #[test]
    fn only_a_file_named_exactly_go_mod_is_a_module() {
        assert!(dirs(&["a/go.mod.bak", "b/my-go.mod", "c/GO.MOD", "d/go.mod/x"]).is_empty());
    }

    #[test]
    fn directories_the_go_command_skips_are_not_marked() {
        assert_eq!(
            dirs(&[
                "testdata/m/go.mod",
                "pkg/testdata/go.mod",
                "vendor/example.com/m/go.mod",
                ".cache/m/go.mod",
                "_old/go.mod",
                "a/_b/go.mod",
                "kept/go.mod",
            ]),
            ["kept"]
        );
    }

    #[test]
    fn module_directories_are_sorted_without_repeats() {
        assert_eq!(dirs(&["b/go.mod", "a/go.mod", "b/go.mod"]), ["a", "b"]);
    }

    #[test]
    fn a_context_is_set_again_only_when_its_modules_or_generation_change() {
        let modules = vec!["a".to_string()];
        assert!(!needs_update(None, 1, &[]));
        assert!(needs_update(None, 1, &modules));
        let sent = (1, modules.clone());
        assert!(!needs_update(Some(&sent), 1, &modules));
        assert!(needs_update(Some(&sent), 2, &modules));
        assert!(needs_update(Some(&sent), 1, &["b".to_string()]));
        // Marks set before are cleared with an empty call.
        assert!(needs_update(Some(&sent), 1, &[]));
        assert!(!needs_update(Some(&(1, Vec::new())), 1, &[]));
    }

    #[test]
    fn only_a_local_context_with_one_root_is_marked() {
        let context = |kind: &str, folders: Value| json!({ "contextID": "c1", "generation": 1, "authority": { "kind": kind }, "folders": folders });
        let one = json!([{ "rootID": "r1", "name": "app", "path": "/w/app" }]);
        assert_eq!(
            single_local_root(&context("local", one.clone())),
            Some("r1")
        );
        assert_eq!(single_local_root(&context("remote", one)), None);
        let two = json!([{ "rootID": "r1" }, { "rootID": "r2" }]);
        assert_eq!(single_local_root(&context("local", two)), None);
        assert_eq!(single_local_root(&context("local", json!([]))), None);
    }

    #[test]
    fn an_expired_context_or_a_refused_grant_is_not_recorded() {
        for host_code in ["plugin.host.contextExpired", "plugin.host.permissionDenied"] {
            let failure = Failure::of(host_code, "go.modules.searchFailed", 1);
            assert_eq!(failure.code(), None, "{host_code}");
        }
        let failure = Failure::of("plugin.host.limitExceeded", "go.modules.markFailed", 3);
        assert_eq!(failure.code(), Some("go.modules.markFailed"));
        assert_eq!(failure.count(), 3);
    }
}
