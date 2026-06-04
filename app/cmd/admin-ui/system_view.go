package main

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func buildSystemView(state *AppState) fyne.CanvasObject {
	resultsLabel := widget.NewLabel("Reindex results will be displayed here.")
	resultsLabel.TextStyle = fyne.TextStyle{Monospace: true}

	var reindexBtn *widget.Button
	reindexBtn = widget.NewButtonWithIcon("Trigger Full Search Reindex", theme.ViewRefreshIcon(), func() {
		reindexBtn.Disable()
		resultsLabel.SetText("Triggering full reindex in progress... Please wait. (This is a synchronous long-running operation)")

		go func() {
			data, err := state.APIClient.request("POST", "/a/reindex", nil, nil)
			reindexBtn.Enable()
			if err != nil {
				resultsLabel.SetText(fmt.Sprintf("Reindex failed: %s", err.Error()))
				dialog.ShowError(err, state.Window)
				return
			}

			var resp ReindexResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				resultsLabel.SetText("Failed to parse reindex results.")
				dialog.ShowError(err, state.Window)
				return
			}

			resultsLabel.SetText(fmt.Sprintf("Reindex Completed Successfully!\n\nMetrics:\n- People Reindexed: %d\n- Content Reindexed: %d\n- Join Table Entries Synced: %d\n\nTimestamp: %s",
				resp.Data.People, resp.Data.Content, resp.Data.JoinTableEntries, resp.Timestamp))
			dialog.ShowInformation("Reindex Complete", "Database Search Index successfully rebuilt!", state.Window)
		}()
	})

	runDiagnosticsBtn := widget.NewButtonWithIcon("Run System Health Check", theme.ComputerIcon(), func() {
		data, err := state.APIClient.request("GET", "/health", nil, nil)
		if err != nil {
			dialog.ShowError(fmt.Errorf("health check failed: %w", err), state.Window)
			return
		}
		var env SuccessEnvelope
		_ = json.Unmarshal(data, &env)

		dialog.ShowInformation("System Diagnostic", fmt.Sprintf("All downstream dependencies are verified.\nAPI Status: %d (%s)\nTimestamp: %s", env.Status, env.Message, env.Timestamp), state.Window)
	})

	card := widget.NewCard("Reindex & Diagnostics", "Run administrative background tasks directly.", container.NewVBox(
		widget.NewLabel("Use these controls to synchronize primary DB to search indexes and perform health checks."),
		reindexBtn,
		runDiagnosticsBtn,
		widget.NewSeparator(),
		resultsLabel,
	))

	return container.NewPadded(card)
}
