package homepage

import (
	"fmt"
	"os"

	"github.com/deblasis/termhome/pkg/logging"
	"gopkg.in/yaml.v3"
)

// LoadSettings loads the settings configuration from the specified YAML file.
func LoadSettings(filePath string) (*Settings, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// It's okay if settings.yaml doesn't exist, return default settings
		if os.IsNotExist(err) {
			return &Settings{
				Title:       "Termhome Dashboard",
				Description: "A terminal homepage dashboard",
				Theme:       "dark",
				Status: StatusSettings{
					CheckInterval: 60, // Default 60 second interval
				},
			}, nil
		}
		return nil, fmt.Errorf("failed to read settings file %s: %w", filePath, err)
	}

	logging.Debug("Loading settings from %s", filePath)

	// Attempt to handle the format with the initial dash separator
	var settings Settings
	err = yaml.Unmarshal(data, &settings)
	if err != nil {
		logging.Warn("Failed to unmarshal settings directly: %v", err)

		// Try with a different approach - parse as a generic interface first
		var rawData interface{}
		if err := yaml.Unmarshal(data, &rawData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal settings file %s: %w", filePath, err)
		}

		// Convert the raw data to our format
		settingsData, err := yaml.Marshal(rawData)
		if err != nil {
			return nil, fmt.Errorf("failed to remarshal settings data: %w", err)
		}

		// Try to unmarshal again
		if err := yaml.Unmarshal(settingsData, &settings); err != nil {
			return nil, fmt.Errorf("failed to unmarshal converted settings data: %w", err)
		}
	}

	// Set defaults for missing fields
	if settings.Title == "" {
		settings.Title = "Termhome Dashboard"
	}

	if settings.Status.CheckInterval <= 0 {
		settings.Status.CheckInterval = 60 // Default 60 second interval
	}

	// If no theme is specified, default to dark
	if settings.Theme == "" {
		settings.Theme = "dark"
	}

	logging.Debug("Loaded settings: title=%s, theme=%s, interval=%d",
		settings.Title, settings.Theme, settings.Status.CheckInterval)

	return &settings, nil
}

// LoadServices loads the service configurations from the specified YAML file.
// It expects the format to be an array of groups, consistent with gethomepage.dev.
// Example:
// - Group1:
//   - Service1:
//     href: ...
//
// - Group2:
//   - ServiceA:
//     href: ...
func LoadServices(filePath string) ([]*ServiceGroup, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// If services.yaml doesn't exist, return an empty list
		if os.IsNotExist(err) {
			return []*ServiceGroup{}, nil
		}
		return nil, fmt.Errorf("failed to read services file %s: %w", filePath, err)
	}

	logging.Debug("Loading services from %s (expecting array format)", filePath)

	// Expect the format to be an array of maps, where each map represents a group.
	var arrayFormat []map[string]interface{}
	err = yaml.Unmarshal(data, &arrayFormat)

	if err != nil {
		// If unmarshaling fails, it's likely not the expected format or invalid YAML.
		return nil, fmt.Errorf("failed to unmarshal services file %s as array format: %w. Ensure it starts with a '-' for each group", filePath, err)
	}

	// Process the array format
	logging.Debug("Parsing services file as array format (gethomepage style with dashes)")
	var serviceGroups []*ServiceGroup

	// Process each group entry in the array
	for i, groupEntry := range arrayFormat {
		if len(groupEntry) != 1 {
			logging.Warn("Service group entry at index %d does not have exactly one key, skipping.", i)
			continue // Expecting map like {"Group Name": [services...]}
		}

		for groupName, groupData := range groupEntry {
			// Convert the services within this group
			services, err := convertServicesData(groupData) // Use existing helper
			if err != nil {
				logging.Warn("Error converting services for group '%s': %v", groupName, err)
				continue // Skip group if services conversion fails
			}

			// Add the parsed group to the list
			group := &ServiceGroup{
				Name:     groupName,
				Services: services,
			}
			serviceGroups = append(serviceGroups, group)
		}
	}

	logging.Debug("Loaded %d service groups using array format", len(serviceGroups))
	return serviceGroups, nil
}

// Helper function to convert group data to services
func convertServicesData(groupData interface{}) ([]*Service, error) {
	// The groupData is a list of service maps
	servicesList, ok := groupData.([]interface{})
	if !ok {
		return nil, fmt.Errorf("group data is not a list")
	}

	var services []*Service

	for _, serviceEntry := range servicesList {
		// Each service entry is a map with a single key (the service name)
		serviceMap, ok := serviceEntry.(map[string]interface{})
		if !ok {
			logging.Warn("Service entry is not a map, skipping")
			continue
		}

		for serviceName, serviceData := range serviceMap {
			// Create a temporary map with the service name included
			serviceWithName := map[string]interface{}{
				"name": serviceName,
			}

			// Copy all the service properties
			if servicePropsMap, ok := serviceData.(map[string]interface{}); ok {
				for k, v := range servicePropsMap {
					serviceWithName[k] = v
				}
			}

			// Marshal and unmarshal to convert to our Service struct
			marshaledData, err := yaml.Marshal(serviceWithName)
			if err != nil {
				logging.Warn("Failed to marshal service data: %v", err)
				continue
			}

			var service Service
			if err = yaml.Unmarshal(marshaledData, &service); err != nil {
				logging.Warn("Failed to unmarshal service: %v", err)
				continue
			}

			services = append(services, &service)
		}
	}

	return services, nil
}

