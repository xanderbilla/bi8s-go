package main

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// --- STATE MANAGEMENT ---

type AppState struct {
	APIClient       *Client
	Window          fyne.Window
	ContentList     []ContentDetail
	PeopleList      []PersonDetail
	AttributesList  []AttributeDetail
	SelectedContent *ContentDetail
	SelectedPerson  *PersonDetail
	SelectedAttr    *AttributeDetail

	// UI Refresh Callbacks
	RefreshContentList func()
	RefreshPeopleList  func()
	RefreshAttrList    func()
}

// --- MAIN APPLICATION ENTRY ---

func main() {
	a := app.NewWithID("dev.emm4bi8s.admin-ui")
	w := a.NewWindow("Bi8s API Admin Dashboard")
	w.Resize(fyne.NewSize(1150, 720))

	// Global State
	state := &AppState{
		APIClient: NewClient("http://localhost:8180/v1"),
		Window:    w,
	}

	// Main Layout Navigation
	navItems := []string{
		"Content Management",
		"People Management",
		"Attributes Management",
		"System Operations",
		"API Configuration",
	}

	contentArea := container.NewMax()

	// Screen Builder maps
	screens := map[string]fyne.CanvasObject{
		"Content Management":    buildContentView(state),
		"People Management":     buildPeopleView(state),
		"Attributes Management": buildAttributesView(state),
		"System Operations":     buildSystemView(state),
		"API Configuration":     buildSettingsView(state, contentArea, navItems),
	}

	// Navigation side panel list
	navList := widget.NewList(
		func() int {
			return len(navItems)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.TextStyle = fyne.TextStyle{Bold: true}
			return container.NewBorder(nil, nil, widget.NewIcon(theme.DocumentIcon()), nil, label)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			border := obj.(*fyne.Container)
			label := border.Objects[0].(*widget.Label)
			label.SetText(navItems[id])

			iconObj := border.Objects[1].(*widget.Icon)
			switch navItems[id] {
			case "Content Management":
				iconObj.SetResource(theme.StorageIcon())
			case "People Management":
				iconObj.SetResource(theme.HomeIcon())
			case "Attributes Management":
				iconObj.SetResource(theme.GridIcon())
			case "System Operations":
				iconObj.SetResource(theme.ViewRefreshIcon())
			case "API Configuration":
				iconObj.SetResource(theme.SettingsIcon())
			}
		},
	)

	navList.OnSelected = func(id widget.ListItemID) {
		selectedScreen := navItems[id]
		contentArea.Objects = []fyne.CanvasObject{screens[selectedScreen]}
		contentArea.Refresh()
	}

	// Set initial navigation selection
	navList.Select(0)

	// Header Layout
	headerTitle := widget.NewLabel("BI8S ADMINISTRATION CMS")
	headerTitle.TextStyle = fyne.TextStyle{Bold: true, Italic: true}
	headerStatus := widget.NewLabel("API Base: http://localhost:8180/v1")
	headerStatus.TextStyle = fyne.TextStyle{Monospace: true}
	header := container.NewBorder(nil, nil, headerTitle, headerStatus, widget.NewSeparator())

	// Assemble layout
	sidebar := container.NewBorder(
		widget.NewLabelWithStyle(" NAVIGATION", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		navList,
	)

	mainSplit := container.NewHSplit(sidebar, contentArea)
	mainSplit.Offset = 0.22

	rootContainer := container.NewBorder(header, nil, nil, nil, mainSplit)

	w.SetContent(rootContainer)
	w.ShowAndRun()
}

// Settings View for API configuration
func buildSettingsView(state *AppState, contentArea *fyne.Container, navItems []string) fyne.CanvasObject {
	urlEntry := widget.NewEntry()
	urlEntry.SetText(state.APIClient.BaseURL)
	urlEntry.PlaceHolder = "e.g., http://localhost:8180/v1"

	statusLabel := widget.NewLabel("Status: Connection Untested")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	testBtn := widget.NewButtonWithIcon("Test Connection", theme.CheckButtonIcon(), func() {
		c := NewClient(urlEntry.Text)
		data, err := c.request("GET", "/health", nil, nil)
		if err != nil {
			statusLabel.SetText(fmt.Sprintf("Status: Error (%s)", err.Error()))
			dialog.ShowError(err, state.Window)
			return
		}

		var health SuccessEnvelope
		if err := json.Unmarshal(data, &health); err != nil {
			statusLabel.SetText("Status: Error (Invalid Response)")
			dialog.ShowError(fmt.Errorf("json parse error: %w", err), state.Window)
			return
		}

		statusLabel.SetText("Status: Connected (200 OK)")
		state.APIClient = c
		dialog.ShowInformation("Connection Success", "Successfully connected to Bi8s backend API!", state.Window)
	})

	saveBtn := widget.NewButtonWithIcon("Apply Configuration", theme.ConfirmIcon(), func() {
		state.APIClient = NewClient(urlEntry.Text)
		dialog.ShowInformation("Config Saved", "API Client base URL updated.", state.Window)
	})

	card := widget.NewCard("API Connection Settings", "Manage target service end-point configurations.", container.NewVBox(
		widget.NewLabel("Backend Base URL:"),
		urlEntry,
		container.NewHBox(testBtn, saveBtn),
		statusLabel,
	))

	return container.NewPadded(card)
}
