package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Templates for configuration files

// SettingsTemplate is the template for settings.yaml
const SettingsTemplate = `# Termhome Settings
---
title: Termhome Dashboard
description: A simple terminal homepage dashboard
theme: dark
showStats: false
hideVersion: false
status:
  checkInterval: 10 # Default status check interval in seconds, overrides individual services if set

# Layout configuration example (uncomment to use)
# layout:
#   Applications:
#     style: row
#     columns: 3
#   Documentation:
#     style: column
#     iconsOnly: true
`

// ServicesTemplate is the template for services.yaml
const ServicesTemplate = `# Termhome Services Configuration
---
- Applications:
    - GitHub:
        icon: github
        href: https://github.com
        description: Where the world builds software
        siteMonitor: https://github.com
        siteMonitorMethod: HEAD
        siteMonitorTimeout: 5
        siteMonitorInterval: 60
        siteMonitorExpectedCodes: [200, 301, 302]
    - ChatGPT:
        icon: openai
        href: https://chatgpt.com
        description: The AI chatbot
        ping: chatgpt.com
        pingCount: 3
        pingInterval: 60

- Development Tools:
    - V0:
        icon: v0
        href: https://v0.dev
        siteMonitor: https://v0.dev
        siteMonitorMethod: GET
        siteMonitorTimeout: 10
        siteMonitorInterval: 60
        siteMonitorExpectedCodes: [429]
        siteMonitorHeaders:
            User-Agent: "Termhome/1.0"

- Monitoring:
    - Local Network:
        icon: network
        description: Local network status
        ping: 192.168.1.1
        pingInterval: 30
`

// BookmarksTemplate is the template for bookmarks.yaml
const BookmarksTemplate = `# Termhome Bookmarks Configuration
---
- Documentation:
    - Termhome Docs:
        icon: github
        href: https://github.com/deblasis/termhome
        description: Termhome documentation

- Search Engines:
    - Google:
        abbr: G
        icon: google
        href: https://google.com
        description: Google Search
    - DuckDuckGo:
        abbr: DDG
        icon: duckduckgo
        href: https://duckduckgo.com
        description: Privacy-focused search engine
    - Perplexity:
        abbr: P
        icon: perplexity
        href: https://perplexity.com
        description: AI-powered search engine
`

// DockerTemplate is the template for docker.yaml
const DockerTemplate = `# Direct Docker socket connection
local-docker:
  socket: /var/run/docker.sock
  interval: 10  # Check every 10 seconds

# Connection via Docker proxy (uncomment to enable)
# docker-proxy:
#   host: dockerproxy
#   port: 2375
`

// GetAllTemplates returns a map of filenames to their template content
func GetAllTemplates() map[string]string {
	return map[string]string{
		"settings.yaml":  SettingsTemplate,
		"services.yaml":  ServicesTemplate,
		"bookmarks.yaml": BookmarksTemplate,
		"docker.yaml":    DockerTemplate,
	}
}

// InitializeConfigDir creates a config directory with example configuration files
func InitializeConfigDir(configDir string) error {
	// Create the config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Get all templates
	templates := GetAllTemplates()

	// Write the files
	for filename, content := range templates {
		filePath := filepath.Join(configDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filePath, err)
		}
	}

	return nil
}
