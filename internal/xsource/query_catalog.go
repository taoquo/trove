package xsource

import "strings"

// The X web client's own identifiers.
//
// None of these are secrets: the bearer token ships in X's public JavaScript and
// the user agent is a browser's. They are constants only so the request shape is
// in one place instead of scattered through the request builder.
//
// Being an unpublished interface, all of it can change without notice. That is
// why the query id is resolved through a catalog with a user override, and why
// a failure is classified rather than swallowed.
const (
	// BuiltinQueryID is the operation id observed in practice. It is a default, not a
	// guarantee: X rotates these.
	BuiltinQueryID = "-LGfdImKeQz0xS_jjUwzlA"

	// BearerToken is the web client's public bearer token.
	BearerToken = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

	// UserAgent is the browser the requests claim to be.
	UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

	// PageSize is both the requested page size and a hard ceiling: the endpoint
	// clamps count to 20.
	PageSize = 20

	// maxResponseBytes bounds a response body, so a hostile or broken response
	// cannot exhaust memory.
	maxResponseBytes = 8 << 20 // 8 MiB
)

// BookmarksFeatures is the feature-flag map the timeline request carries.
//
// Declared as an ordered slice of pairs rather than a map so the serialised JSON
// is deterministic, which keeps request fingerprints and test fixtures stable.
var BookmarksFeatures = []FeatureFlag{
	{"graphql_timeline_v2_bookmark_timeline", true},
	{"responsive_web_graphql_exclude_directive_enabled", true},
	{"verified_phone_label_enabled", false},
	{"responsive_web_graphql_timeline_navigation_enabled", true},
	{"responsive_web_graphql_skip_user_profile_image_extensions_enabled", false},
	{"creator_subscriptions_tweet_preview_api_enabled", true},
	{"communities_web_enable_tweet_community_results_fetch", true},
	{"c9s_tweet_anatomy_moderator_badge_enabled", true},
	{"tweetypie_unmention_optimization_enabled", true},
	{"responsive_web_edit_tweet_api_enabled", true},
	{"longform_notetweets_consumption_enabled", true},
	{"responsive_web_media_download_video_enabled", false},
	{"responsive_web_enhance_cards_enabled", false},
}

// FeatureFlag is one entry of the features map.
type FeatureFlag struct {
	Name  string
	Value bool
}

// QueryIDSource says where the query id in use came from, so the diagnostics
// screen can explain a failure without guessing.
type QueryIDSource = string

const (
	// QueryIDFromOverride means the user set it.
	QueryIDFromOverride QueryIDSource = "override"
	// QueryIDFromBuiltin means the app's own value is in use.
	QueryIDFromBuiltin QueryIDSource = "builtin"
)

// QueryCatalog resolves the operation id to call.
//
// Priority is user override, then the built-in value.
//
// A third tier — discovering the current id by fetching X's page and its
// JavaScript bundles — is deliberately *not* implemented yet. It would add a
// second scraping surface (page fetch, bundle fetch, parsing with its own size
// and timeout limits) for a case the override already covers, and a wrong guess
// is worse than a clear failure. The spec recommends deferring it; this is that
// deferral, recorded rather than silently dropped.
type QueryCatalog struct {
	override string
}

// NewQueryCatalog builds a catalog, optionally with a user override.
func NewQueryCatalog(override string) QueryCatalog {
	return QueryCatalog{override: strings.TrimSpace(override)}
}

// Resolve returns the query id and where it came from.
func (c QueryCatalog) Resolve() (string, QueryIDSource) {
	if c.override != "" {
		return c.override, QueryIDFromOverride
	}
	return BuiltinQueryID, QueryIDFromBuiltin
}
