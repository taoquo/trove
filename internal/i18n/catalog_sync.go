package i18n

// What a sync says: the status line, its failure kinds, the receipts, and the
// confirmations that stand between an action and the library.

const (
	ConfirmDeleteBookmarksTitle Key = "confirm.deleteBookmarks.title"
	ConfirmDeleteBookmarksBody  Key = "confirm.deleteBookmarks.body"
	SyncMoreAvailable           Key = "syncMoreAvailable"
	SyncStored                  Key = "syncStored"
	SyncNothingNew              Key = "syncNothingNew"
	// SyncProgressLabel is the progress line: "Syncing... {pages} pages · {items} items".
	SyncProgressLabel Key = "syncProgressLabel"
	// SyncWaitLabel is the same line while X has asked the run to wait: "Rate limited —
	// waiting {n} s". A rate-limit window is minutes long, and a page counter that does
	// not move for minutes reads as a hang.
	SyncWaitLabel Key = "syncWaitLabel"
	// The X credentials a desktop build has to be told about, because it has no browser
	// session of its own.
	SyncNeedsXCredentials Key = "syncNeedsXCredentials"
	SyncLookingForBrowser Key = "syncLookingForBrowser"
	ToastNoLink           Key = "noLinkToCopy"
	ToastDone             Key = "done"
	// The archive's receipts. The row leaves the list as the action runs, so the sentence
	// is what tells the reader it went where they meant.
	ToastArchived       Key = "toast.archived"
	ToastUnarchived     Key = "toast.unarchived"
	ToastArchivedMany   Key = "toast.archivedMany"
	ToastUnarchivedMany Key = "toast.unarchivedMany"

	StatusIdle      Key = "status.idle"
	StatusRunning   Key = "status.running"
	StatusSuccess   Key = "status.success"
	StatusPartial   Key = "status.partial"
	StatusCancelled Key = "status.cancelled" // this app's own label for this status
	StatusError     Key = "status.error"

	// The sync failure kinds. They are the reason the shell reports a failure at all: the
	// sidebar's chip says "Error", and these are what go where the explanation belongs.
	SyncErrorAuthExpired   Key = "sync.error.auth_expired"
	SyncErrorRateLimited   Key = "sync.error.rate_limited"
	SyncErrorNetworkError  Key = "sync.error.network_error"
	SyncErrorSchemaChanged Key = "sync.error.x_schema_changed"
	SyncErrorInterrupted   Key = "sync.error.interrupted"
	SyncErrorUnknown       Key = "sync.error.unknown"
	// SyncErrorInProgress is not a fault: another process owns the library's
	// sync. It gets its own sentence so a second window or an overlapping
	// scheduled command does not read as a broken sync.
	SyncErrorInProgress Key = "sync.error.sync_in_progress"
	// SyncErrorCredentialStore means the saved X session could not be read. It is
	// deliberately not the expired-session sentence: the cookies may be fine, and
	// the fix is in the system's secure storage, not on X.
	SyncErrorCredentialStore Key = "sync.error.credential_store"

	// CardSelectBookmark is the card's corner checkbox. The name carries the author, so each
	// box says which post it picks; the bare word alone repeated down a column identifies
	// nothing.
	CardSelectBookmark    Key = "card.selectBookmark"
	CardSelectBookmarkFor Key = "card.selectBookmarkFor"
)

