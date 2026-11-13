// trove landing page: language switch and release links.
// No framework, no build step, no third-party requests.

const REPO = "taoquo/trove";
const FILES = {
  "mac-arm64": "trove-macos-arm64.dmg",
  "mac-x64": "trove-macos-x64.dmg",
  windows: "trove-windows-x64-setup.exe",
};

const STRINGS = {
  en: {
    "meta.title": "trove — the feed forgets, your library shouldn't",
    "meta.description":
      "trove files your X bookmarks into one local database you own. Sync, tag, search and read offline — no account, no sync server, no telemetry.",
    skip: "Skip to content",
    "nav.system": "What it does",
    "nav.privacy": "Privacy",
    "nav.install": "Install",
    "nav.download": "Download",
    "hero.kicker": "Local-first bookmark library",
    "hero.title": "The feed forgets. Your library shouldn't.",
    "hero.lede":
      "trove takes everything you saved on X — posts, authors, dates, counts — and files it into one database you own. Sync in the background, tag what matters, and find it again next year.",
    "hero.mac": "Download for macOS",
    "hero.macNote": "Apple silicon",
    "hero.windows": "Windows",
    "hero.windowsNote": "x64 installer",
    "hero.meta": "Free. No account, no telemetry — the app never phones home.",
    "hero.caption": "Fig. 1 — The workspace. Accounts and posts in this sample are invented.",
    "statement.line": "A bookmark is a promise you made to your future self.",
    "statement.body":
      "X keeps that promise in a tab you will never open again. trove keeps it in a file you can open, search, back up and read offline — on your own disk, in a format that will outlast the app.",
    "system.note": "What it does",
    "system.title": "Four things, done properly.",
    "system.body":
      "No dashboards, no streaks, no assistant in the corner. One workspace, one detail pane, and a sync that behaves when the network does not.",
    "system.one.name": "Sync",
    "system.one.body":
      "Pull every bookmark from X into local SQLite, with author, date and engagement counts. An interrupted run resumes; it does not start over.",
    "system.two.name": "Tags",
    "system.two.body":
      "Aliases, themes and a review queue. Move a whole shelf into a new tag in one pass, and merge the near-duplicates that accumulate.",
    "system.three.name": "Auto-tag",
    "system.three.body":
      "Keyword heuristics out of the box; bring your own model key for a second pass. Every suggestion waits for your yes.",
    "system.four.name": "Detail",
    "system.four.body":
      "The post, the thread, the date you saved it, your own note — read without leaving the library or loading x.com.",
    "workspace.note": "The workspace",
    "workspace.title": "A reading desk, not a feed.",
    "workspace.body":
      "Three columns: what you saved, what it says, what it belongs to. No infinite scroll, no recommendation, no reason to open the timeline again.",
    "workspace.point1": "Search text, author and tag at the same time",
    "workspace.point2": "Notes travel with the bookmark, not in a separate app",
    "workspace.point3": "Media cached locally, so a deleted post stays readable",
    "workspace.caption": "Fig. 2 — Detail. Illustrative sample.",
    "tagging.note": "The tag library",
    "tagging.title": "Tags you can actually keep.",
    "tagging.body":
      "\"design\" and \"ui\" stop being two shelves. Suggestions come from your own keywords first, never from a model you did not choose.",
    "tagging.point1": "Accept, reject or merge suggestions in a batch",
    "tagging.point2": "Every tag shows its count, so dead shelves are visible",
    "tagging.point3": "Export the whole library as one .trovebundle file",
    "tagging.caption": "Fig. 3 — Tags. Illustrative sample.",
    "privacy.note": "Privacy",
    "privacy.title": "Your reading list is nobody's business.",
    "privacy.body":
      "trove talks to X with your own session cookies and writes to a SQLite file on your disk. There is no trove account, no sync server and no analytics endpoint to opt out of.",
    "privacy.one.title": "Local by default",
    "privacy.one.body":
      "Every bookmark, tag and note lives in one database file. Back it up by copying it.",
    "privacy.two.title": "Credentials in the system vault",
    "privacy.two.body":
      "Cookies go to the macOS Keychain or Windows Credential Manager. The app checks that a key exists; it never prints one.",
    "privacy.three.title": "Nothing to opt out of",
    "privacy.three.body":
      "No telemetry, no crash reporting, no update pings. Downloads come from GitHub releases when you ask.",
    "privacy.sample":
      "The screenshots on this page come from a seeded demo library. Every account, post and tag in them is invented; none belongs to a real X user.",
    "install.note": "Install",
    "install.title": "Two minutes, no account.",
    "install.one.title": "Download",
    "install.one.body": "Pick the build for your platform below. Every build is the same app.",
    "install.two.title": "Open it",
    "install.two.body":
      "macOS: right-click the app and choose Open on first launch. Windows: run the installer.",
    "install.three.title": "Connect X",
    "install.three.body":
      "trove reads the session cookies from a browser you are already signed in to, or you paste <code>auth_token</code> and <code>ct0</code> once.",
    "install.four.title": "Sync",
    "install.four.body":
      "The first full sync runs in the background. The app stays usable while it works.",
    "download.note": "Download",
    "download.title": "Download trove.",
    "download.body":
      "Version 0.3.0, an unsigned development build for macOS 12+ and Windows 10+. Links point at the latest GitHub release.",
    "download.macArm": "macOS, Apple silicon",
    "download.macIntel": "macOS, Intel",
    "download.windows": "Windows, x64",
    "download.get": "Download",
    "download.note2":
      "Unsigned builds: macOS needs right-click → Open on first launch; Windows may show a SmartScreen prompt.",
    "footer.line": "A quiet place for what you saved.",
    "footer.releases": "Releases",
    "footer.top": "Back to top",
  },
  zh: {
    "meta.title": "trove — 信息流会忘记，你的书库不会",
    "meta.description":
      "trove 把 X 书签收进一个属于你的本地数据库。同步、打标、搜索、离线阅读——没有账号，没有同步服务器，没有遥测。",
    skip: "跳到正文",
    "nav.system": "功能",
    "nav.privacy": "隐私",
    "nav.install": "安装",
    "nav.download": "下载",
    "hero.kicker": "本地优先的书签库",
    "hero.title": "信息流会忘记，你的书库不会。",
    "hero.lede":
      "trove 把你在 X 上存下的一切——帖子、作者、日期、计数——收进一个属于你的数据库。后台同步，给值得的东西打上标签，明年还能找回来。",
    "hero.mac": "下载 macOS 版",
    "hero.macNote": "Apple 芯片",
    "hero.windows": "Windows",
    "hero.windowsNote": "x64 安装包",
    "hero.meta": "免费。无需账号，没有遥测——应用从不联网汇报。",
    "hero.caption": "图 1 — 工作区。示例中的账号与帖子均为虚构。",
    "statement.line": "收藏，是你对未来的自己许下的承诺。",
    "statement.body":
      "X 把这份承诺放在一个你永远不会再打开的标签页里。trove 把它放进一个可以打开、搜索、备份、离线阅读的文件——在你自己的磁盘上，格式比应用活得更久。",
    "system.note": "功能",
    "system.title": "四件事，做到位。",
    "system.body":
      "没有仪表盘，没有连续打卡，没有角落里的助手。一个工作区、一个详情面板，和一次讲道理的同步。",
    "system.one.name": "同步",
    "system.one.body":
      "把 X 上的书签全部拉进本地 SQLite，带上作者、日期和互动数。中断的那一轮会接着跑，不必从头再来。",
    "system.two.name": "标签",
    "system.two.body":
      "别名、主题和审核队列。一个操作就能把整排书签挪进新标签，也能合并慢慢堆积的近义词。",
    "system.three.name": "自动打标",
    "system.three.body":
      "默认用关键词启发式；想加一遍，就接你自己的模型 key。每条建议都等你点头。",
    "system.four.name": "详情",
    "system.four.body":
      "帖子、整个讨论串、收藏它的日期、你自己的备注——不必离开书库，也不必打开 x.com。",
    "workspace.note": "工作区",
    "workspace.title": "是阅读桌，不是信息流。",
    "workspace.body":
      "三栏：你存了什么，它写了什么，它属于哪里。没有无限滚动，没有推荐，没有理由再打开时间线。",
    "workspace.point1": "同时搜索正文、作者和标签",
    "workspace.point2": "备注跟着书签走，不放在另一个应用里",
    "workspace.point3": "媒体缓存在本地，原帖删了也还读得到",
    "workspace.caption": "图 2 — 详情。示意样本。",
    "tagging.note": "标签库",
    "tagging.title": "养得住的标签。",
    "tagging.body":
      "「design」和「ui」不再各占一层架子。建议先来自你自己的关键词，而不是一个你没选的模型。",
    "tagging.point1": "批量接受、拒绝或合并建议",
    "tagging.point2": "每个标签都显示数量，冷掉的架子一眼可见",
    "tagging.point3": "整个书库导出一个 .trovebundle 文件",
    "tagging.caption": "图 3 — 标签。示意样本。",
    "privacy.note": "隐私",
    "privacy.title": "你的阅读清单，与别人无关。",
    "privacy.body":
      "trove 用你自己的会话 cookie 和 X 通信，写的是你磁盘上的 SQLite 文件。没有 trove 账号，没有同步服务器，也没有需要你关掉的统计接口。",
    "privacy.one.title": "默认就在本地",
    "privacy.one.body": "每条书签、标签和备注都在一个数据库文件里。复制一份就是备份。",
    "privacy.two.title": "凭据放在系统保险箱",
    "privacy.two.body":
      "cookie 写入 macOS Keychain 或 Windows Credential Manager。应用只检查密钥是否存在，从不把它打印出来。",
    "privacy.three.title": "没有需要退订的东西",
    "privacy.three.body":
      "没有遥测，没有崩溃上报，没有更新推送。要下载时，才去 GitHub Release。",
    "privacy.sample":
      "页面上的截图来自一份虚构的示例书库：其中的账号、帖子和标签都是编的，没有一条属于真实的 X 用户。",
    "install.note": "安装",
    "install.title": "两分钟，不用注册。",
    "install.one.title": "下载",
    "install.one.body": "在下面选对应平台的版本。每个构建都是同一个应用。",
    "install.two.title": "打开",
    "install.two.body": "macOS：首次启动时右键点应用、选「打开」。Windows：运行安装包。",
    "install.three.title": "连接 X",
    "install.three.body":
      "trove 从你已登录的浏览器读取会话 cookie，或者由你粘贴一次 <code>auth_token</code> 和 <code>ct0</code>。",
    "install.four.title": "同步",
    "install.four.body": "首次全量同步在后台进行，期间应用照常可用。",
    "download.note": "下载",
    "download.title": "下载 trove。",
    "download.body":
      "版本 0.3.0，未签名的开发构建，支持 macOS 12+ 与 Windows 10+。链接指向最新的 GitHub Release。",
    "download.macArm": "macOS，Apple 芯片",
    "download.macIntel": "macOS，Intel",
    "download.windows": "Windows，x64",
    "download.get": "下载",
    "download.note2":
      "未签名构建：macOS 首次启动需右键 →「打开」；Windows 可能弹出 SmartScreen 提示。",
    "footer.line": "给存下来的东西，一个安静的地方。",
    "footer.releases": "版本发布",
    "footer.top": "回到顶部",
  },
};