// LoadBookmarks loads the bookmark configurations from the specified YAML file.
func LoadBookmarks(filePath string) ([]*BookmarkGroup, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// If bookmarks.yaml doesn't exist, return an empty list
		if os.IsNotExist(err) {
			return []*BookmarkGroup{}, nil
		}
		return nil, fmt.Errorf("failed to read bookmarks file %s: %w", filePath, err)
	}

	logging.Debug("Loading bookmarks from %s", filePath)

	// Try to parse as a list of maps where each map is a group (gethomepage format, starting with dashes)
	// Example:
	// - Group1:
	//     - Bookmark1:
	//         href: ...
	var arrayFormat []map[string]interface{}
	err = yaml.Unmarshal(data, &arrayFormat)

	if err == nil && len(arrayFormat) > 0 {
		logging.Debug("Parsed bookmarks file as array format (gethomepage style with dashes)")
		var bookmarkGroups []*BookmarkGroup

		// Process each group in the array
		for _, groupEntry := range arrayFormat {
			for groupName, groupData := range groupEntry {
				// Convert the bookmarks to our format
				bookmarks, err := convertBookmarksData(groupData)
				if err != nil {
					logging.Warn("Error converting bookmarks for group %s: %v", groupName, err)
					continue
				}

				// Add group to the list
				group := &BookmarkGroup{
					Name:      groupName,
					Bookmarks: bookmarks,
				}
				bookmarkGroups = append(bookmarkGroups, group)
			}
		}

		logging.Debug("Loaded %d bookmark groups using array format", len(bookmarkGroups))
		return bookmarkGroups, nil
	}

	// If the array format failed, try the map format
	logging.Debug("Array format parsing failed, trying map format")
	var bookmarksConfig BookmarksConfig
	err = yaml.Unmarshal(data, &bookmarksConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal bookmarks file %s: %w", filePath, err)
	}

	// Convert map to slice of BookmarkGroup for easier iteration
	var bookmarkGroups []*BookmarkGroup
	for groupName, bookmarks := range bookmarksConfig {
		// Ensure bookmarks slice is not nil if group exists but is empty
		if bookmarks == nil {
			bookmarks = []*Bookmark{}
		}
		group := &BookmarkGroup{
			Name:      groupName,
			Bookmarks: bookmarks,
		}
		bookmarkGroups = append(bookmarkGroups, group)
	}

	logging.Debug("Loaded %d bookmark groups", len(bookmarkGroups))
	return bookmarkGroups, nil
}

// Helper function to convert group data to bookmarks
func convertBookmarksData(groupData interface{}) ([]*Bookmark, error) {
	// The groupData is a list of bookmark maps
	bookmarksList, ok := groupData.([]interface{})
	if !ok {
		return nil, fmt.Errorf("group data is not a list")
	}

	var bookmarks []*Bookmark

	for _, bookmarkEntry := range bookmarksList {
		// Each bookmark entry is a map with a single key (the bookmark name)
		bookmarkMap, ok := bookmarkEntry.(map[string]interface{})
		if !ok {
			logging.Warn("Bookmark entry is not a map, skipping")
			continue
		}

		for bookmarkName, bookmarkData := range bookmarkMap {
			// Create a temporary map with the bookmark name included
			bookmarkWithName := map[string]interface{}{
				"name": bookmarkName,
			}

			// Copy all the bookmark properties
			if bookmarkPropsMap, ok := bookmarkData.(map[string]interface{}); ok {
				for k, v := range bookmarkPropsMap {
					bookmarkWithName[k] = v
				}
			}

			// Marshal and unmarshal to convert to our Bookmark struct
			marshaledData, err := yaml.Marshal(bookmarkWithName)
			if err != nil {
				logging.Warn("Failed to marshal bookmark data: %v", err)
				continue
			}

			var bookmark Bookmark
			if err = yaml.Unmarshal(marshaledData, &bookmark); err != nil {
				logging.Warn("Failed to unmarshal bookmark: %v", err)
				continue
			}

			bookmarks = append(bookmarks, &bookmark)
		}
	}

	return bookmarks, nil
}

// LoadDockerConfig loads the Docker configuration from the specified YAML file.
func LoadDockerConfig(filePath string) (*DockerConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// If docker.yaml doesn't exist, return nil without error
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read Docker config file %s: %w", filePath, err)
	}

	var dockerConfig map[string]*DockerConfig
	err = yaml.Unmarshal(data, &dockerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Docker config file %s: %w", filePath, err)
	}

	// For now we'll just use the first Docker configuration found
	// In a future enhancement we could handle multiple Docker servers
	for _, config := range dockerConfig {
		if config != nil {
			// Set default interval if not specified
			if config.Interval <= 0 {
				config.Interval = 60 // Default to 60 seconds
			}
			return config, nil
		}
	}

	// Return a default config if the file exists but is empty
	return &DockerConfig{
		Interval: 60,
		Includes: []string{"*"},
	}, nil
}
