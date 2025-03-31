package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/deblasis/termhome/pkg/config"
	"github.com/deblasis/termhome/pkg/homepage"
	"github.com/deblasis/termhome/pkg/logging"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Global variables for UI management
var (
	// Context for termination
	globalCtx    context.Context
	globalCancel context.CancelFunc

	// tview application
	app *tview.Application

	// Flag to indicate if app is fully initialized
	appInitialized bool

	// Root container
	mainContainer *tview.Flex

	// Service views for updates
	serviceViews map[string]*tview.TextView

	// Global settings
	globalSettings *homepage.Settings

	// Navigation state
	currentFocus      tview.Primitive
	allFocusableBoxes []tview.Primitive
	isMaximized       bool
	maximizedBox      tview.Primitive
	originalLayout    *tview.Flex

	// Mouse handling for double-click
	lastClickTime    int64
	lastClickedBox   tview.Primitive
	doubleClickDelay int64 = 500 // Milliseconds
)

func init() {
	// Initialize logging with defaults - send to file only
	logOpts := logging.DefaultOptions()
	logOpts.LogFileName = "termhome.log"

	// Create log directory if it doesn't exist
	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err == nil {
		logOpts.LogDir = logDir
	}

	logger := logging.New(logOpts)
	logging.SetGlobalLogger(logger)

	// Replace standard library logger to capture logs from other packages
	logging.ReplaceStdLogger()

	// Initialize service views map
	serviceViews = make(map[string]*tview.TextView)
}

