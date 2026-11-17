package i18n

// The library's words: the sidebar, the tag and list pages, the suggestions and
// review panels, and the inspector.

const (
	SidebarLibrary        Key = "sidebar.library"
	SidebarAllBookmarks   Key = "sidebar.allBookmarks"
	SidebarInbox          Key = "sidebar.inbox"
	SidebarUnclassified   Key = "sidebar.unclassified"
	SidebarLists          Key = "sidebar.lists"
	SidebarTags           Key = "sidebar.tags"
	SidebarNoLists        Key = "sidebar.noLists"
	SidebarNoTagsCreated  Key = "sidebar.noTagsCreated"
	SidebarNoAuthors      Key = "sidebar.noAuthors"
	SidebarNoAuthorsMatch Key = "sidebar.noAuthorsMatch"
	// SidebarNoTagsMatch is the same answer on the tags tab, which drew nothing at all.
	SidebarNoTagsMatch Key = "sidebar.noTagsMatch"
	TagTableName       Key = "tagTableName"
	TagTableBookmarks  Key = "tagTableBookmarks"
	TagTablePending    Key = "tagTablePending"
	TagTableUpdated    Key = "tagTableUpdated"
	TagsCountLabel     Key = "tagsCountLabel"

	InspectorTitle Key = "inspector.title"
	InspectorEmpty Key = "inspector.empty"
	// InspectorPublished is the tooltip on the published date, which the detail draws as a small
	// clock rather than as a labelled row. timelineTitle, savedAt, done and noTagsYet left
	// the catalog with the rail and the footer: the detail has no timeline, no saved date, no
	// Done button (the close control does that) and no sentence saying it has no tags (the tag
	// field says so).
	InspectorPublished   Key = "publishedAt"
	TagManageLabel       Key = "manageTagsLabel"
	TagExitManagement    Key = "exitTagManagement"
	TagSearchPlaceholder Key = "searchTagsPlaceholder"
	TagFilterAll         Key = "tagFilterAll"
	TagFilterOrphan      Key = "tagFilterOrphan"
	TagFilterLongTail    Key = "tagFilterLongTail"
	TagFilterPending     Key = "tagFilterPending"
	TagSortMost          Key = "tagSortMostBookmarks"
	TagSortFewest        Key = "tagSortFewestBookmarks"
	TagSortName          Key = "tagSortName"
	TagSortRecent        Key = "tagSortRecent"
	TagTagsSuffix        Key = "tagsCountSuffix"
	TagOrphanSuffix      Key = "orphanTagsSuffix"
	TagLongTailSuffix    Key = "longTailTagsSuffix"
	TagHealthOk          Key = "tagHealthOk"
	TagHealthTooMany     Key = "tagHealthTooMany"
	TagCleanupOrphans    Key = "cleanupOrphanTagsLabel"
	TagCleanupConfirm    Key = "cleanupOrphanTagsConfirmPrefix"
	TagMergeLabel        Key = "mergeTagsLabel"
	TagDeleteLabel       Key = "deleteTagsLabel"
	TagDeleteFailed      Key = "deleteTagsFailed"
	TagSelectAll         Key = "selectAllTags"
	TagNoTagsCreated     Key = "noTagsCreated"
	// TagLibraryReading stands in for the empty state on the first frame of the page:
	// the counts are read off the window's thread.
	TagLibraryReading     Key = "tagLibraryReading"
	TagNoTagsDescription  Key = "noTagsCreatedDescription"
	TagNoMatchTitle       Key = "noTagsMatchTitle"
	TagNoMatchDescription Key = "noTagsMatchDescription"
	TagBatchSelected      Key = "tagBatchSelected"
	TagAliasesLabel       Key = "tagAliasesLabel"
	TagCleanupDone        Key = "tag.cleanupDone"
	ReviewLabel           Key = "reviewVocabularyLabel"
	ReviewRunning         Key = "reviewVocabularyRunning"
	ReviewTitle           Key = "reviewVocabularyTitle"
	ReviewDescription     Key = "reviewVocabularyDescription"
	ReviewEmpty           Key = "reviewVocabularyEmpty"
	ReviewNoTags          Key = "reviewVocabularyNoTags"
	ReviewFailed          Key = "reviewVocabularyFailedPrefix"
	ReviewRetry           Key = "reviewRetry"
	ReviewMerge           Key = "reviewMergeLabel"
	ReviewRename          Key = "reviewRenameLabel"
	ReviewApply           Key = "reviewApplyLabel"
	ReviewDismiss         Key = "reviewDismissLabel"
	ReviewAppliedCount    Key = "review.appliedCount"
	TagMergeInto          Key = "mergeTargetLabel"
	TagAliasesPlaceholder Key = "tagAliasesPlaceholder"
	TagRenameLabel        Key = "renameTagLabel"
	TagRenameTaken        Key = "renameTagTaken"
	// The tag table's row controls carry their row's name: a table of dozens drew dozens of
	// identical "Rename tag" and "Delete" buttons, so a screen reader could not say which
	// row each belonged to and a test could only ever press the first one.
	TagBookmarksFor      Key = "tag.bookmarksFor"
	TagRenameFor         Key = "tag.renameFor"
	TagAliasesFor        Key = "tag.aliasesFor"
	TagDeleteFor         Key = "tag.deleteFor"
	TagSelectRowFor      Key = "tag.selectRowFor"
	SidebarNewList       Key = "newListPlaceholder"
	RenameListLabel      Key = "renameListLabel" // this app's own wording; lists are native
	DeleteListConfirm    Key = "deleteListConfirm"
	SuggestionsAcceptAll Key = "acceptAllSuggestions"
	LastSyncLabel        Key = "lastSync"
	// LastSyncNever is the word for a workspace that has never synced. It lives outside
	// the main copy catalog, which is why the catalog comparison never reported it missing.
	LastSyncNever            Key = "syncNever"
	SavedViewsTitle          Key = "savedViewsTitle"
	SavedViewNamePlaceholder Key = "savedViewNamePlaceholder"
	SaveCurrentView          Key = "saveCurrentView"
	DeleteSavedView          Key = "deleteSavedView"
	SavedViewApply           Key = "savedViewApply"
	// SavedViewApplyNamed is the apply button with the view it restores. This build draws an
	// icon, so the view's name has to be in the control's label.
	SavedViewApplyNamed  Key = "savedViewApplyNamed"
	DefaultSavedViewName Key = "defaultSavedViewName"
	AuthorsTitle         Key = "authorsTitle"
	DeleteTagConfirm     Key = "deleteTagConfirmPrefix"
	CreateTagLabel       Key = "createTagLabel"
	CreateTagPrompt      Key = "createTagPrompt"
	CreateTagSymbol      Key = "createTagSymbol" // a symbol, not a word: both catalogs hold "+"
	// AuthorAvatarFallback is the mark an author row shows when the author has no picture. It
	// is a literal "@", a symbol, and TestNoEnglishIsLeftInTheChineseCatalogue's letter test
	// lets it through for that reason.
	AuthorAvatarFallback    Key = "authorAvatarFallback"
	DeleteTagsConfirm       Key = "deleteTagsConfirmPrefix"
	SuggestionsPendingTitle Key = "pendingSuggestionsTitle"
	TagMergeNeedTarget      Key = "mergeNeedTarget"
	TagMergeNeedsSource     Key = "mergeNeedsSource"
	TagMergeChoose          Key = "mergeChoose"
	TagMergeConfirmBody     Key = "mergeConfirmBody"
	TagMergeDone            Key = "mergeDone"
	// The two notes the header shows while a library-wide edit runs off the window's
	// thread; the window keeps drawing while they are up.
	TagMerging     Key = "tagMerging"
	ReviewApplying Key = "reviewApplying"

	SuggestionsTitle  Key = "suggestions.title"
	SuggestionsEmpty  Key = "suggestions.empty"
	SuggestionsAccept Key = "suggestions.accept"
	SuggestionsReject Key = "suggestions.reject"
	// SuggestionsAcceptFor and SuggestionsRejectFor carry the tag the row proposes. A panel
	// of ten rows drawn with the bare words announces ten identically named buttons.
	SuggestionsAcceptFor Key = "suggestions.acceptFor"
	SuggestionsRejectFor Key = "suggestions.rejectFor"
	// SuggestionsApplyFailed is the batch's failure count: one row can fail while the
	// rest are accepted, and the sentence has to say both numbers.
	SuggestionsApplyFailed Key = "suggestions.applyFailed"
)

