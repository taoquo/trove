package i18n

// The window's own words: the menu bar, the toolbar, the results header, the card
// and batch bar, the empty states and the media preview.

const (
	AppName Key = "app.name"

	MenuFile           Key = "menu.file"
	MenuEdit           Key = "menu.edit"
	MenuView           Key = "menu.view"
	MenuWindow         Key = "menu.window"
	BundleExportTitle  Key = "bundle.exportTitle"
	BundleExportLegacy Key = "bundle.exportLegacy"
	BundleExported     Key = "bundle.exported"
	// The in-flight sentences for the file commands. Each names what is happening so
	// the toolbar line is not empty while work runs off the window's thread.
	BundleChoosingWhere Key = "bundle.choosingWhere"
	BundleExporting     Key = "bundle.exporting"
	BundleReading       Key = "bundle.reading"
	BundleImporting     Key = "bundle.importing"
	BundleImportTitle   Key = "bundle.importTitle"
	BundleRestoreTitle  Key = "bundle.restoreTitle"
	BundleImportBody    Key = "bundle.importBody"
	BundleReport        Key = "bundle.report"
	ActionImport        Key = "action.import"
	ActionRestore       Key = "action.restore"

	MenuExport Key = "menu.export"
	// MenuAddBookmark opens the add form. A page-based client saves from x.com itself and has
	// no such command; this app has no page, so it needs an entry point of its own -- and the
	// sidebar header has no room for one, so it lives in the menu bar instead.
	MenuAddBookmark Key = "menu.addBookmark"
	MenuImport      Key = "menu.import"
	// MenuCheckUpdates opens MyGo's update window. It is in the app menu beside About,
	// where macOS users look for it, and it also works when the background check is off.
	MenuCheckUpdates Key = "menu.checkUpdates"

	ToolbarGraph Key = "toolbar.graph"
	// ViewGrid and ViewList name the toolbar's two icon-only view toggles. The toggles carry
	// no visible text, so these names are the shell's own copy.
	ViewGrid               Key = "view.grid"
	ViewList               Key = "view.list"
	GraphSummary           Key = "graph.summary"
	GraphHidden            Key = "graph.hidden"
	GraphAuthorNodesSuffix Key = "graphAuthorNodesSuffix"
	GraphAuthorsSuffix     Key = "graphAuthorsSuffix"
	GraphEmptyTitle        Key = "graphEmptyTitle"
	// GraphCanvas names the graph's drawing surface for a screen reader: the dots are drawn,
	// not elements, so the canvas is the only thing a reader can land on.
	GraphCanvas Key = "graphCanvas"
	// GraphLegend says what the two marks encode: a dot's size is the tag's bookmarks, a
	// line's weight is how often the two tags appear together. This build's addition -- the
	// source draws both encodings without saying so, and a map without a key is a picture.
	GraphLegend Key = "graphLegend"
	// GraphFocusPanel names the card the focused tag's numbers are shown on, so assistive
	// technology reads the panel as one thing rather than as three loose lines.
	GraphFocusPanel       Key = "graphFocusPanel"
	GraphEmptyDescription Key = "graphEmptyDescription"
	// GraphBuildingTitle stands in for the empty state while the graph is being built:
	// the read walks the whole library, so the first frame after the switch has nothing
	// to draw yet and "no relations" would be a lie.
	GraphBuildingTitle Key = "graphBuildingTitle"
	GraphShowAuthors   Key = "graphShowAuthors"
	GraphClearFocus    Key = "graphClearFocus"
	GraphOpenBookmarks Key = "graphOpenBookmarks"
	// SearchLabel names the magnifier that opens the sidebar's search field. The field is closed
	// by default, so this string is the only thing that says the search exists.
	SearchLabel         Key = "searchLabel"
	BookmarksCountLabel Key = "bookmarksCountLabel"
	ToolbarCount        Key = "toolbar.count"
	// The key is namespaced: a key whose text equals the key cannot be told apart from
	// a missing translation, and an existing test refuses that shape.
	ResultsLabel Key = "toolbar.results"
	// ResultsLoadingMore is the line at the end of the grid while the next page is
	// read. The action it replaces ("load more") is gone: the scroll performs it.
	ResultsLoadingMore Key = "toolbar.loadingMore"
	// ToolbarSearch holds the search-box placeholder. MyGo's SearchField takes no
	// placeholder, so nothing renders it -- but a key holding the string is what the other
	// count measures. The two counts pull in opposite directions here, and this is the side
	// that keeps the wording.
	ToolbarSearch      Key = "searchPlaceholder"
	ActiveFiltersLabel Key = "activeFilters"
	ToolbarFilters     Key = "filters"
	FilterConditions   Key = "filterConditions"
	FilterDescription  Key = "filterDescription"
	FilterHasMedia     Key = "hasMedia"
	FilterHasVideo     Key = "hasVideo"
	FilterArchived     Key = "archived"
	// FilterArchivedWithCount is the same row once the count is known. It is a second
	// key rather than a suffix because the number goes inside the label, and Chinese
	// brackets are not the English ones.
	FilterArchivedWithCount Key = "archived.withCount"
	ToolbarShowingOf        Key = "toolbar.showingOf"
	// ToolbarRefreshing is the quiet note beside the count while a background refresh
	// runs. The page stays on screen, so without it a click on a filter looks like it
	// did nothing until the new page lands.
	ToolbarRefreshing Key = "toolbar.refreshing"
	// ToolbarSearching is the note between a keystroke and the read it asks for. The
	// field waits a moment before reading, so without it the list looks deaf while the
	// reader types.
	ToolbarSearching Key = "toolbar.searching"
	FilterLongform   Key = "filter.longform"
	FilterClearAll   Key = "filter.clearAll"

	SortTimeline    Key = "sort.timeline"
	SortSavedDesc   Key = "sort.savedDesc"
	SortSavedAsc    Key = "sort.savedAsc"
	SortCreatedDesc Key = "sort.createdDesc"
	SortLikesDesc   Key = "sort.likesDesc"

	LoadingFeedTitle   Key = "loadingFeedTitle"
	LoadFailedTitle    Key = "loadFailedTitle"
	InfoLabel          Key = "infoLabel"
	AddBookmark        Key = "addBookmark"
	AddLinkPlaceholder Key = "addLinkPlaceholder"
	AddTextPlaceholder Key = "addTextPlaceholder"
	AddNotAPostLink    Key = "addNotAPostLink"
	ActionRetry        Key = "retryLoad"
	EmptyLibrary       Key = "empty.library"
	EmptyLibraryDetail Key = "emptyFeedDescription"
	// EmptyLibraryConnect is the first run's one action: with no session stored, the
	// way into an empty library is the browser read, not a sync that would fail.
	EmptyLibraryConnect Key = "empty.libraryConnect"
	EmptyFilterDetail   Key = "noBookmarksDescription"
	EmptyFilter         Key = "empty.filter"
	// EmptyArchive is the archive's own empty state. "No bookmarks match the current
	// filters" is advice for a narrowed list; an empty archive is a place that has not
	// been used yet, and the archive is itself one of the filters it would blame.
	EmptyArchive       Key = "empty.archive"
	EmptyArchiveDetail Key = "empty.archiveDescription"
	// ResultsListLabel names the results list's single keyboard stop. The cards are no
	// longer stops of their own: a page of 80 cards put 80 Tab presses between the
	// toolbar and the status line, and one stop plus the arrows is how a list is walked.
	ResultsListLabel Key = "results.list"

	ActionSync Key = "action.sync"
	// The scroll-ends capsule's two controls. They are the shell's own copy, in both
	// catalogs.
	ActionScrollTop    Key = "action.scrollTop"
	ActionScrollBottom Key = "action.scrollBottom"
	ActionCancel       Key = "action.cancel"
	ActionAllow        Key = "action.allow"
	ActionDelete       Key = "action.delete"
	ActionRemove       Key = "action.remove"
	// ActionRemoveNamed is ActionRemove with the thing it removes. A row of tag pills ends with
	// one remove control each, so six pills read out as six "Remove"s in a row; a single pill
	// is one button whose text is the tag, so its name carries the tag.
	ActionRemoveNamed Key = "action.removeNamed"
	// ActionOpenNamed, ActionRenameNamed and ActionDeleteNamed are the row actions of a
	// management table: several rows carry the same icon, and a generic name gives a screen
	// reader the same words once per row with nothing saying which row it acts on.
	ActionOpenNamed      Key = "action.openNamed"
	ActionRenameNamed    Key = "action.renameNamed"
	ActionDeleteNamed    Key = "action.deleteNamed"
	ActionSettings       Key = "action.settings"
	ActionAddTag         Key = "action.addTag"
	ActionMoveToList     Key = "action.moveToList"
	ActionReview         Key = "action.reviewSuggestions"
	ActionRunAutotag     Key = "action.runAutotag"
	ActionClearSelection Key = "action.clearSelection"
	BatchSelected        Key = "batch.selected"
	BatchSelectCurrent   Key = "batch.selectCurrent"
	BatchSelectLoaded    Key = "batch.selectLoaded"
	BatchApplyTag        Key = "batch.applyTag"
	BatchTooMany         Key = "batch.tooMany"
	// BatchChooseTag is the empty value of the bulk tag picker, and BatchChooseList is this
	// build's own for the move picker it adds. Both are what the trigger shows before
	// anything is chosen: a picker that draws nothing reads as broken.
	BatchChooseTag  Key = "batch.chooseTag"
	BatchChooseList Key = "batch.chooseList"
	// BatchTagged and BatchMoved are the two bulk writes' result sentences. The bar stays
	// standing after a write, so without them a completed action and a missed press look
	// the same.
	BatchTagged          Key = "batch.tagged"
	BatchTaggedOne       Key = "batch.taggedOne"
	BatchMoved           Key = "batch.moved"
	BatchMovedOne        Key = "batch.movedOne"
	ExportData           Key = "exportData"
	ExportingData        Key = "exporting"
	ActionAdd            Key = "action.add"
	ActionSave           Key = "action.save"
	ActionTestConnection Key = "action.testConnection"
	// SearchPlaceholder is the toolbar field's prompt.
	SearchPlaceholder        Key = ToolbarSearch
	SearchAuthors            Key = "searchAuthors"
	SearchAuthorsPlaceholder Key = "searchAuthorsPlaceholder"
	ActionSaveCookies        Key = "action.saveCookies"
	ActionReconnectX         Key = "action.reconnectX"
	ActionRunNow             Key = "settingsRunAutoTagNow"
	ActionStopRun            Key = "settingsAutoTagCancel"
	ActionClear              Key = "action.clear"
	ActionLoadMore           Key = "action.loadMore"
	ActionShowMore           Key = "action.showMore"
	ActionFocusSearch        Key = "action.focusSearch"

	AppearanceSystem Key = "appearance.system"
	AppearanceLight  Key = "appearance.light"
	AppearanceDark   Key = "appearance.dark"
	ActionCopyLink   Key = "copyLink"
	// ActionCopied is the copy chip's own confirmation, shown in its place for a moment
	// after the link is on the clipboard.
	ActionCopied Key = "copied"
	// The row menu's own words.
	ActionOpenBookmark   Key = "action.openBookmark"
	ActionCopyText       Key = "action.copyText"
	ActionAddToSelection Key = "action.addToSelection"
	// ActionRemoveFromSelection is the same row of the menu once the bookmark is ticked.
	ActionRemoveFromSelection Key = "action.removeFromSelection"
	// ActionArchive and ActionUnarchive name the state the bookmark is about to be in, so
	// the menu row in the archive reads as the way back out.
	ActionArchive         Key = "action.archive"
	ActionUnarchive       Key = "action.unarchive"
	ActionFilterTag       Key = "action.filterTag"
	ActionOnlyTag         Key = "action.onlyTag"
	ActionFilterAuthor    Key = "action.filterAuthor"
	ActionOnlyAuthor      Key = "action.onlyAuthor"
	ActionCopyHandle      Key = "action.copyHandle"
	ActionCollapseSidebar Key = "action.collapseSidebar"
	ActionExpandSidebar   Key = "action.expandSidebar"
	ActionToggleSidebar   Key = "action.toggleSidebar"
	OpenOnX               Key = "openOnX"
	MediaPrevious         Key = "previousMedia"
	MediaNext             Key = "nextMedia"
	MediaClose            Key = "closePreview"
	// MediaPlay is the video poster's play control in the preview, and MediaPlayGif the same
	// control over an animated GIF, which X serves as an mp4 too and which is shown as video.
	MediaPlay    Key = "playMedia"
	MediaPlayGif Key = "playGif"
	// MediaTitle names the preview dialog.
	MediaTitle Key = "mediaTitle"
	// ActionUndo is the toast's one button: archiving is reversible, and the receipt is
	// where the way back belongs while the reader still knows what they just did.
	ActionUndo Key = "action.undo"
	// ActionOpenSyncSettings is the next step under a sync failure. The sentence names
	// Settings > Sync; this is the control that goes there.
	ActionOpenSyncSettings Key = "action.openSyncSettings"
)

