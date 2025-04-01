package homepage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadSettings_ValidFile checks if a valid settings.yaml file is parsed correctly.
func TestLoadSettings_ValidFile(t *testing.T) {
	testContent := `
---
title: My Test Dashboard
description: Test Description
theme: light
status:
  checkInterval: 30
`
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "settings.yaml")

	err := os.WriteFile(tempFile, []byte(testContent), 0644)
	assert.NoError(t, err, "Failed to write temporary settings file")

	settings, err := LoadSettings(tempFile)

	assert.NoError(t, err, "LoadSettings returned an error for a valid file")
	assert.NotNil(t, settings, "LoadSettings returned nil settings for a valid file")

	// Assert specific values
	assert.Equal(t, "My Test Dashboard", settings.Title, "Parsed title does not match")
	assert.Equal(t, "Test Description", settings.Description, "Parsed description does not match")
	assert.Equal(t, "light", settings.Theme, "Parsed theme does not match")
	assert.Equal(t, 30, settings.Status.CheckInterval, "Parsed checkInterval does not match")

}

// TestLoadSettings_NonExistentFile checks behavior when the settings file doesn't exist.
func TestLoadSettings_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentFile := filepath.Join(tempDir, "nonexistent_settings.yaml")

	settings, err := LoadSettings(nonExistentFile)

	assert.NoError(t, err, "LoadSettings returned an error when file does not exist")
	assert.NotNil(t, settings, "LoadSettings returned nil settings when file does not exist")

	// Check for default values
	assert.Equal(t, "Termhome Dashboard", settings.Title, "Default title is incorrect")
	assert.Equal(t, "dark", settings.Theme, "Default theme is incorrect")
	assert.Equal(t, 60, settings.Status.CheckInterval, "Default checkInterval is incorrect")
}

// TestLoadSettings_InvalidYAML checks behavior with malformed YAML.
func TestLoadSettings_InvalidYAML(t *testing.T) {
	invalidContent := `title: My Test Dashboard
this is not valid yaml: :
`
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "invalid_settings.yaml")

	err := os.WriteFile(tempFile, []byte(invalidContent), 0644)
	assert.NoError(t, err, "Failed to write temporary invalid settings file")

	settings, err := LoadSettings(tempFile)

	assert.Error(t, err, "LoadSettings should return an error for invalid YAML")
	assert.Nil(t, settings, "LoadSettings should return nil settings for invalid YAML")
}

// TestLoadServices_TwoGroupsOneServiceEach verifies parsing of a simple two-group, one-service-each config.
func TestLoadServices_TwoGroupsOneServiceEach(t *testing.T) {
	testContent := `
- Group A:
    - Service A:
        href: http://service-a.com/
        description: First service

- Group B:
    - Service B:
        href: http://service-b.org/
        icon: custom-icon
`
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "services.yaml")

	err := os.WriteFile(tempFile, []byte(testContent), 0644)
	assert.NoError(t, err, "Failed to write temporary services file")

	serviceGroups, err := LoadServices(tempFile)

	assert.NoError(t, err, "LoadServices returned an error for a valid file")
	assert.NotNil(t, serviceGroups, "LoadServices returned nil for a valid file")
	assert.Len(t, serviceGroups, 2, "Expected 2 service groups to be parsed")

	// Find Group A (order isn't guaranteed by map iteration during parse)
	var groupA *ServiceGroup
	for _, g := range serviceGroups {
		if g.Name == "Group A" {
			groupA = g
			break
		}
	}
	assert.NotNil(t, groupA, "Group A not found in parsed groups")
	if groupA != nil {
		assert.Len(t, groupA.Services, 1, "Expected 1 service in Group A")
		if len(groupA.Services) == 1 {
			assert.Equal(t, "Service A", groupA.Services[0].Name, "Service name in Group A mismatch")
			assert.Equal(t, "http://service-a.com/", groupA.Services[0].Href, "Service href in Group A mismatch")
			assert.Equal(t, "First service", groupA.Services[0].Description, "Service description in Group A mismatch")
		}
	}

	// Find Group B
	var groupB *ServiceGroup
	for _, g := range serviceGroups {
		if g.Name == "Group B" {
			groupB = g
			break
		}
	}
	assert.NotNil(t, groupB, "Group B not found in parsed groups")
	if groupB != nil {
		assert.Len(t, groupB.Services, 1, "Expected 1 service in Group B")
		if len(groupB.Services) == 1 {
			assert.Equal(t, "Service B", groupB.Services[0].Name, "Service name in Group B mismatch")
			assert.Equal(t, "http://service-b.org/", groupB.Services[0].Href, "Service href in Group B mismatch")
			assert.Equal(t, "custom-icon", groupB.Services[0].Icon, "Service icon in Group B mismatch")
		}
	}
}