func main() {
	logging.Info("Starting Termhome")

	// --- Command line handling ---
	// Check if there's a subcommand
	if len(os.Args) > 1 && os.Args[1] == "init" {
		// Parse flags for the init subcommand
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		configDirInit := initCmd.String("config-dir", "./config", "Directory to create example configuration files in")
		if len(os.Args) > 2 {
			initCmd.Parse(os.Args[2:])
		}

		// Initialize configs
		if err := config.InitializeConfigDir(*configDirInit); err != nil {
			logging.Fatal("Failed to initialize example configs: %v", err)
		}
		logging.Info("Example configuration files created in %s", *configDirInit)
		fmt.Printf("Example configuration files created in %s\n", *configDirInit)
		return
	}

	// Main application flags
	mainCmd := flag.NewFlagSet("termhome", flag.ExitOnError)
	configDir := mainCmd.String("config-dir", "./config", "Directory containing the configuration files")
	logLevel := mainCmd.String("log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR, FATAL)")
	mainCmd.Parse(os.Args[1:])

	// Set log level from command line
	logging.SetGlobalLogLevel(logging.ParseLogLevel(*logLevel))

	logging.Info("Using config directory: %s", *configDir)

	// --- Configuration paths ---
	settingsPath := filepath.Join(*configDir, "settings.yaml")
	servicesPath := filepath.Join(*configDir, "services.yaml")
	bookmarksPath := filepath.Join(*configDir, "bookmarks.yaml")
	dockerPath := filepath.Join(*configDir, "docker.yaml")

	logging.Info("Config paths: settings=%s, services=%s, bookmarks=%s, docker=%s",
		settingsPath, servicesPath, bookmarksPath, dockerPath)

	// Load settings
	settings, err := homepage.LoadSettings(settingsPath)
	if err != nil {
		logging.Fatal("Error loading settings: %v", err)
	} else {
		logging.Info("Settings loaded successfully.")
	}

	// Store settings globally
	globalSettings = settings

	// Load service groups
	serviceGroups, err := homepage.LoadServices(servicesPath)
	if err != nil {
		logging.Warn("Warning: Error loading services: %v", err)
		serviceGroups = []*homepage.ServiceGroup{}
	} else {
		logging.Info("Services loaded successfully: %d groups found.", len(serviceGroups))
	}

	// Load bookmark groups
	bookmarkGroups, err := homepage.LoadBookmarks(bookmarksPath)
	if err != nil {
		logging.Warn("Warning: Error loading bookmarks: %v", err)
		bookmarkGroups = []*homepage.BookmarkGroup{}
	} else {
		logging.Info("Bookmarks loaded successfully: %d groups found.", len(bookmarkGroups))
	}

	// Load Docker configuration
	dockerConfig, err := homepage.LoadDockerConfig(dockerPath)
	if err != nil {
		logging.Warn("Warning: Error loading Docker config: %v", err)
	} else if dockerConfig != nil {
		logging.Info("Docker config loaded successfully.")
	}

	// Create a context that will be canceled when the program exits
	ctx, cancel := context.WithCancel(context.Background())
	globalCtx = ctx
	globalCancel = cancel

	// Store groups for status updates
	homepage.StoreCachedGroups(serviceGroups)
	homepage.StoreCachedBookmarks(bookmarkGroups)

	// Initialize status monitor
	logging.Info("Initializing status monitor...")
	statusMonitor := homepage.NewStatusMonitor(statusUpdateCallback)
	homepage.SetStatusMonitor(statusMonitor) // Set global monitor
	defer statusMonitor.Stop()               // Ensure it stops when program exits

	// Set the global interval if configured
	if settings.Status.CheckInterval > 0 {
		statusMonitor.SetGlobalInterval(settings.Status.CheckInterval)
	}

	// Check if we have any content to display, and show a message if not
	noServices := len(serviceGroups) == 0
	noBookmarks := len(bookmarkGroups) == 0

	if noServices && noBookmarks {
		app = tview.NewApplication()
		messageBox := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetText(fmt.Sprintf("[yellow::b]Welcome to Termhome![:-:-]\n\n"+
				"[red]No services or bookmarks are defined.[:-:-]\n\n"+
				"Please adjust the configuration in [green]%s[:-:-]\n\n"+
				"[red]If you want example configuration files,\n"+
				"run [green]termhome init[:-:-]", *configDir))

		messageBox.SetBorder(true)

		// Add a simple key handler to quit
		app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape || event.Rune() == 'q' || event.Rune() == 'Q' {
				app.Stop()
				return nil
			}
			return event
		})

		if err := app.SetRoot(messageBox, true).Run(); err != nil {
			logging.Fatal("Application error: %v", err)
		}
		return
	}

	// Initialize the application
	app = tview.NewApplication()

	// Create main container
	mainContainer = createMainContainer(settings, serviceGroups, bookmarkGroups)

	// Run Docker autodiscovery if configured
	if dockerConfig != nil {
		logging.Info("Running initial Docker container autodiscovery...")
		if err := statusMonitor.RunInitialDockerDiscovery(dockerConfig); err != nil {
			logging.Warn("Failed to run initial Docker discovery: %v", err)
		} else {
			logging.Info("Initial Docker discovery completed successfully.")
		}

		// Add Docker monitoring for ongoing updates
		if err := statusMonitor.AddDockerMonitoring(dockerConfig); err != nil {
			logging.Warn("Failed to initialize Docker monitoring: %v", err)
		} else {
			logging.Info("Docker monitoring initialized successfully.")
		}
	}

	// Add services to the status monitor
	for _, group := range serviceGroups {
		for _, service := range group.Services {
			if service.Name == "" {
				continue
			}
			statusMonitor.AddService(service)
		}
	}

	// Set app as initialized
	appInitialized = true

	// Handle OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logging.Info("Received signal, shutting down...")
		cancel()
		app.Stop()
	}()

	// Set up key handlers
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Global key handlers
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' || event.Rune() == 'Q' {
			app.Stop()
			return nil
		}

		// Tab key to cycle focus between boxes
		if event.Key() == tcell.KeyTab {
			if len(allFocusableBoxes) > 0 {
				// Find current focus index
				focusIndex := -1
				for i, box := range allFocusableBoxes {
					if box == currentFocus {
						focusIndex = i
						break
					}
				}

				// Move to next box
				nextIndex := (focusIndex + 1) % len(allFocusableBoxes)
				currentFocus = allFocusableBoxes[nextIndex]
				app.SetFocus(currentFocus)
			}
			return nil
		}

		// Arrow keys for navigation between boxes
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown ||
			event.Key() == tcell.KeyLeft || event.Key() == tcell.KeyRight {
			navigateWithArrows(event.Key())
			return nil
		}

		// Space key to maximize/restore focused box
		if event.Rune() == ' ' {
			toggleMaximize()
			return nil
		}

		return event
	})

	// Run the application
	if err := app.SetRoot(mainContainer, true).EnableMouse(true).Run(); err != nil {
		logging.Fatal("Application error: %v", err)
	}

	logging.Info("Termhome exiting...")
}

