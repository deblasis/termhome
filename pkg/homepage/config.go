package homepage

// Settings represents the structure of the settings.yaml file.
type Settings struct {
	Title             string                 `yaml:"title"`             // Optional: Title of the homepage
	Description       string                 `yaml:"description"`       // Optional: Description of the homepage
	StartUrl          string                 `yaml:"startUrl"`          // Optional: Start URL for installable apps
	Background        string                 `yaml:"background"`        // Optional: Background image URL or path
	BackgroundOpacity float64                `yaml:"backgroundOpacity"` // Optional: Background opacity (0-1)
	BackgroundBlur    int                    `yaml:"backgroundBlur"`    // Optional: Background blur amount
	CardBlur          int                    `yaml:"cardBlur"`          // Optional: Card background blur
	Favicon           string                 `yaml:"favicon"`           // Optional: Favicon URL or path
	Theme             string                 `yaml:"theme"`             // Optional: Theme (dark/light)
	Color             string                 `yaml:"color"`             // Optional: Color palette
	Layout            map[string]GroupLayout `yaml:"layout"`            // Optional: Layout configuration
	HeaderStyle       string                 `yaml:"headerStyle"`       // Optional: Header style
	BaseURL           string                 `yaml:"baseUrl"`           // Optional: Base URL for relative links
	Language          string                 `yaml:"language"`          // Optional: Interface language
	LinkTarget        string                 `yaml:"linkTarget"`        // Optional: Link target (_blank, _self, etc.)
	HideVersion       bool                   `yaml:"hideVersion"`       // Optional: Hide version display
	ShowStats         bool                   `yaml:"showStats"`         // Optional: Show Docker stats
	BookmarksStyle    string                 `yaml:"bookmarksStyle"`    // Optional: Bookmarks style (default/icons)
	Status            StatusSettings         `yaml:"status"`            // Optional: Status monitoring settings
	InstanceName      string                 `yaml:"instanceName"`      // Optional: Instance name
	HideErrors        bool                   `yaml:"hideErrors"`        // Optional: Hide widget error messages
}

// GroupLayout holds layout configuration for a service or bookmark group
type GroupLayout struct {
	Style       string `yaml:"style"`       // Optional: Layout style (row/column)
	Columns     int    `yaml:"columns"`     // Optional: Number of columns
	IconsOnly   bool   `yaml:"iconsOnly"`   // Optional: Icons only mode for bookmarks
	Collapsible bool   `yaml:"collapsible"` // Optional: Make section collapsible
	Collapsed   bool   `yaml:"collapsed"`   // Optional: Initial collapsed state
	EqualHeight bool   `yaml:"equalHeight"` // Optional: Use equal height cards
}

// StatusSettings holds global status monitoring settings
type StatusSettings struct {
	CheckInterval int                    `yaml:"checkInterval"` // Global status check interval in seconds
	DefaultStyle  map[string]StatusStyle `yaml:"style"`         // Default status styles
}

// StatusStyle defines custom styling for status indicators
type StatusStyle struct {
	Icon  string `yaml:"icon"`  // Custom icon for this status
	Color string `yaml:"color"` // Custom color for this status
}