const buttons = {
  en: document.querySelector("#lang-en"),
  zh: document.querySelector("#lang-zh"),
};

let lang = localStorage.getItem("trove-lang");
if (lang !== "en" && lang !== "zh") {
  lang = (navigator.language || "en").toLowerCase().startsWith("zh") ? "zh" : "en";
}

function text(key) {
  return STRINGS[lang][key] || STRINGS.en[key] || "";
}

function applyLanguage(next) {
  lang = next;
  localStorage.setItem("trove-lang", lang);
  document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
  document.title = text("meta.title");

  const description = document.querySelector('meta[name="description"]');
  if (description) description.setAttribute("content", text("meta.description"));

  document.querySelectorAll("[data-i18n]").forEach((element) => {
    const value = text(element.dataset.i18n);
    if (value) element.textContent = value;
  });
  document.querySelectorAll("[data-i18n-html]").forEach((element) => {
    const value = text(element.dataset.i18nHtml);
    if (value) element.innerHTML = value;
  });

  document.querySelectorAll("[data-src-en][data-src-zh]").forEach((image) => {
    image.src = image.dataset[lang === "en" ? "srcEn" : "srcZh"];
  });

  Object.entries(buttons).forEach(([code, button]) => {
    if (!button) return;
    button.setAttribute("aria-pressed", String(code === lang));
  });
}

document.querySelectorAll("[data-download]").forEach((link) => {
  const file = FILES[link.dataset.download];
  if (file) link.href = `https://github.com/${REPO}/releases/latest/download/${file}`;
});

Object.entries(buttons).forEach(([code, button]) => {
  if (button) button.addEventListener("click", () => applyLanguage(code));
});

applyLanguage(lang);
