// trove landing page: language, theme-safe copy, and stable download links.
// No framework, no build step, no third-party requests.

const REPO = "taoquo/trove";
const FILES = {
  "mac-arm64": "trove-macos-arm64.dmg",
  "mac-x64": "trove-macos-x64.dmg",
  windows: "trove-windows-x64-setup.exe",
};

const STRINGS = {
  en: {
    "meta.title": "trove — your bookmarks, finally a library",
    "meta.description":
      "trove pulls your X bookmarks into a local, searchable workspace. Tag, search, and keep them without sending your library anywhere.",
    "nav.features": "Features",
    "nav.privacy": "Privacy",
    "nav.install": "Install",
    "nav.download": "Download",
    "hero.kicker": "Local-first bookmark library",
    "hero.title": "Your bookmarks,<br>finally a library.",
    "hero.lede":
      "trove pulls your X bookmarks into a local, searchable workspace. Tag them, find them, and keep them — without sending your library anywhere.",
    "hero.mac": "Download for macOS",
    "hero.macNote": "Apple silicon & Intel",
    "hero.windows": "Windows",
    "hero.windowsNote": "x64 installer",
    "hero.meta": "macOS 12+ · Windows 10+ · No account, no telemetry",
    "hero.tabAll": "All bookmarks",
    "hero.tabTags": "Tags",
    "hero.tabAuthors": "Authors",
    "hero.footCount": "8 bookmarks",
    "hero.footLocal": "Local · SQLite",
    "statement.line": "The feed forgets. A library remembers.",
    "statement.body":
      "X is good at saving a post for later. It is less good at helping you find it again. trove turns the archive into a place you can browse, search, and arrange — on your own disk.",
    "features.one.title": "Bring the whole archive",
    "features.one.body":
      "Sync every bookmark you have saved on X into one local database. Search the text, the author, the tags, or the date.",
    "features.two.title": "Organize by meaning",
    "features.two.body":
      "A tag library with aliases and parent tags. Filter by author, media, or a saved view. Batch actions when a whole shelf needs moving.",
    "features.three.title": "Let a model do the first pass",
    "features.three.body":
      "Use heuristics or bring your own LLM key. Suggestions wait in a review queue; nothing is applied until you say so.",
    "library.note": "The workspace",
    "library.title": "A workspace, not a timeline.",
    "library.body":
      "Bookmarks, tags, and authors share one quiet surface. Read the post without leaving the library, open a detail card, or move a selection into a new tag. The interface stays out of the way.",
    "library.point1": "Full-text search over posts, authors, and notes",
    "library.point2": "Grid and list views, with saved views for the ways you read",
    "library.point3": "Native detail cards and an image lightbox",
    "tagging.note": "The tag library",
    "tagging.title": "Your vocabulary, not a hashtag soup.",
    "tagging.body":
      "Tags have aliases, parent tags, and a review path. Automatic suggestions are just suggestions: accept the useful ones, rename the near misses, and keep the vocabulary yours.",
    "tagging.point1": "Aliases and parent tags keep related ideas together",
    "tagging.point2": "A review queue for heuristic or model suggestions",
    "tagging.point3": "Batch actions over a selection",
    "privacy.note": "Local-first, by construction",
    "privacy.title": "Nothing leaves this machine.",
    "privacy.body":
      "Your bookmarks live in a SQLite file. X cookies and model keys live in the system's secure store. There is no account, no sync server, and no telemetry.",
    "privacy.one.title": "SQLite on disk",
    "privacy.one.body": "A single local database you can back up, inspect, or move.",
    "privacy.two.title": "System credentials",
    "privacy.two.body":
      "macOS Keychain or Windows Credential Manager. The app only checks presence; it never prints a secret.",
    "privacy.three.title": "Your keys, your choice",
    "privacy.three.body":
      "Automatic tagging is optional. Point it at a provider you trust, or keep it heuristic.",
    "install.note": "Getting started",
    "install.title": "Four steps to a library.",
    "install.one.title": "Download your build",
    "install.one.body":
      "Apple silicon, Intel Mac, or Windows x64. The macOS app is a signed disk image; Windows has an installer and a portable executable.",
    "install.two.title": "Connect X",
    "install.two.body":
      "Paste your <code>auth_token</code> and <code>ct0</code> cookies, or let trove read them from a signed-in browser profile.",
    "install.three.title": "Add a model key, if you want",
    "install.three.body":
      "Automatic tagging works with heuristics alone. An LLM key adds a second pass with your own provider.",
    "install.four.title": "Sync and arrange",
    "install.four.body":
      "Pull the archive once, then search, tag, and keep it current on your own schedule.",
    "download.note": "Download",
    "download.title": "Pick the build for this computer.",
    "download.body":
      "Every build is the same app. Downloads point at the latest GitHub release.",
    "download.macArm": "macOS · Apple silicon",
    "download.macIntel": "macOS · Intel",
    "download.windows": "Windows · x64",
    "download.note2":
      "Requires macOS 12 or later, or Windows 10/11. A GitHub release with these asset names is needed before the links resolve.",
    "footer.line": "Local-first software for people who save too much on X.",
    "footer.releases": "Releases",
    "footer.top": "Back to top",
    "lang.switch": "Switch language",
  },
  zh: {
    "meta.title": "trove — 你的书签，终于像一座图书馆",
    "meta.description":
      "trove 把 X 书签收进一个本地、可搜索的工作区。打标签、找回来、留下来，书库不必离开这台电脑。",
    "nav.features": "功能",
    "nav.privacy": "隐私",
    "nav.install": "安装",
    "nav.download": "下载",
    "hero.kicker": "本地优先的书签库",
    "hero.title": "你的书签，<br>终于像一座图书馆。",
    "hero.lede":
      "trove 把 X 书签收进一个本地、可搜索的工作区。打标签、找回来、留下来——书库不必离开这台电脑。",
    "hero.mac": "下载 macOS 版",
    "hero.macNote": "Apple 芯片与 Intel",
    "hero.windows": "Windows",
    "hero.windowsNote": "x64 安装包",
    "hero.meta": "macOS 12+ · Windows 10+ · 无需账号，无遥测",
    "hero.tabAll": "全部书签",
    "hero.tabTags": "标签",
    "hero.tabAuthors": "作者",
    "hero.footCount": "8 条书签",
    "hero.footLocal": "本地 · SQLite",
    "statement.line": "信息流会忘记，图书馆会记得。",
    "statement.body":
      "X 擅长把帖子存到“以后”，却不擅长让你再找到它。trove 把这份存档变成一个可以浏览、搜索和整理的地方——放在你自己的磁盘上。",
    "features.one.title": "把整座存档带回来",
    "features.one.body":
      "把 X 上保存过的书签同步进一个本地数据库。按正文、作者、标签或日期搜索。",
    "features.two.title": "按意义整理",
    "features.two.body":
      "标签库支持别名和父标签。按作者、媒体或已保存视图筛选；需要时对整批书签执行操作。",
    "features.three.title": "让模型做第一遍",
    "features.three.body":
      "用启发式规则，或接入你自己的 LLM key。建议先进入审核队列；你确认之前不会应用。",
    "library.note": "工作区",
    "library.title": "是工作区，不是时间线。",
    "library.body":
      "书签、标签和作者共享一个安静界面。不用离开书库就能读帖子、打开详情卡，或把一批选择移进新标签。界面不会挡路。",
    "library.point1": "对正文、作者和备注做全文搜索",
    "library.point2": "网格与列表视图，以及按阅读方式保存的视图",
    "library.point3": "原生详情卡与图片灯箱",
    "tagging.note": "标签库",
    "tagging.title": "你的词汇，不是话题标签汤。",
    "tagging.body":
      "标签有别名、父标签和审核路径。自动建议只是建议：留下有用的，改掉差一点的，让词汇始终属于你。",
    "tagging.point1": "别名和父标签把相关概念放在一起",
    "tagging.point2": "启发式或模型建议的审核队列",
    "tagging.point3": "对选区执行批量操作",
    "privacy.note": "本地优先，写在结构里",
    "privacy.title": "什么都不离开这台电脑。",
    "privacy.body":
      "书签在 SQLite 文件里，X cookie 和模型 key 在系统安全存储里。没有账号，没有同步服务器，没有遥测。",
    "privacy.one.title": "磁盘上的 SQLite",
    "privacy.one.body": "一个可备份、可检查、可迁移的本地数据库。",
    "privacy.two.title": "系统凭据",
    "privacy.two.body":
      "macOS Keychain 或 Windows Credential Manager。应用只检查存在性，从不打印密钥。",
    "privacy.three.title": "你的 key，你决定",
    "privacy.three.body":
      "自动打标是可选的。接你信任的供应商，或者只用启发式规则。",
    "install.note": "开始使用",
    "install.title": "四步，建一座书库。",
    "install.one.title": "下载对应版本",
    "install.one.body":
      "Apple 芯片、Intel Mac 或 Windows x64。macOS 是签名磁盘映像；Windows 有安装包和便携版。",
    "install.two.title": "连接 X",
    "install.two.body":
      "粘贴 <code>auth_token</code> 和 <code>ct0</code> cookie，或让 trove 从已登录的浏览器配置读取。",
    "install.three.title": "可选：添加模型 key",
    "install.three.body":
      "只靠启发式规则也能自动打标。LLM key 会增加一遍由你的供应商完成的打标。",
    "install.four.title": "同步并整理",
    "install.four.body":
      "先完整拉取一次存档，然后按自己的节奏搜索、打标和更新。",
    "download.note": "下载",
    "download.title": "选这台电脑对应的版本。",
    "download.body":
      "每个构建都是同一个应用。下载链接指向最新的 GitHub Release。",
    "download.macArm": "macOS · Apple 芯片",
    "download.macIntel": "macOS · Intel",
    "download.windows": "Windows · x64",
    "download.note2":
      "需要 macOS 12 或更高，或 Windows 10/11。链接里的文件名需要先有一次对应的 GitHub Release 才能解析。",
    "footer.line": "给在 X 上收藏太多的人的本地优先软件。",
    "footer.releases": "版本发布",
    "footer.top": "回到顶部",
    "lang.switch": "切换语言",
  },
};

const languageButton = document.querySelector("#lang");
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

  languageButton.textContent = lang === "en" ? "中文" : "English";
  languageButton.setAttribute("aria-label", text("lang.switch"));
}

document.querySelectorAll("[data-download]").forEach((link) => {
  const file = FILES[link.dataset.download];
  if (file) link.href = `https://github.com/${REPO}/releases/latest/download/${file}`;
});

languageButton.addEventListener("click", () => {
  applyLanguage(lang === "en" ? "zh" : "en");
});

applyLanguage(lang);
