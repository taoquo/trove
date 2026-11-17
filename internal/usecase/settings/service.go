// Package settings holds the app's own configuration.
//
// It is deliberately not a free-form key-value pass-through. Which keys exist, and
// which of them may leave the machine, are decisions this package makes explicitly:
// a backup that quietly carried an API endpoint would hand a user's private
// infrastructure to whoever they shared the file with.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

// Setting keys.
const (
	preferencesKey = "app.preferences"
	// llmEndpointsKey holds per-provider base URLs.
	//
	// A custom endpoint is often a private gateway on a private network, so it is
	// treated as sensitive even though it is not a credential.
	llmEndpointsKey = "llm.endpoints"
)

// ExportableKeys are the only settings a backup may carry.
//
// It is an allowlist rather than a denylist on purpose: a new setting added later
// is excluded by default, so forgetting to classify one fails safe.
var ExportableKeys = []string{preferencesKey}

// SensitiveKeys never leave the machine.
var SensitiveKeys = []string{llmEndpointsKey}

// IsExportable reports whether a key may appear in a backup.
func IsExportable(key string) bool {
	for _, candidate := range ExportableKeys {
		if key == candidate {
			return true
		}
	}
	return false
}

// IsSensitive reports whether a key is deliberately withheld from exports.
func IsSensitive(key string) bool {
	for _, candidate := range SensitiveKeys {
		if key == candidate {
			return true
		}
	}
	return false
}

// Appearance is the theme preference.
type Appearance = string

const (
	AppearanceSystem Appearance = "system"
	AppearanceLight  Appearance = "light"
	AppearanceDark   Appearance = "dark"
)

// AllAppearances is the whitelist.
var AllAppearances = []Appearance{AppearanceSystem, AppearanceLight, AppearanceDark}

// ValidAppearance reports whether a stored preference is known.
func ValidAppearance(value string) bool {
	for _, candidate := range AllAppearances {
		if value == candidate {
			return true
		}
	}
	return false
}

// Locales supported by the interface.
var AllLocales = []string{"zh-CN", "en"}

// ValidLocale reports whether a locale is supported. An empty value is valid and
// means no choice has been made, which the interface resolves to English.
func ValidLocale(value string) bool {
	if value == "" {
		return true
	}
	for _, candidate := range AllLocales {
		if value == candidate {
			return true
		}
	}
	return false
}

// SyncPreferences configures syncing.
type SyncPreferences struct {
	// PageLimit caps a first full sync.
	PageLimit int `json:"pageLimit"`
	// SyncOnLaunch starts a sync when the app opens.
	SyncOnLaunch bool `json:"syncOnLaunch"`
	// SaveRawPayloads keeps the untouched responses, which multiplies the
	// database size for data the app does not read.
	SaveRawPayloads bool `json:"saveRawPayloads"`
	// StopBufferPages is how many extra pages an incremental sync fetches after it
	// starts seeing bookmarks it already has. Larger is safer and slower, which is
	// exactly what the hint says.
	StopBufferPages int `json:"stopBufferPages"`
}

// BackupPreferences holds the automatic-backup choice.
type BackupPreferences struct {
	// AutoBackups is absent in preferences written before automatic backups
	// existed, and absent means on: the point of the feature is that a reader who
	// never opens this page still has a copy. Turning it off stores false, which is
	// the only shape that can tell "chose off" from "never chose".
	AutoBackups *bool `json:"autoBackups,omitempty"`
}

// DiagnosticsPreferences are the troubleshooting aids. Both are off until the reader
// asks for them: a copy of the reader's activity on disk is not something to turn on
// unasked.
type DiagnosticsPreferences struct {
	// FileLog writes events and counts to a rotating log beside the library.
	FileLog bool `json:"fileLog,omitempty"`
}

// Preferences is the whole user-facing configuration.
type Preferences struct {
	Appearance Appearance `json:"appearance"`
	// Locale is empty to follow the system.
	Locale      string                 `json:"locale"`
	Sync        SyncPreferences        `json:"sync"`
	Backup      BackupPreferences      `json:"backup"`
	Layout      LayoutPreferences      `json:"layout"`
	Diagnostics DiagnosticsPreferences `json:"diagnostics"`
	// ViewMode is how the results are laid out. It is stored because a saved view keeps it
	// and a restart should not rearrange the library.
	ViewMode ViewMode `json:"viewMode,omitempty"`
}

// ViewMode is the results layout.
type ViewMode string