// createMainContainer creates the main UI with individual boxes
func createMainContainer(settings *homepage.Settings, serviceGroups []*homepage.ServiceGroup, bookmarkGroups []*homepage.BookmarkGroup) *tview.Flex {
	// Create a flex container for the main layout
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Create header with title
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("[yellow::b]%s[-]", settings.Title))
	header.SetBorder(true)

	// Create content area split into services and bookmarks columns
	contentFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn)

	// Initialize the focusable boxes list
	allFocusableBoxes = []tview.Primitive{}

	// Create services panel if available
	if len(serviceGroups) > 0 {
		servicesPanel := createServicesPanel(serviceGroups)
		contentFlex.AddItem(servicesPanel, 0, 1, true) // Weight 1, focusable
	}

	// Create bookmarks panel if available
	if len(bookmarkGroups) > 0 {
		bookmarksPanel := createBookmarksPanel(bookmarkGroups)
		contentFlex.AddItem(bookmarksPanel, 0, 1, false) // Weight 1, not focusable by default
	}

	// Create footer with help - smaller, just text
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[red]Q/Esc: Quit | Tab/Arrows: Navigate | Space/DoubleClick: Maximize[-]")

	// No border for status bar, make it smaller
	footer.SetBorder(false)

	// Add components to main layout
	mainFlex.AddItem(header, 3, 1, false)     // Height 3, not focusable
	mainFlex.AddItem(contentFlex, 0, 1, true) // Expand to fill space, focusable
	mainFlex.AddItem(footer, 1, 1, false)     // Height 1, not focusable - reduced from 3 to 1

	// Save original layout for maximize/restore
	originalLayout = mainFlex

	// Set first box as current focus if available
	if len(allFocusableBoxes) > 0 {
		currentFocus = allFocusableBoxes[0]
		app.SetFocus(currentFocus)
	}

	return mainFlex
}

// createServicesPanel creates a panel with service groups
func createServicesPanel(serviceGroups []*homepage.ServiceGroup) tview.Primitive {
	// Create a flex layout for services
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Add title
	title := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[::b]Services[-]")
	title.SetBorder(true)
	flex.AddItem(title, 3, 1, false)

	// Add each service group
	for _, group := range serviceGroups {
		// Create a box for the group
		groupView := createServiceGroupBox(group)
		flex.AddItem(groupView, 0, 1, false)
	}

	return flex
}

