package homepage

import (
	"fmt"

	"github.com/deblasis/termhome/pkg/logging"
)

// Terminal hyperlink escape sequences
const (
	linkStart = "\033]8;;"
	linkMid   = "\007"
	linkEnd   = "\033]8;;\007"
)

// Wrap text in terminal hyperlink escape codes for clickable links
func makeClickableLink(displayText, url string) string {
	// For now, display with an indicator rather than using escape sequences
	// that may not be compatible with all terminals/SSH sessions
	return fmt.Sprintf("%s [%s]", displayText, url)
}

// Global status monitor instance
var statusMonitor *StatusMonitor

// SetStatusMonitor sets the global status monitor instance
func SetStatusMonitor(monitor *StatusMonitor) {
	statusMonitor = monitor
}

// Map to track which services belong to which groups
var serviceToGroupMap = make(map[string]string)

// RegisterGroupService records which group a service belongs to
func RegisterGroupService(serviceName string, groupName string) {
	serviceToGroupMap[serviceName] = groupName
}

// Global cache of service groups for layout rebuilding
var cachedServiceGroups []*ServiceGroup

// Global cache of bookmark groups for layout rebuilding
var cachedBookmarkGroups []*BookmarkGroup

// StoreCachedGroups stores the service groups for later use
func StoreCachedGroups(groups []*ServiceGroup) {
	cachedServiceGroups = groups
}

// StoreCachedBookmarks stores the bookmark groups for later use
func StoreCachedBookmarks(groups []*BookmarkGroup) {
	cachedBookmarkGroups = groups
}

// GetCachedGroups returns the cached service groups
func GetCachedGroups() []*ServiceGroup {
	return cachedServiceGroups
}

// GetCachedBookmarks returns the cached bookmark groups
func GetCachedBookmarks() []*BookmarkGroup {
	return cachedBookmarkGroups
}

// isDynamicGroup checks if a group is dynamically added (for now, Database is considered dynamic)
func isDynamicGroup(groupName string) bool {
	return groupName == "Database" || groupName == "Docker"
}

// RebuildUIFunc is a function that rebuilds the UI with updated groups
type RebuildUIFunc func()

// Global rebuild function
var rebuildUI RebuildUIFunc

// RegisterUIRebuildFunc registers a function to rebuild the UI when needed
func RegisterUIRebuildFunc(fn RebuildUIFunc) {
	rebuildUI = fn
}

// RequestUIRebuild requests a rebuild of the UI
func RequestUIRebuild() {
	if rebuildUI != nil {
		logging.Info("Requesting UI rebuild...")
		rebuildUI()
	} else {
		logging.Warn("UI rebuild requested but no rebuild function registered")
	}
}

// uiRebuildFunc is a function that rebuilds the UI
var uiRebuildFunc func() error

// RegisterUIRebuildFunc registers a function to rebuild the UI
func RegisterUIRebuildFuncWithError(rebuildFunc func() error) {
	uiRebuildFunc = rebuildFunc
}

// RequestUIRebuild requests a UI rebuild
func RequestUIRebuildWithError() error {
	if uiRebuildFunc != nil {
		return uiRebuildFunc()
	}
	return fmt.Errorf("UI rebuild function not registered")
}