// TestConvertServicesData verifies the helper function for converting service data.
func TestConvertServicesData(t *testing.T) {
	groupData := []interface{}{
		map[string]interface{}{ // Service One - No ping
			"Service One": map[string]interface{}{ // Use map[string]interface{} for properties
				"href":        "http://one.com",
				"description": "Desc 1",
			},
		},
		map[string]interface{}{ // Service Two - With string ping
			"Service Two": map[string]interface{}{ // Use map[string]interface{} for properties
				"href": "http://two.net",
				"icon": "icon-two",
				"ping": "two.net", // Ping is now just a string
				// Optionally add other ping fields if needed for the test
				// "pingCount": 3,
				// "pingInterval": 30,
			},
		},
	}

	services, err := convertServicesData(groupData)

	assert.NoError(t, err, "convertServicesData returned an error")
	assert.NotNil(t, services, "convertServicesData returned nil services")
	assert.Len(t, services, 2, "Expected 2 services to be converted")

	// Check Service One
	assert.Equal(t, "Service One", services[0].Name)
	assert.Equal(t, "http://one.com", services[0].Href)
	assert.Equal(t, "Desc 1", services[0].Description)
	assert.Equal(t, "", services[0].Ping, "Service One Ping string should be empty")

	// Check Service Two
	assert.Equal(t, "Service Two", services[1].Name)
	assert.Equal(t, "http://two.net", services[1].Href)
	assert.Equal(t, "icon-two", services[1].Icon)
	assert.NotEqual(t, "", services[1].Ping, "Service Two Ping string should not be empty")
	assert.Equal(t, "two.net", services[1].Ping) // Check the Ping string directly
}

// TestConvertBookmarksData verifies the helper function for converting bookmark data.
func TestConvertBookmarksData(t *testing.T) {
	// This groupData MUST match the nested list structure expected by the corrected convertBookmarksData
	// [ { "Name": [ { "prop": "val" } ] } ]
	groupData := []interface{}{
		map[string]interface{}{ // Bookmark 1: Github
			"Github": []interface{}{ // List containing the properties map
				map[string]interface{}{
					"abbr": "GH",
					"href": "https://github.com/",
				},
			},
		},
		map[string]interface{}{ // Bookmark 2: Reddit
			"Reddit": []interface{}{ // List containing the properties map
				map[string]interface{}{
					"icon":        "reddit.png",
					"href":        "https://reddit.com/",
					"description": "The front page of the internet",
				},
			},
		},
		map[string]interface{}{ // Bookmark 3: YouTube (with only href)
			"YouTube": []interface{}{ // List containing the properties map
				map[string]interface{}{
					"href": "https://youtube.com/",
				},
			},
		},
	}

	bookmarks, err := convertBookmarksData(groupData)

	assert.NoError(t, err, "convertBookmarksData returned an error")
	assert.NotNil(t, bookmarks, "convertBookmarksData returned nil bookmarks")
	assert.Len(t, bookmarks, 3, "Expected 3 bookmarks to be converted")

	// Check Github
	assert.Equal(t, "Github", bookmarks[0].Name)
	assert.Equal(t, "https://github.com/", bookmarks[0].Href)
	assert.Equal(t, "GH", bookmarks[0].Abbr)
	assert.Equal(t, "", bookmarks[0].Icon)
	assert.Equal(t, "", bookmarks[0].Description)

	// Check Reddit
	assert.Equal(t, "Reddit", bookmarks[1].Name)
	assert.Equal(t, "https://reddit.com/", bookmarks[1].Href)
	assert.Equal(t, "reddit.png", bookmarks[1].Icon)
	assert.Equal(t, "The front page of the internet", bookmarks[1].Description)
	assert.Equal(t, "", bookmarks[1].Abbr)

	// Check YouTube
	assert.Equal(t, "YouTube", bookmarks[2].Name)
	assert.Equal(t, "https://youtube.com/", bookmarks[2].Href)
	assert.Equal(t, "", bookmarks[2].Abbr)
	assert.Equal(t, "", bookmarks[2].Icon)
	assert.Equal(t, "", bookmarks[2].Description)
}

// Helper function to find a bookmark group by name
// ... existing code ...