// createServiceGroupBox creates a box for a single service group
func createServiceGroupBox(group *homepage.ServiceGroup) *tview.TextView {
	// Create text view with scrolling
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true).
		SetScrollable(true)

	// Set border with title
	textView.SetBorder(true).
		SetTitle(group.Name).
		SetTitleColor(tcell.ColorGreen)

	// Save the view for updates
	serviceViews[group.Name] = textView

	// Make it focusable and add keyboard handler for scrolling
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			// Let main handler handle this
			return event
		}
		// Let arrow keys be handled by main handler
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown ||
			event.Key() == tcell.KeyLeft || event.Key() == tcell.KeyRight {
			return event
		}
		return event
	})

	// Add mouse capture for double-click
	textView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			currentTime := time.Now().UnixNano() / int64(time.Millisecond)

			// Check if this is a double click on the same box
			if textView == lastClickedBox && currentTime-lastClickTime < doubleClickDelay {
				// Double click detected, toggle maximize
				toggleMaximize()
				lastClickTime = 0 // Reset to prevent triple-click detection
			} else {
				// First click, record time and box
				lastClickTime = currentTime
				lastClickedBox = textView

				// Focus the clicked box
				currentFocus = textView
				app.SetFocus(currentFocus)
			}
		}
		return action, event
	})

	// Set custom draw function to draw scrollbar
	textView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		// Call original drawing first
		left, top, innerWidth, innerHeight := textView.Box.GetInnerRect()

		// Draw scrollbar if needed
		rows, _ := textView.GetScrollOffset()
		totalRows := len(strings.Split(textView.GetText(false), "\n"))

		if totalRows > innerHeight {
			// Calculate scrollbar position and size
			scrollHeight := innerHeight - 2 // Adjust for arrows
			scrollPosition := int(float64(rows) / float64(totalRows) * float64(scrollHeight))
			scrollSize := int(float64(innerHeight) / float64(totalRows) * float64(scrollHeight))
			if scrollSize < 1 {
				scrollSize = 1
			}

			// Draw up arrow at top
			screen.SetContent(left+innerWidth-1, top, '▲', nil, tcell.StyleDefault.Foreground(tcell.ColorGray))

			// Draw scrollbar track and thumb
			for i := 0; i < scrollHeight; i++ {
				if i >= scrollPosition && i < scrollPosition+scrollSize {
					screen.SetContent(left+innerWidth-1, top+i+1, '█', nil, tcell.StyleDefault.Foreground(tcell.ColorWhite))
				} else {
					screen.SetContent(left+innerWidth-1, top+i+1, '│', nil, tcell.StyleDefault.Foreground(tcell.ColorGray))
				}
			}

			// Draw down arrow at bottom
			screen.SetContent(left+innerWidth-1, top+innerHeight-1, '▼', nil, tcell.StyleDefault.Foreground(tcell.ColorGray))
		}

		// Return original inner rectangle
		return left, top, innerWidth, innerHeight
	})

	// Add to focusable boxes
	allFocusableBoxes = append(allFocusableBoxes, textView)

	// Generate initial content
	renderServiceGroup(textView, group)

	return textView
}

// renderServiceGroup renders a service group to a text view
func renderServiceGroup(view *tview.TextView, group *homepage.ServiceGroup) {
	// Clear current content
	view.Clear()

	// Add each service
	for _, service := range group.Services {
		renderService(view, service)
	}
}

// createBookmarksPanel creates a panel with bookmark groups
func createBookmarksPanel(bookmarkGroups []*homepage.BookmarkGroup) tview.Primitive {
	// Create a flex layout for bookmarks
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Add title
	title := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[::b]Bookmarks[-]")
	title.SetBorder(true)
	flex.AddItem(title, 3, 1, false)

	// Add each bookmark group
	for _, group := range bookmarkGroups {
		// Create a box for the group
		groupView := createBookmarkGroupBox(group)
		flex.AddItem(groupView, 0, 1, false)
	}

	return flex
}