var shellMessages = messages{
	EN: {
		AppName:                   "trove",
		MenuFile:                  "File",
		MenuEdit:                  "Edit",
		MenuView:                  "View",
		MenuWindow:                "Window",
		BundleExportTitle:         "Export backup",
		BundleExportLegacy:        "Export legacy backup",
		BundleExported:            "Backup exported",
		BundleChoosingWhere:       "Choosing where to save…",
		BundleExporting:           "Exporting the library…",
		BundleReading:             "Reading the file…",
		BundleImporting:           "Importing…",
		BundleImportTitle:         "Import backup",
		BundleRestoreTitle:        "Restore backup",
		BundleImportBody:          "%d new, %d already saved",
		BundleReport:              "%d added · %d updated · %d tags · %d raw payloads",
		ActionImport:              "Import",
		ActionRestore:             "Restore",
		MenuExport:                "Export…",
		MenuAddBookmark:           "Add Bookmark…",
		MenuImport:                "Import…",
		MenuCheckUpdates:          "Check for Updates…",
		ToolbarGraph:              "Graph",
		ViewGrid:                  "Grid view",
		ViewList:                  "List view",
		GraphSummary:              "%d tags \u00b7 %d links",
		GraphHidden:               "%d hidden",
		GraphAuthorNodesSuffix:    "author nodes",
		GraphAuthorsSuffix:        "authors",
		GraphEmptyTitle:           "No relations to show yet",
		GraphCanvas:               "Tag relation graph",
		GraphLegend:               "A bigger dot is a tag with more bookmarks · a heavier line is a pair that appears together more often",
		GraphFocusPanel:           "Focused tag",
		GraphEmptyDescription:     "Tag a few bookmarks, or widen the current filters.",
		GraphBuildingTitle:        "Building the relation graph…",
		GraphShowAuthors:          "Show authors",
		GraphClearFocus:           "Clear focus",
		GraphOpenBookmarks:        "View bookmarks",
		SearchLabel:               "Search",
		BookmarksCountLabel:       "bookmarks",
		ToolbarCount:              "%d",
		ResultsLabel:              "results",
		ResultsLoadingMore:        "Loading more...",
		ToolbarSearch:             "Search bookmarks, authors and notes...",
		ActiveFiltersLabel:        "Active filters:",
		ToolbarFilters:            "Filters",
		FilterConditions:          "Filter conditions",
		FilterDescription:         "Choose the filters to apply.",
		FilterHasMedia:            "Has media",
		FilterHasVideo:            "Has video",
		FilterArchived:            "Archived",
		FilterArchivedWithCount:   "Archived (%d)",
		ToolbarShowingOf:          "%d / %d",
		ToolbarRefreshing:         "Updating…",
		ToolbarSearching:          "Searching…",
		FilterLongform:            "Longform",
		FilterClearAll:            "Clear all",
		SortTimeline:              "Recently saved",
		SortSavedDesc:             "Recently saved",
		SortSavedAsc:              "Oldest saved",
		SortCreatedDesc:           "Newest published",
		SortLikesDesc:             "Most liked",
		LoadingFeedTitle:          "Loading your bookmark feed",
		LoadFailedTitle:           "Failed to load bookmarks",
		InfoLabel:                 "Info",
		AddBookmark:               "Add bookmark",
		AddLinkPlaceholder:        "Paste a link to the post",
		AddTextPlaceholder:        "Post text (optional)",
		AddNotAPostLink:           "That is not a link to an X post.",
		ActionRetry:               "Retry load",
		EmptyLibrary:              "Your library is empty",
		EmptyLibraryDetail:        "trove copies the bookmarks you already saved on X. Sign in to x.com in your browser, then connect — or import a backup you already have.",
		EmptyLibraryConnect:       "Connect X",
		EmptyFilterDetail:         "Try adjusting your search, tags, or author scope.",
		EmptyFilter:               "No bookmarks match the current filters",
		EmptyArchive:              "The archive is empty",
		EmptyArchiveDetail:        "Bookmarks you archive leave the library and wait here, with their tags and their list. Use the row menu to archive one, or select several and use the bar above the list.",
		ResultsListLabel:          "Bookmarks list",
		ActionSync:                "Sync now",
		ActionScrollTop:           "Scroll to top",
		ActionScrollBottom:        "Scroll to bottom",
		ActionCancel:              "Cancel",
		ActionAllow:               "Allow",
		ActionDelete:              "Delete",
		ActionRemove:              "Remove",
		ActionRemoveNamed:         "Remove %s",
		ActionOpenNamed:           "Open %s",
		ActionRenameNamed:         "Rename %s",
		ActionDeleteNamed:         "Delete %s",
		ActionSettings:            "Settings",
		ActionAddTag:              "Add tag",
		ActionMoveToList:          "Move to list",
		ActionReview:              "Tidy vocabulary",
		ActionRunAutotag:          "Run auto-tagging",
		ActionClearSelection:      "Clear selection",
		BatchSelected:             "%d selected",
		BatchSelectCurrent:        "Select current results",
		BatchSelectLoaded:         "Select loaded",
		BatchApplyTag:             "Apply tag",
		BatchTooMany:              "Too many results to select at once",
		BatchChooseTag:            "Choose tag",
		BatchChooseList:           "Choose list",
		BatchTagged:               "Tagged %d bookmarks",
		BatchTaggedOne:            "Tagged 1 bookmark",
		BatchMoved:                "Moved %d bookmarks to %s",
		BatchMovedOne:             "Moved 1 bookmark to %s",
		ExportData:                "Export data",
		ExportingData:             "Exporting...",
		ActionAdd:                 "Add",
		ActionSave:                "Save",
		ActionTestConnection:      "Test connection",
		SearchAuthors:             "Search authors",
		SearchAuthorsPlaceholder:  "Search authors...",
		ActionSaveCookies:         "Save credentials",
		ActionReconnectX:          "Reconnect from browser",
		ActionRunNow:              "Analyze now",
		ActionStopRun:             "Stop this run",
		ActionClear:               "Clear",
		ActionLoadMore:            "Load more",
		ActionShowMore:            "Show more",
		ActionFocusSearch:         "Search",
		AppearanceSystem:          "System",
		AppearanceLight:           "Light",
		AppearanceDark:            "Dark",
		ActionCopyLink:            "Copy link",
		ActionCopied:              "Copied",
		ActionOpenBookmark:        "Open bookmark",
		ActionCopyText:            "Copy text",
		ActionAddToSelection:      "Add to selection",
		ActionRemoveFromSelection: "Remove from selection",
		ActionArchive:             "Archive",
		ActionUnarchive:           "Move out of archive",
		ActionFilterTag:           "Filter by this tag",
		ActionOnlyTag:             "Only this tag",
		ActionFilterAuthor:        "Filter by this author",
		ActionOnlyAuthor:          "Only this author",
		ActionCopyHandle:          "Copy @handle",
		ActionCollapseSidebar:     "Hide sidebar",
		ActionExpandSidebar:       "Show sidebar",
		ActionToggleSidebar:       "Show/Hide Sidebar",
		OpenOnX:                   "Open on X",
		MediaPrevious:             "Previous media",
		MediaNext:                 "Next media",
		MediaClose:                "Close preview",
		MediaPlay:                 "Play video",
		MediaPlayGif:              "Play GIF",
		MediaTitle:                "Media",
		ActionUndo:                "Undo",
		ActionOpenSyncSettings:    "Open sync settings",
	},
	ZHCN: {
		AppName:                   "trove",
		MenuFile:                  "文件",
		MenuEdit:                  "编辑",
		MenuView:                  "显示",
		MenuWindow:                "窗口",
		BundleExportTitle:         "导出备份",
		BundleExportLegacy:        "导出兼容备份",
		BundleExported:            "备份已导出",
		BundleChoosingWhere:       "正在选择保存位置…",
		BundleExporting:           "正在导出书库…",
		BundleReading:             "正在读取文件…",
		BundleImporting:           "正在导入…",
		BundleImportTitle:         "导入备份",
		BundleRestoreTitle:        "恢复备份",
		BundleImportBody:          "%d 条新增，%d 条已存在",
		BundleReport:              "新增 %d 条 · 更新 %d 条 · 标签 %d 个 · 原始负载 %d 条",
		ActionImport:              "导入",
		ActionRestore:             "恢复",
		MenuExport:                "导出…",
		MenuAddBookmark:           "添加书签…",
		MenuImport:                "导入…",
		MenuCheckUpdates:          "检查更新…",
		ToolbarGraph:              "\u5173\u7cfb\u56fe",
		ViewGrid:                  "\u7f51\u683c\u89c6\u56fe",
		ViewList:                  "\u5217\u8868\u89c6\u56fe",
		GraphSummary:              "%d 个标签 \u00b7 %d 条关联",
		GraphHidden:               "\u5df2\u9690\u85cf %d \u4e2a",
		GraphAuthorNodesSuffix:    "位作者节点",
		GraphAuthorsSuffix:        "位作者",
		GraphEmptyTitle:           "暂无可展示的关联",
		GraphCanvas:               "标签关系图",
		GraphLegend:               "点越大，标签下的书签越多 · 连线越粗，两个标签一起出现得越多",
		GraphFocusPanel:           "聚焦的标签",
		GraphEmptyDescription:     "先给书签打上标签，或放宽当前的筛选条件。",
		GraphBuildingTitle:        "正在构建关系图…",
		GraphShowAuthors:          "显示作者",
		GraphClearFocus:           "取消聚焦",
		GraphOpenBookmarks:        "查看该标签的书签",
		SearchLabel:               "搜索",
		BookmarksCountLabel:       "书签",
		ToolbarCount:              "%d",
		ResultsLabel:              "结果",
		ResultsLoadingMore:        "正在加载更多...",
		ToolbarSearch:             "搜索书签、作者和备注...",
		ActiveFiltersLabel:        "活跃筛选:",
		ToolbarFilters:            "筛选",
		FilterConditions:          "筛选条件",
		FilterDescription:         "选择要应用的过滤条件。",
		FilterHasMedia:            "包含媒体",
		FilterHasVideo:            "包含视频",
		FilterArchived:            "已归档",
		FilterArchivedWithCount:   "已归档（%d）",
		ToolbarShowingOf:          "%d / %d",
		ToolbarRefreshing:         "更新中…",
		ToolbarSearching:          "搜索中…",
		FilterLongform:            "长文",
		FilterClearAll:            "清除全部",
		SortTimeline:              "最近保存",
		SortSavedDesc:             "最近保存",
		SortSavedAsc:              "最早保存",
		SortCreatedDesc:           "最近发布",
		SortLikesDesc:             "最多喜欢",
		LoadingFeedTitle:          "正在整理你的书签",
		LoadFailedTitle:           "书签加载失败",
		InfoLabel:                 "信息",
		AddBookmark:               "添加书签",
		AddLinkPlaceholder:        "粘贴帖子链接",
		AddTextPlaceholder:        "帖子正文（可选）",
		AddNotAPostLink:           "这不是一条 X 帖子的链接。",
		ActionRetry:               "重新加载",
		EmptyLibrary:              "书库还是空的",
		EmptyLibraryDetail:        "trove 会把你已经在 X 上收藏的帖子复制进来。先在浏览器里登录 x.com，再点连接；也可以直接导入已有的备份。",
		EmptyLibraryConnect:       "连接 X",
		EmptyFilterDetail:         "试试调整搜索词、标签或作者范围。",
		EmptyFilter:               "当前筛选条件下没有匹配的书签",
		EmptyArchive:              "归档里还没有内容",
		EmptyArchiveDetail:        "归档后的书签会带着标签和所在列表离开书库，留在这里。可以用行菜单归档一条，或选中多条后用列表上方的操作栏。",
		ResultsListLabel:          "书签列表",
		ActionSync:                "立即同步",
		ActionScrollTop:           "回到顶部",
		ActionScrollBottom:        "回到底部",
		ActionCancel:              "取消",
		ActionAllow:               "允许",
		ActionDelete:              "删除",
		ActionRemove:              "移除",
		ActionRemoveNamed:         "移除 %s",
		ActionOpenNamed:           "打开 %s",
		ActionRenameNamed:         "重命名 %s",
		ActionDeleteNamed:         "删除 %s",
		ActionSettings:            "设置",
		ActionAddTag:              "添加标签",
		ActionMoveToList:          "移动到列表",
		ActionReview:              "整理词表",
		ActionRunAutotag:          "运行自动打标",
		ActionClearSelection:      "取消选择",
		BatchSelected:             "已选择 %d 条",
		BatchSelectCurrent:        "选择当前结果",
		BatchSelectLoaded:         "选择已加载",
		BatchApplyTag:             "应用标签",
		BatchTooMany:              "结果太多，无法一次全选",
		BatchChooseTag:            "选择标签",
		BatchChooseList:           "选择列表",
		BatchTagged:               "已为 %d 条书签添加标签",
		BatchTaggedOne:            "已为 1 条书签添加标签",
		BatchMoved:                "已将 %d 条书签移动到 %s",
		BatchMovedOne:             "已将 1 条书签移动到 %s",
		ExportData:                "导出数据",
		ExportingData:             "导出中...",
		ActionAdd:                 "添加",
		ActionSave:                "保存",
		ActionTestConnection:      "测试连接",
		SearchAuthors:             "搜索作者",
		SearchAuthorsPlaceholder:  "搜索作者...",
		ActionSaveCookies:         "保存凭证",
		ActionReconnectX:          "从浏览器重新连接",
		ActionRunNow:              "立即分析",
		ActionStopRun:             "停止本次分析",
		ActionClear:               "清除",
		ActionLoadMore:            "加载更多",
		ActionShowMore:            "显示更多",
		ActionFocusSearch:         "搜索",
		AppearanceSystem:          "跟随系统",
		AppearanceLight:           "浅色",
		AppearanceDark:            "深色",
		ActionCopyLink:            "复制链接",
		ActionCopied:              "已复制",
		ActionOpenBookmark:        "打开书签",
		ActionCopyText:            "复制正文",
		ActionAddToSelection:      "加入选择",
		ActionRemoveFromSelection: "移出选择",
		ActionArchive:             "归档",
		ActionUnarchive:           "移出归档",
		ActionFilterTag:           "按此标签筛选",
		ActionOnlyTag:             "只看此标签",
		ActionFilterAuthor:        "按此作者筛选",
		ActionOnlyAuthor:          "只看此作者",
		ActionCopyHandle:          "复制 @handle",
		ActionCollapseSidebar:     "收起侧栏",
		ActionExpandSidebar:       "展开侧栏",
		ActionToggleSidebar:       "显示/隐藏侧栏",
		OpenOnX:                   "在 X 中打开",
		MediaPrevious:             "上一张",
		MediaNext:                 "下一张",
		MediaClose:                "关闭预览",
		MediaPlay:                 "播放视频",
		MediaPlayGif:              "播放 GIF",
		MediaTitle:                "媒体资源",
		ActionUndo:                "撤销",
		ActionOpenSyncSettings:    "打开同步设置",
	},
}
