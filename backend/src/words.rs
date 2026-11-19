//! The words the previews show, in the languages the plugin translates its
//! settings to. Setting names match the titles of the declared rows under the
//! region (`plugin.json` and `l10n/zh-Hans.json`), so a card names the row
//! that changes it.
//!
//! Go code and gopls's own output, such as documentation and diagnostics,
//! stay as the tools write them.

pub struct Words {
    pub preview_title: &'static str,
    pub preview_label: &'static str,
    pub follows_options: &'static str,
    pub includes_draft: &'static str,
    pub on: &'static str,
    pub off: &'static str,

    pub hover_title: &'static str,
    pub hover_options: [&'static str; 4],
    /// What each hover option shows, in the order of `hover_options`.
    pub hover_notes: [&'static str; 4],

    pub completion_title: &'static str,
    pub before: &'static str,
    pub after: &'static str,
    pub placeholders_on: &'static str,
    pub placeholders_off: &'static str,

    pub format_title: &'static str,
    pub gofumpt_on: &'static str,
    pub gofumpt_off: &'static str,

    pub staticcheck_title: &'static str,
    pub staticcheck_on: &'static str,
    pub staticcheck_off: &'static str,

    pub resolution_title: &'static str,
    pub resolution_label: &'static str,
    pub step_setting: &'static str,
    pub step_setting_unset: &'static str,
    pub step_setting_only: &'static str,
    pub step_setting_strict: &'static str,
    pub step_toolchain: &'static str,
    pub step_toolchain_detail: [&'static str; 2],
    pub step_automatic: &'static str,
    pub step_automatic_detail: [&'static str; 2],
    pub in_use: &'static str,
    pub skipped: &'static str,
    pub not_used: &'static str,
    pub first: &'static str,
    pub last: &'static str,
    pub used_for: &'static str,
    pub uses: [&'static str; 5],
}

const ZH: Words = Words {
    preview_title: "效果预览",
    preview_label: "gopls 选项的效果预览",
    follows_options: "示例代码，随下方选项即时变化",
    includes_draft: "含未应用的修改",
    on: "开",
    off: "关",

    hover_title: "悬停文档",
    hover_options: ["完整文档", "第一句", "一行", "只显示签名"],
    hover_notes: [
        "签名、完整文档和文档链接",
        "签名和文档的第一句",
        "带包名的一行签名，不显示文档",
        "只显示签名，不显示文档",
    ],

    completion_title: "参数占位符",
    before: "补全前",
    after: "补全后",
    placeholders_on: "补全时填好参数名，按 Tab 跳到下一个",
    placeholders_off: "只补全函数名，参数自己填",

    format_title: "gofumpt 格式化",
    gofumpt_on: "去掉块首空行，八进制写成 0o",
    gofumpt_off: "标准 gofmt 格式",

    staticcheck_title: "Staticcheck 分析",
    staticcheck_on: "另外报告 Staticcheck 发现的问题",
    staticcheck_off: "只报告编译错误和 gopls 默认检查",

    resolution_title: "Lithe 按这个顺序找 go",
    resolution_label: "Lithe 查找 go 程序的顺序",
    step_setting: "本页「Go 程序」",
    step_setting_unset: "未填写",
    step_setting_only: "填写后只用这一个",
    step_setting_strict: "不可用时报错，不往下找",
    step_toolchain: "工具链里的 Go",
    step_toolchain_detail: ["项目选的 Go，其次全局默认", "在「工具链」页选择"],
    step_automatic: "自动查找",
    step_automatic_detail: ["GOROOT → PATH 中的 go", "→ 常见安装位置"],
    in_use: "使用中",
    skipped: "跳过",
    not_used: "不使用",
    first: "先找这里",
    last: "最后",
    used_for: "用于",
    uses: ["读取模块", "构建", "检查", "测试", "运行"],
};

const EN: Words = Words {
    preview_title: "Preview",
    preview_label: "Preview of the gopls options",
    follows_options: "Sample code; follows the options below",
    includes_draft: "Includes changes not applied yet",
    on: "On",
    off: "Off",

    hover_title: "Hover documentation",
    hover_options: ["Full documentation", "First sentence", "One line", "Signature only"],
    hover_notes: [
        "Signature, the whole doc and a link to it",
        "Signature and the doc's first sentence",
        "One line with the package; no documentation",
        "The signature alone; no documentation",
    ],

    completion_title: "Parameter placeholders",
    before: "Typed",
    after: "Completed",
    placeholders_on: "Fills in parameter names; Tab moves to the next",
    placeholders_off: "Completes the name; you type the arguments",

    format_title: "gofumpt formatting",
    gofumpt_on: "No blank line opening a block; octal as 0o",
    gofumpt_off: "Standard gofmt formatting",

    staticcheck_title: "Staticcheck analyses",
    staticcheck_on: "Also reports what Staticcheck finds",
    staticcheck_off: "Only compile errors and gopls's default checks",

    resolution_title: "How Lithe finds go",
    resolution_label: "The order in which Lithe looks for the go program",
    step_setting: "Go executable, below",
    step_setting_unset: "Not set",
    step_setting_only: "When set, only this one",
    step_setting_strict: "If it fails, Lithe stops here",
    step_toolchain: "Go in Toolchains",
    step_toolchain_detail: ["The project's Go, then the default", "Chosen on the Toolchains page"],
    step_automatic: "Automatic",
    step_automatic_detail: ["GOROOT → go on PATH", "→ usual install folders"],
    in_use: "In use",
    skipped: "Skipped",
    not_used: "Not used",
    first: "Tried first",
    last: "Tried last",
    used_for: "Used to",
    uses: ["read the module", "build", "vet", "test", "run"],
};

/// The words for the host's interface language: Chinese for a `zh` tag,
/// English otherwise.
pub fn for_language(tag: &str) -> &'static Words {
    if tag == "zh" || tag.starts_with("zh-") {
        &ZH
    } else {
        &EN
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn a_chinese_tag_of_any_script_or_region_gets_chinese() {
        for tag in ["zh", "zh-Hans", "zh-CN", "zh-Hant-TW"] {
            assert_eq!(for_language(tag).on, ZH.on, "{tag}");
        }
        for tag in ["en", "en-GB", "fr", "zhx", ""] {
            assert_eq!(for_language(tag).on, EN.on, "{tag}");
        }
    }
}