// createBookmarkGroupBox creates a box for a single bookmark group
func createBookmarkGroupBox(group *homepage.BookmarkGroup) *tview.TextView {
	// Create text view with scrolling
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true).
		SetScrollable(true)

	// Set border with title
	textView.SetBorder(true).
		SetTitle(group.Name).
		SetTitleColor(tcell.ColorBlue)

	// Make it focusable and add keyboard handler for scrolling
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			// Let main handler handle this
			return event
		}
		// Let arrow keys be handled by main handler
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown ||
			event.Key() == tcell.KeyLeft || event.Key() == tcell.KeyRight {
			return event
		}
		return event
	})

	// Add mouse capture for double-click
	textView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			currentTime := time.Now().UnixNano() / int64(time.Millisecond)

			// Check if this is a double click on the same box
			if textView == lastClickedBox && currentTime-lastClickTime < doubleClickDelay {
				// Double click detected, toggle maximize
				toggleMaximize()
				lastClickTime = 0 // Reset to prevent triple-click detection
			} else {
				// First click, record time and box
				lastClickTime = currentTime
				lastClickedBox = textView

				// Focus the clicked box
				currentFocus = textView
				app.SetFocus(currentFocus)
			}
		}
		return action, event
	})

	// Set custom draw function to draw scrollbar
	textView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		// Call original drawing first
		left, top, innerWidth, innerHeight := textView.Box.GetInnerRect()

		// Draw scrollbar if needed
		rows, _ := textView.GetScrollOffset()
		totalRows := len(strings.Split(textView.GetText(false), "\n"))

		if totalRows > innerHeight {
			// Calculate scrollbar position and size
			scrollHeight := innerHeight - 2 // Adjust for arrows
			scrollPosition := int(float64(rows) / float64(totalRows) * float64(scrollHeight))
			scrollSize := int(float64(innerHeight) / float64(totalRows) * float64(scrollHeight))
			if scrollSize < 1 {
				scrollSize = 1
			}

			// Draw up arrow at top
			screen.SetContent(left+innerWidth-1, top, '▲', nil, tcell.StyleDefault.Foreground(tcell.ColorGray))

			// Draw scrollbar track and thumb
			for i := 0; i < scrollHeight; i++ {
				if i >= scrollPosition && i < scrollPosition+scrollSize {
					screen.SetContent(left+innerWidth-1, top+i+1, '█', nil, tcell.StyleDefault.Foreground(tcell.ColorWhite))
				} else {
					screen.SetContent(left+innerWidth-1, top+i+1, '│', nil, tcell.StyleDefault.Foreground(tcell.ColorGray))
				}
			}

			// Draw down arrow at bottom
			screen.SetContent(left+innerWidth-1, top+innerHeight-1, '▼', nil, tcell.StyleDefault.Foreground(tcell.ColorGray))
		}

		// Return original inner rectangle
		return left, top, innerWidth, innerHeight
	})

	// Add to focusable boxes
	allFocusableBoxes = append(allFocusableBoxes, textView)

	// Generate content
	for _, bookmark := range group.Bookmarks {
		renderBookmark(textView, bookmark)
	}

	return textView
}

// renderService displays a single service with its status
func renderService(view *tview.TextView, service *homepage.Service) {
	// Name and link
	if service.Href != "" {
		fmt.Fprintf(view, "[white::b]%s[::] [#2db7f5](%s)[-]\n", service.Name, service.Href)
	} else {
		fmt.Fprintf(view, "[white::b]%s[-]\n", service.Name)
	}

	// Description if available
	if service.Description != "" {
		fmt.Fprintf(view, "  [#888888]%s[-]\n", service.Description)
	}

	// Status if not disabled
	if !service.DisableStatus {
		monitor := homepage.GetStatusMonitor()
		if monitor != nil {
			result := monitor.GetStatus(service.Name)
			status := result.State
			message := result.Message

			if status == homepage.StatusUnknown {
				message = "Status unknown"
			}

			// Format status based on state
			var statusColor string
			var statusIcon string

			switch status {
			case homepage.StatusOK:
				statusColor = "green"
				statusIcon = "✓"
			case homepage.StatusWarning:
				statusColor = "yellow"
				statusIcon = "!"
			case homepage.StatusCritical:
				statusColor = "red"
				statusIcon = "✗"
			default:
				statusColor = "gray"
				statusIcon = "?"
			}

			fmt.Fprintf(view, "  [%s]%s %s[-]\n", statusColor, statusIcon, message)
		}
	}

	// Separator
	fmt.Fprintf(view, "\n")
}

// renderBookmark displays a single bookmark
func renderBookmark(view *tview.TextView, bookmark *homepage.Bookmark) {
	// Get display name
	displayName := bookmark.Name
	if displayName == "" {
		displayName = bookmark.Abbr
	}

	// Name and link
	fmt.Fprintf(view, "[white::bu]%s[::-] [#2db7f5](%s)[-]\n", displayName, bookmark.Href)

	// Description if available
	if bookmark.Description != "" {
		fmt.Fprintf(view, "  [#888888]%s[-]\n", bookmark.Description)
	}

	// Separator
	fmt.Fprintf(view, "\n")
}