// Service represents a single service entry within a group in services.yaml.
type Service struct {
	Name                     string                 `yaml:"name"`                     // Required: Name of the service
	Href                     string                 `yaml:"href"`                     // Optional: URL for the service
	Description              string                 `yaml:"description"`              // Optional: Description shown below the name
	Icon                     string                 `yaml:"icon"`                     // Optional: Icon for the service
	Status                   string                 `yaml:"status"`                   // Optional: Custom field for static text status
	Ping                     string                 `yaml:"ping"`                     // Optional: Host to ping (simple ICMP check)
	PingCount                int                    `yaml:"pingCount"`                // Optional: Number of pings to send (default: 3)
	PingInterval             int                    `yaml:"pingInterval"`             // Optional: Ping interval in seconds (default: 60)
	SiteMonitor              string                 `yaml:"siteMonitor"`              // Optional: URL to monitor (simple HTTP check)
	SiteMonitorMethod        string                 `yaml:"siteMonitorMethod"`        // Optional: HTTP method for site monitor (default: HEAD)
	SiteMonitorTimeout       int                    `yaml:"siteMonitorTimeout"`       // Optional: Request timeout for site monitor in seconds (default: 10)
	SiteMonitorInterval      int                    `yaml:"siteMonitorInterval"`      // Optional: Check interval for site monitor in seconds (default: 60)
	SiteMonitorExpectedCodes []int                  `yaml:"siteMonitorExpectedCodes"` // Optional: HTTP codes to consider "up" (default: [200])
	SiteMonitorHeaders       map[string]string      `yaml:"siteMonitorHeaders"`       // Optional: Headers to include in the site monitor request
	SiteMonitorSkipVerify    bool                   `yaml:"siteMonitorSkipVerify"`    // Optional: Skip TLS certificate verification for site monitor
	StatusStyle              map[string]StatusStyle `yaml:"statusStyle"`              // Optional: Custom styling for status indicators
	DisableStatus            bool                   `yaml:"disableStatus"`            // Optional: Disable status monitoring for this service
	Server                   string                 `yaml:"server"`                   // Optional: Docker server reference
	Container                string                 `yaml:"container"`                // Optional: Docker container name
	ShowStats                bool                   `yaml:"showStats"`                // Optional: Show Docker stats
	Widget                   interface{}            `yaml:"widget"`                   // Optional: Widget configuration
	SubtitleURL              string                 `yaml:"subtitleUrl"`              // Optional: URL for subtitle content
}

// ServiceGroup represents a group of services in services.yaml.
// The key in the YAML map becomes the group name.
type ServiceGroup struct {
	Name     string
	Services []*Service // Slice of services in this group
}

// ServicesConfig represents the structure of the services.yaml file.
// It's a map where keys are group names and values are lists of services.
type ServicesConfig map[string][]*Service

// Bookmark represents a single bookmark entry within a group in bookmarks.yaml.
type Bookmark struct {
	Name        string `yaml:"name"`        // Optional: Display name of the bookmark
	Abbr        string `yaml:"abbr"`        // Optional: Abbreviation, used if name is missing
	Href        string `yaml:"href"`        // Required: URL for the bookmark
	Description string `yaml:"description"` // Optional: Description shown on hover/tooltip (or below name)
	Icon        string `yaml:"icon"`        // Optional: Icon for the bookmark
}

// BookmarkGroup represents a group of bookmarks in bookmarks.yaml.
// The key in the YAML map becomes the group name.
type BookmarkGroup struct {
	Name      string
	Bookmarks []*Bookmark // Slice of bookmarks in this group
}

// BookmarksConfig represents the structure of the bookmarks.yaml file.
// It's a map where keys are group names and values are lists of bookmarks.
type BookmarksConfig map[string][]*Bookmark

// DockerConfig holds configuration for Docker container monitoring
type DockerConfig struct {
	Socket               string   `yaml:"socket"`               // Unix socket path
	Host                 string   `yaml:"host"`                 // Remote host
	Port                 int      `yaml:"port"`                 // Remote port
	Interval             int      `yaml:"interval"`             // Check interval in seconds
	Includes             []string `yaml:"includes"`             // Container name patterns to include
	Excludes             []string `yaml:"excludes"`             // Container name patterns to exclude
	DisableAutodiscovery bool     `yaml:"disableAutodiscovery"` // Disable auto-discovery based on labels
}

// DockerContainer represents a single Docker container
type DockerContainer struct {
	ID      string   `yaml:"id"`      // Container ID
	Name    string   `yaml:"name"`    // Container name
	Image   string   `yaml:"image"`   // Image name
	Status  string   `yaml:"status"`  // Running status
	Created int64    `yaml:"created"` // Creation timestamp
	Ports   []string `yaml:"ports"`   // Exposed ports
}