// The two layouts the interface offers.
const (
	ViewGrid ViewMode = "grid"
	ViewList ViewMode = "list"
)

// LayoutPreferences are the column widths, stored as leftSidebarWidth and
// rightSidebarWidth.
//
// They belong in the persisted preferences rather than in memory because a user who
// drags a divider expects it to still be where they left it tomorrow.
type LayoutPreferences struct {
	LeftSidebarWidth  float32 `json:"leftSidebarWidth"`
	RightSidebarWidth float32 `json:"rightSidebarWidth"`
	// SidebarCollapsed hides the left column. The choice is kept across launches, so a
	// reader who put the sidebar away does not find it back on every start.
	SidebarCollapsed bool `json:"sidebarCollapsed,omitempty"`
	// SidebarChosen records that the reader picked the layout themselves. Without it the
	// window's width decides -- the sidebar is shown from 1024 points up, hidden below,
	// following the 1024-point breakpoint. A stored SidebarCollapsed from before this
	// field means the press that hid the column was a pick, so the shell treats it as one
	// when it loads.
	SidebarChosen bool `json:"sidebarChosen,omitempty"`
}

// The widths the shell lays out with, and the bounds it clamps to.
//
// The left column is 212px, dragged between 168 and 320 and never past
// 36% of the window. The inspector is this app's own -- this shell is the only
// implementation with a third column -- so its 360/320/520 defaults and bounds are the
// app's own.
const (
	DefaultLeftSidebarWidth  float32 = 212
	DefaultRightSidebarWidth float32 = 360
	MinSidebarWidth          float32 = 168
	MaxSidebarWidth          float32 = 320
	MinInspectorWidth        float32 = 320
	MaxInspectorWidth        float32 = 520
)

// DefaultLayout is the layout a fresh install uses.
func DefaultLayout() LayoutPreferences {
	return LayoutPreferences{
		LeftSidebarWidth:  DefaultLeftSidebarWidth,
		RightSidebarWidth: DefaultRightSidebarWidth,
	}
}

// NormalizeLayout clamps stored widths into the allowed range.
//
// A stored value can come from an older build, a hand-edited database, or a bundle
// written on a much larger display. Clamping on read is what keeps a bad number from
// producing a column that cannot be dragged back.
func NormalizeLayout(layout LayoutPreferences) LayoutPreferences {
	if layout.LeftSidebarWidth == 0 {
		layout.LeftSidebarWidth = DefaultLeftSidebarWidth
	}
	if layout.RightSidebarWidth == 0 {
		layout.RightSidebarWidth = DefaultRightSidebarWidth
	}
	layout.LeftSidebarWidth = min(max(layout.LeftSidebarWidth, MinSidebarWidth), MaxSidebarWidth)
	layout.RightSidebarWidth = min(max(layout.RightSidebarWidth, MinInspectorWidth), MaxInspectorWidth)
	return layout
}

// DefaultPreferences is what a fresh install uses.
func DefaultPreferences() Preferences {
	return Preferences{
		Appearance: AppearanceSystem,
		Locale:     "",
		Sync: SyncPreferences{
			PageLimit:       0, // the domain's default
			SyncOnLaunch:    false,
			SaveRawPayloads: false,
			StopBufferPages: domain.DefaultIncrementalStopBufferPages,
		},
		// Without this the first run would lay out two zero-width columns, and the
		// widths are only filled in from the database on a later load.
		Layout: DefaultLayout(),
	}
}

// AutoBackupEnabled reports whether the daily copy runs.
func (p Preferences) AutoBackupEnabled() bool {
	return p.Backup.AutoBackups == nil || *p.Backup.AutoBackups
}

// Validate reports an unknown value rather than silently defaulting it.
//
// A stored value this build does not understand means the file was written by a
// different version; quietly substituting a default would hide that.
func (p Preferences) Validate() error {
	if !ValidAppearance(p.Appearance) {
		return fmt.Errorf("settings: unknown appearance %q", p.Appearance)
	}
	if !ValidLocale(p.Locale) {
		return fmt.Errorf("settings: unknown locale %q", p.Locale)
	}
	if p.Sync.PageLimit < 0 {
		return errors.New("settings: the page limit cannot be negative")
	}
	return nil
}