var syncMessages = messages{
	EN: {
		ConfirmDeleteBookmarksTitle: "Delete these bookmarks?",
		ConfirmDeleteBookmarksBody:  "This removes %d bookmark(s) from your library. Your X account is not affected.",
		SyncMoreAvailable:           "Older bookmarks are waiting — sync again to continue.",
		SyncStored:                  "Synced %d new bookmark(s).",
		SyncNothingNew:              "Sync complete. No new bookmarks.",
		SyncProgressLabel:           "Syncing… {pages} pages · {items} items",
		SyncWaitLabel:               "Rate limited — waiting %d s",
		SyncNeedsXCredentials:       "Sync needs an X session. Reconnect from the browser you are signed in to, or add the cookies under Settings > Sync.",
		SyncLookingForBrowser:       "Looking for a signed-in browser…",
		ToastNoLink:                 "No link available",
		ToastDone:                   "Done",
		ToastArchived:               "Archived",
		ToastUnarchived:             "Moved out of the archive",
		ToastArchivedMany:           "Archived %d bookmarks",
		ToastUnarchivedMany:         "Moved %d bookmarks out of the archive",
		StatusIdle:                  "Idle",
		StatusRunning:               "Syncing",
		StatusSuccess:               "Success",
		StatusPartial:               "Partial success",
		StatusCancelled:             "Cancelled",
		StatusError:                 "Error",
		// The English sync error sentences.
		SyncErrorAuthExpired:     "X login expired. Sign in to X again, then sync.",
		SyncErrorRateLimited:     "X is rate limiting sync requests. Try again later.",
		SyncErrorNetworkError:    "Network request failed. Check your connection, then retry.",
		SyncErrorSchemaChanged:   "X page or API structure changed. Update the app, then sync.",
		SyncErrorInterrupted:     "The last sync was interrupted (the browser recycled the background worker). Sync again.",
		SyncErrorUnknown:         "Sync failed. Please retry.",
		SyncErrorInProgress:      "Another sync is already running. Wait for it to finish, then try again.",
		SyncErrorCredentialStore: "Couldn't read the saved X session from the system keychain. Unlock the keychain (or allow this app access), then sync again.",
		CardSelectBookmark:       "Select bookmark",
		CardSelectBookmarkFor:    "Select bookmark by %s",
	},
	ZHCN: {
		ConfirmDeleteBookmarksTitle: "删除这些书签？",
		ConfirmDeleteBookmarksBody:  "将从书库中移除 %d 条书签。你的 X 账号不受影响。",
		SyncMoreAvailable:           "还有更早的书签，再次同步即可继续。",
		SyncStored:                  "已同步 %d 条新书签。",
		SyncNothingNew:              "同步完成，没有新书签。",
		SyncProgressLabel:           "同步中… {pages} 页 · {items} 条",
		SyncWaitLabel:               "已限流，等待 %d 秒",
		SyncNeedsXCredentials:       "同步需要 X 登录状态。请从已登录的浏览器重新连接，或在 设置 > 同步 里填入 cookie。",
		SyncLookingForBrowser:       "正在查找已登录的浏览器…",
		ToastNoLink:                 "暂无可复制链接",
		ToastDone:                   "完成",
		ToastArchived:               "已归档",
		ToastUnarchived:             "已移出归档",
		ToastArchivedMany:           "已归档 %d 条书签",
		ToastUnarchivedMany:         "已将 %d 条书签移出归档",
		StatusIdle:                  "空闲",
		StatusRunning:               "同步中",
		StatusSuccess:               "成功",
		StatusPartial:               "部分成功",
		StatusCancelled:             "已取消",
		// “错误” 是该项状态的用词；“失败” 曾是本目录自己的用词，出现在应用不绘制的状态行中。
		StatusError: "错误",
		// 同步失败类型的中文文案。
		SyncErrorAuthExpired:     "X 登录已失效，请重新登录 X 后再同步。",
		SyncErrorRateLimited:     "X 暂时限制了同步请求，请稍后再试。",
		SyncErrorNetworkError:    "网络请求失败，请检查网络连接后重试。",
		SyncErrorSchemaChanged:   "X 页面或 API 结构已变化，请更新应用后再同步。",
		SyncErrorInterrupted:     "上次同步被中断（浏览器回收了后台进程），请重新同步。",
		SyncErrorUnknown:         "同步失败，请重试。",
		SyncErrorInProgress:      "另一个同步正在进行，请等它结束后再试。",
		SyncErrorCredentialStore: "无法从系统钥匙串读取已保存的 X 登录状态。请先解锁钥匙串（或允许本应用访问），再重新同步。",
		CardSelectBookmark:       "选择书签",
		CardSelectBookmarkFor:    "选择 %s 的书签",
	},
}