var libraryMessages = messages{
	EN: {
		SidebarLibrary:        "Library",
		SidebarAllBookmarks:   "All bookmarks",
		SidebarInbox:          "Inbox",
		SidebarUnclassified:   "Unclassified",
		SidebarLists:          "Lists",
		SidebarTags:           "Tags",
		SidebarNoLists:        "No lists yet",
		SidebarNoTagsCreated:  "No tags created yet.",
		SidebarNoAuthors:      "No authors yet.",
		SidebarNoAuthorsMatch: "No authors match your search.",
		SidebarNoTagsMatch:    "No tags match your search.",
		TagTableName:          "Tag",
		TagTableBookmarks:     "Bookmarks",
		TagTablePending:       "Pending",
		TagTableUpdated:       "Updated",
		TagsCountLabel:        "tags",
		InspectorTitle:        "Details",
		InspectorEmpty:        "Select a bookmark to view details",
		InspectorPublished:    "Published",
		TagManageLabel:        "Manage tags",
		TagExitManagement:     "Back to bookmarks",
		TagSearchPlaceholder:  "Search tag names or aliases...",
		TagFilterAll:          "All tags",
		TagFilterOrphan:       "Orphan tags only",
		TagFilterLongTail:     "Long-tail tags (<=2)",
		TagFilterPending:      "With pending suggestions",
		TagSortMost:           "Most bookmarks",
		TagSortFewest:         "Fewest bookmarks",
		TagSortName:           "Name A-Z",
		TagSortRecent:         "Recently updated",
		TagTagsSuffix:         "tags",
		TagOrphanSuffix:       "orphan tags",
		TagLongTailSuffix:     "long-tail tags",
		TagHealthOk:           "Library size is healthy",
		TagHealthTooMany:      "Too many tags, review the vocabulary",
		TagCleanupOrphans:     "Delete tags without bookmarks",
		TagCleanupConfirm:     "Delete tags without bookmarks",
		TagMergeLabel:         "Merge tags",
		TagDeleteLabel:        "Delete selected tags",
		TagDeleteFailed:       "%d of %d tags could not be deleted",
		TagSelectAll:          "Select listed tags",
		TagNoTagsCreated:      "No tags yet",
		TagLibraryReading:     "Reading tags…",
		TagNoTagsDescription:  "Tags appear here after a sync or auto-tagging, or add one yourself.",
		TagNoMatchTitle:       "No tags match",
		TagNoMatchDescription: "Try a different search term or filter.",
		TagBatchSelected:      "%d tags selected",
		TagAliasesLabel:       "Edit aliases",
		TagCleanupDone:        "Removed %d empty tags.",
		ReviewLabel:           "Tidy vocabulary",
		ReviewRunning:         "Reviewing…",
		ReviewTitle:           "Vocabulary suggestions",
		ReviewDescription:     "Compares the whole tag list and proposes duplicates to merge and names to make clearer. Only tag names are sent to the model, not bookmarks. Pick what you want, then apply.",
		ReviewEmpty:           "The tag list has no duplicates or unclear names, so there is nothing to merge or rename.",
		ReviewNoTags:          "The tag list is empty, so there is nothing to merge or rename yet. Accept a few suggestions or create tags first.",
		ReviewFailed:          "Review failed",
		ReviewRetry:           "Try again",
		ReviewMerge:           "Merge",
		ReviewRename:          "Rename",
		ReviewApply:           "Apply selected",
		ReviewDismiss:         "Dismiss",
		ReviewAppliedCount:    "Applied %d change(s).",
		TagMergeInto:          "Merge into",
		TagAliasesPlaceholder: "Separate aliases with commas",
		TagRenameLabel:        "Rename tag",
		TagRenameTaken:        "“%s” is already used by another tag.",
		TagBookmarksFor:       "View bookmarks of %s",
		TagRenameFor:          "Rename tag %s",
		TagAliasesFor:         "Edit aliases of %s",
		TagDeleteFor:          "Delete tag %s",
		TagSelectRowFor:       "Select tag %s",
		// "List name", not "View name": this field creates a list, and it used to share the
		// saved-view field's wording -- two different things asking for the same word, so a
		// reader could not tell which row they were typing into.
		SidebarNewList:           "List name",
		RenameListLabel:          "Rename list",
		DeleteListConfirm:        "Reset will clear all local bookmarks, tags, and settings. Continue?",
		SuggestionsAcceptAll:     "Accept all suggestions",
		LastSyncLabel:            "Last sync",
		LastSyncNever:            "Not synced yet",
		SavedViewsTitle:          "Saved views",
		SavedViewNamePlaceholder: "View name",
		SaveCurrentView:          "Save current view",
		DeleteSavedView:          "Delete view",
		SavedViewApply:           "Apply view",
		SavedViewApplyNamed:      "Apply view %s",
		DefaultSavedViewName:     "Saved view",
		AuthorsTitle:             "Authors",
		DeleteTagConfirm:         "Delete tag",
		CreateTagLabel:           "Create tag",
		CreateTagPrompt:          "Enter a new tag name",
		CreateTagSymbol:          "+",
		AuthorAvatarFallback:     "@",
		DeleteTagsConfirm:        "Delete selected tags",
		SuggestionsPendingTitle:  "Pending tags",
		TagMergeNeedTarget:       "Pick a tag to merge into.",
		TagMergeNeedsSource:      "Pick at least one other tag to merge.",
		TagMergeChoose:           "Choose tag",
		TagMergeConfirmBody:      "Merge %d tag(s) into “%s”? The merged tags are removed.",
		TagMergeDone:             "Merged %d tag(s).",
		TagMerging:               "Merging tags…",
		ReviewApplying:           "Applying the review…",
		SuggestionsTitle:         "Suggestions",
		SuggestionsEmpty:         "Nothing to review.",
		SuggestionsAccept:        "Accept suggestion",
		SuggestionsReject:        "Reject suggestion",
		SuggestionsAcceptFor:     "Accept suggestion %s",
		SuggestionsRejectFor:     "Reject suggestion %s",
		SuggestionsApplyFailed:   "%d of %d suggestions could not be applied",
	},
	ZHCN: {
		SidebarLibrary:           "资料库",
		SidebarAllBookmarks:      "全部书签",
		SidebarInbox:             "收件箱",
		SidebarUnclassified:      "未分类",
		SidebarLists:             "列表",
		SidebarTags:              "标签",
		SidebarNoLists:           "还没有列表",
		SidebarNoTagsCreated:     "还没有创建任何标签。",
		SidebarNoAuthors:         "还没有作者。",
		SidebarNoAuthorsMatch:    "没有匹配的作者。",
		SidebarNoTagsMatch:       "没有匹配的标签。",
		TagTableName:             "标签",
		TagTableBookmarks:        "书签数",
		TagTablePending:          "待确认",
		TagTableUpdated:          "更新时间",
		TagsCountLabel:           "标签",
		InspectorTitle:           "详情",
		InspectorEmpty:           "选择一个书签以查看详情",
		InspectorPublished:       "发布于",
		TagManageLabel:           "管理标签",
		TagExitManagement:        "返回书签",
		TagSearchPlaceholder:     "搜索标签名或别名...",
		TagFilterAll:             "全部标签",
		TagFilterOrphan:          "仅孤儿标签",
		TagFilterLongTail:        "长尾标签（≤2 条）",
		TagFilterPending:         "有待确认建议",
		TagSortMost:              "书签最多",
		TagSortFewest:            "书签最少",
		TagSortName:              "名称 A-Z",
		TagSortRecent:            "最近更新",
		TagTagsSuffix:            "个标签",
		TagOrphanSuffix:          "个孤儿标签",
		TagLongTailSuffix:        "个长尾标签",
		TagHealthOk:              "标签规模健康",
		TagHealthTooMany:         "标签偏多，建议整理词表",
		TagCleanupOrphans:        "确认删除没有书签的标签",
		TagCleanupConfirm:        "确认删除没有书签的标签",
		TagMergeLabel:            "合并标签",
		TagDeleteLabel:           "删除选中标签",
		TagDeleteFailed:          "有 %d/%d 个标签删除失败",
		TagSelectAll:             "选择当前标签",
		TagNoTagsCreated:         "还没有标签",
		TagLibraryReading:        "正在读取标签…",
		TagNoTagsDescription:     "同步或自动标签生成的标签会出现在这里，也可以手动创建。",
		TagNoMatchTitle:          "没有匹配的标签",
		TagNoMatchDescription:    "试试调整搜索词或筛选条件。",
		TagBatchSelected:         "已选择 %d 个标签",
		TagAliasesLabel:          "编辑别名",
		TagCleanupDone:           "已清理 %d 个空标签。",
		ReviewLabel:              "整理词表",
		ReviewRunning:            "分析中…",
		ReviewTitle:              "词表整理建议",
		ReviewDescription:        "比较全部标签，找出可以合并的重复标签，以及可以改得更清楚的名字；只会把标签名发给模型，不会发送书签内容。勾选后再应用。",
		ReviewEmpty:              "标签列表里没有重复或含糊的名字，暂时不需要合并或重命名。",
		ReviewNoTags:             "词表还是空的，暂时没有可合并或重命名的标签。先确认几条建议或手动创建标签，再来整理。",
		ReviewFailed:             "整理失败",
		ReviewRetry:              "重试",
		ReviewMerge:              "合并",
		ReviewRename:             "重命名",
		ReviewApply:              "应用所选",
		ReviewDismiss:            "关闭建议",
		ReviewAppliedCount:       "已应用 %d 项变更。",
		TagMergeInto:             "合并到",
		TagAliasesPlaceholder:    "用逗号分隔多个别名",
		TagRenameLabel:           "重命名标签",
		TagRenameTaken:           "“%s”已被另一个标签使用。",
		TagBookmarksFor:          "查看 %s 的书签",
		TagRenameFor:             "重命名标签 %s",
		TagAliasesFor:            "编辑 %s 的别名",
		TagDeleteFor:             "删除标签 %s",
		TagSelectRowFor:          "选择标签 %s",
		SidebarNewList:           "列表名称",
		RenameListLabel:          "重命名列表",
		DeleteListConfirm:        "重置会清空所有本地书签、标签和设置，确认继续？",
		SuggestionsAcceptAll:     "接受全部建议",
		LastSyncLabel:            "上次同步",
		LastSyncNever:            "尚未同步",
		SavedViewsTitle:          "保存视图",
		SavedViewNamePlaceholder: "视图名称",
		SaveCurrentView:          "保存当前视图",
		DeleteSavedView:          "删除视图",
		SavedViewApply:           "应用视图",
		SavedViewApplyNamed:      "应用视图 %s",
		DefaultSavedViewName:     "保存视图",
		AuthorsTitle:             "作者",
		DeleteTagConfirm:         "确认删除标签",
		CreateTagLabel:           "创建标签",
		CreateTagPrompt:          "输入新标签名称",
		CreateTagSymbol:          "+",
		AuthorAvatarFallback:     "@",
		DeleteTagsConfirm:        "确认删除选中的标签",
		SuggestionsPendingTitle:  "待确认标签",
		TagMergeNeedTarget:       "请选择合并到的目标标签。",
		TagMergeNeedsSource:      "请再选一个要合并的标签。",
		TagMergeChoose:           "选择标签",
		TagMergeConfirmBody:      "把 %d 个标签合并到“%s”？被合并的标签会被删除。",
		TagMergeDone:             "已合并 %d 个标签。",
		TagMerging:               "正在合并标签…",
		ReviewApplying:           "正在应用整理建议…",
		SuggestionsTitle:         "建议",
		SuggestionsEmpty:         "没有待审阅的建议。",
		SuggestionsAccept:        "接受建议",
		SuggestionsReject:        "拒绝建议",
		SuggestionsAcceptFor:     "接受建议 %s",
		SuggestionsRejectFor:     "拒绝建议 %s",
		SuggestionsApplyFailed:   "%d 条建议中有 %d 条未能应用",
	},
}