// Normalize fills defaults for anything unset.
func (p Preferences) Normalize() Preferences {
	normalized := p
	if normalized.Appearance == "" {
		normalized.Appearance = AppearanceSystem
	}
	normalized.Layout = NormalizeLayout(normalized.Layout)
	// An absent or unknown value means the grid rather than the zero value of the type.
	if normalized.ViewMode != ViewList {
		normalized.ViewMode = ViewGrid
	}
	if normalized.Sync.StopBufferPages <= 0 {
		normalized.Sync.StopBufferPages = domain.DefaultIncrementalStopBufferPages
	}
	return normalized
}

// Service reads and writes the configuration.
type Service struct {
	repos ports.Repositories
	clock ports.Clock
}

// New builds a service.
func New(repos ports.Repositories, clock ports.Clock) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Service{repos: repos, clock: clock}
}

// Load reads the preferences, falling back to the defaults.
//
// An unreadable or unknown blob falls back rather than failing: the setting is
// recoverable and the library is not, so a bad value must not stop the app from
// opening.
func (s *Service) Load(ctx context.Context) (Preferences, error) {
	if s.repos.Settings == nil {
		return DefaultPreferences(), nil
	}

	raw, found, err := s.repos.Settings.Get(ctx, preferencesKey)
	if err != nil {
		return Preferences{}, err
	}
	if !found || raw == "" {
		return DefaultPreferences(), nil
	}

	var preferences Preferences
	if err := json.Unmarshal([]byte(raw), &preferences); err != nil {
		return DefaultPreferences(), nil
	}
	preferences = preferences.Normalize()
	if err := preferences.Validate(); err != nil {
		// A value from another build must not brick the app; the defaults are a
		// working configuration and the user can change it again.
		return DefaultPreferences(), nil
	}
	return preferences, nil
}

// Save validates and writes the preferences.
func (s *Service) Save(ctx context.Context, preferences Preferences) error {
	normalized := preferences.Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("settings: encode preferences: %w", err)
	}
	return s.repos.Settings.Set(ctx, preferencesKey, string(encoded), s.clock.Now())
}

// Endpoints returns the stored per-provider base URLs.
//
// They are read and written through this package but are never exported, which is
// why they have their own key rather than living inside the preferences blob.
func (s *Service) Endpoints(ctx context.Context) (map[string]string, error) {
	if s.repos.Settings == nil {
		return map[string]string{}, nil
	}

	raw, found, err := s.repos.Settings.Get(ctx, llmEndpointsKey)
	if err != nil || !found || raw == "" {
		return map[string]string{}, err
	}

	var endpoints map[string]string
	if err := json.Unmarshal([]byte(raw), &endpoints); err != nil {
		return map[string]string{}, nil
	}
	return endpoints, nil
}

// SaveEndpoints writes the per-provider base URLs.
func (s *Service) SaveEndpoints(ctx context.Context, endpoints map[string]string) error {
	cleaned := map[string]string{}
	for providerID, endpoint := range endpoints {
		trimmedProvider := strings.TrimSpace(providerID)
		trimmedEndpoint := strings.TrimSpace(endpoint)
		if trimmedProvider == "" || trimmedEndpoint == "" {
			continue
		}
		cleaned[trimmedProvider] = trimmedEndpoint
	}

	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return fmt.Errorf("settings: encode endpoints: %w", err)
	}
	return s.repos.Settings.Set(ctx, llmEndpointsKey, string(encoded), s.clock.Now())
}

// Export returns the settings a backup may carry.
//
// It filters rather than returning everything, so a key added later is withheld
// until someone decides it is safe. The decision is written down in one place
// instead of being implied by whatever the store happens to hold.
func (s *Service) Export(ctx context.Context) (map[string]string, error) {
	if s.repos.Settings == nil {
		return map[string]string{}, nil
	}

	all, err := s.repos.Settings.All(ctx)
	if err != nil {
		return nil, err
	}

	exported := make(map[string]string, len(ExportableKeys))
	for key, value := range all {
		if IsExportable(key) {
			exported[key] = value
		}
	}
	return exported, nil
}

// Withheld returns the keys that exist but are never exported.
//
// The UI shows this count, so a user can see that their endpoint configuration is
// deliberately not in the file they are about to share.
func (s *Service) Withheld(ctx context.Context) ([]string, error) {
	if s.repos.Settings == nil {
		return nil, nil
	}

	all, err := s.repos.Settings.All(ctx)
	if err != nil {
		return nil, err
	}

	var withheld []string
	for key := range all {
		if IsSensitive(key) {
			withheld = append(withheld, key)
		}
	}
	return withheld, nil
}

// Now reports the service's clock reading, so a caller that persists alongside it
// uses one clock.
func (s *Service) Now() time.Time { return s.clock.Now() }