// formatLine creates a line with the specified character
func formatLine(char rune, length int) string {
	line := ""
	for i := 0; i < length; i++ {
		line += string(char)
	}
	return line
}

// statusUpdateCallback is called when a service status changes
func statusUpdateCallback(serviceName string, state homepage.StatusState, message string) {
	// Log the update
	logging.Info("Status update for %s: %s - %s", serviceName, state, message)

	// Skip updates if app isn't initialized
	if !appInitialized || app == nil {
		return
	}

	// Queue UI refresh
	app.QueueUpdateDraw(func() {
		// Update service group view
		if textView, ok := serviceViews[findServiceGroupName(serviceName)]; ok {
			group := findServiceGroup(serviceName)
			if group != nil {
				renderServiceGroup(textView, group)
			}
		}
	})
}

// findServiceGroupName finds the name of a group a service belongs to
func findServiceGroupName(serviceName string) string {
	group := findServiceGroup(serviceName)
	if group != nil {
		return group.Name
	}
	return ""
}

// findServiceGroup finds the group a service belongs to
func findServiceGroup(serviceName string) *homepage.ServiceGroup {
	groups := homepage.GetCachedGroups()
	for _, group := range groups {
		for _, service := range group.Services {
			if service.Name == serviceName {
				return group
			}
		}
	}
	return nil
}

// toggleMaximize switches between maximized and normal view for the focused box
func toggleMaximize() {
	if isMaximized {
		// Restore original layout
		app.SetRoot(originalLayout, true)
		app.SetFocus(currentFocus)
		isMaximized = false
	} else if currentFocus != nil {
		// Save the currently focused box
		maximizedBox = currentFocus

		// Create a new layout with just this box
		maxLayout := tview.NewFlex().SetDirection(tview.FlexRow)

		// Add header
		header := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetText(fmt.Sprintf("[yellow::b]%s - Maximized View[-]", globalSettings.Title))
		header.SetBorder(true)

		// Add footer - smaller
		footer := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetText("[red]Q/Esc: Quit | Space/DoubleClick: Restore[-]")
		footer.SetBorder(false)

		// Add components
		maxLayout.AddItem(header, 3, 1, false)
		maxLayout.AddItem(maximizedBox, 0, 1, true)
		maxLayout.AddItem(footer, 1, 1, false)

		// Set the new layout
		app.SetRoot(maxLayout, true)
		app.SetFocus(maximizedBox)
		isMaximized = true
	}
}

// getBoxPosition returns the row and column position of a box in the grid
func getBoxPosition(box tview.Primitive) (row, col int) {
	for i, b := range allFocusableBoxes {
		if b == box {
			// Assume 2 columns layout (services and bookmarks)
			// This is a simplified approach assuming a grid-like layout
			// Modify this based on actual layout structure
			col = i % 2
			row = i / 2
			return
		}
	}
	return -1, -1
}

// findBoxAt finds the box at the specified row and column
func findBoxAt(row, col int) tview.Primitive {
	// Simple 2-column layout assumption
	// Modify based on actual layout structure
	index := row*2 + col
	if index >= 0 && index < len(allFocusableBoxes) {
		return allFocusableBoxes[index]
	}
	return nil
}

// navigateWithArrows moves focus based on arrow key direction
func navigateWithArrows(key tcell.Key) {
	if len(allFocusableBoxes) == 0 || currentFocus == nil {
		return
	}

	// Get current position
	currentRow, currentCol := getBoxPosition(currentFocus)
	if currentRow < 0 || currentCol < 0 {
		return
	}

	// Calculate new position based on key
	newRow, newCol := currentRow, currentCol
	switch key {
	case tcell.KeyUp:
		newRow--
	case tcell.KeyDown:
		newRow++
	case tcell.KeyLeft:
		newCol--
	case tcell.KeyRight:
		newCol++
	}

	// Find box at new position
	newBox := findBoxAt(newRow, newCol)
	if newBox != nil {
		currentFocus = newBox
		app.SetFocus(currentFocus)
	}
}
