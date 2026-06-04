package main

import (
	"fmt"
	"io"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// MultiSelectChipSelector provides a searchable, async select dropdown with chips/badges below it.
type MultiSelectChipSelector struct {
	Container fyne.CanvasObject
	Selected  []EntityRef
	OnChange  func([]EntityRef)
}

func NewMultiSelectChipSelector(
	placeholder string,
	searchFunc func(string) []EntityRef,
	onChange func([]EntityRef),
) *MultiSelectChipSelector {
	selector := &MultiSelectChipSelector{
		Selected: []EntityRef{},
		OnChange: onChange,
	}

	chipsContainer := container.NewHBox()

	var refreshChips func()
	refreshChips = func() {
		chipsContainer.Objects = nil
		for _, item := range selector.Selected {
			itm := item // capture loop variable
			chipLabel := widget.NewLabel(itm.Name)
			chipLabel.TextStyle = fyne.TextStyle{Bold: true}

			removeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
				newSelected := []EntityRef{}
				for _, s := range selector.Selected {
					if s.ID != itm.ID {
						newSelected = append(newSelected, s)
					}
				}
				selector.Selected = newSelected
				refreshChips()
				if selector.OnChange != nil {
					selector.OnChange(selector.Selected)
				}
			})
			removeBtn.Importance = widget.LowImportance

			chip := container.NewBorder(nil, nil, nil, removeBtn, chipLabel)
			chipsContainer.Add(chip)
		}
		chipsContainer.Refresh()
	}

	entry := widget.NewEntry()
	entry.PlaceHolder = placeholder

	resultItems := []EntityRef{}
	resultsList := widget.NewList(
		func() int { return len(resultItems) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			return label
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(fmt.Sprintf("%s (%s)", resultItems[id].Name, resultItems[id].ID))
		},
	)
	resultsList.Resize(fyne.NewSize(100, 140))
	resultsWrap := container.NewMax(resultsList)
	resultsWrap.Hide()

	addSelection := func(ref EntityRef) {
		alreadySelected := false
		for _, s := range selector.Selected {
			if s.ID == ref.ID {
				alreadySelected = true
				break
			}
		}
		if alreadySelected {
			return
		}
		selector.Selected = append(selector.Selected, ref)
		refreshChips()
		if selector.OnChange != nil {
			selector.OnChange(selector.Selected)
		}
	}

	resultsList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(resultItems) {
			return
		}
		addSelection(resultItems[id])
		entry.SetText("")
		resultItems = []EntityRef{}
		resultsList.Refresh()
		resultsWrap.Hide()
	}

	entry.OnChanged = func(text string) {
		query := strings.TrimSpace(text)
		if query == "" {
			resultItems = []EntityRef{}
			resultsList.Refresh()
			resultsWrap.Hide()
			return
		}

		matches := searchFunc(query)
		filtered := make([]EntityRef, 0, len(matches))
		for _, m := range matches {
			exists := false
			for _, s := range selector.Selected {
				if s.ID == m.ID {
					exists = true
					break
				}
			}
			if !exists {
				filtered = append(filtered, m)
			}
		}
		resultItems = filtered
		resultsList.Refresh()
		if len(resultItems) == 0 {
			resultsWrap.Hide()
			return
		}
		resultsWrap.Show()
	}

	selector.Container = container.NewVBox(entry, resultsWrap, chipsContainer)
	return selector
}

// Clear resets the selected list in the chip selector
func (m *MultiSelectChipSelector) Clear() {
	m.Selected = nil
	if box, ok := m.Container.(*fyne.Container); ok {
		if chips, ok := box.Objects[2].(*fyne.Container); ok {
			chips.Objects = nil
			chips.Refresh()
		}
	}
}

// SetSelected initializes the chip selector with items
func (m *MultiSelectChipSelector) SetSelected(items []EntityRef) {
	m.Selected = items
	if box, ok := m.Container.(*fyne.Container); ok {
		if chipsContainer, ok := box.Objects[2].(*fyne.Container); ok {
			chipsContainer.Objects = nil
			for _, item := range m.Selected {
				itm := item
				chipLabel := widget.NewLabel(itm.Name)
				chipLabel.TextStyle = fyne.TextStyle{Bold: true}

				removeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
					newSelected := []EntityRef{}
					for _, s := range m.Selected {
						if s.ID != itm.ID {
							newSelected = append(newSelected, s)
						}
					}
					m.SetSelected(newSelected)
					if m.OnChange != nil {
						m.OnChange(m.Selected)
					}
				})
				removeBtn.Importance = widget.LowImportance

				chip := container.NewBorder(nil, nil, nil, removeBtn, chipLabel)
				chipsContainer.Add(chip)
			}
			chipsContainer.Refresh()
		}
	}
}

// Helper to create required form fields with asterisks
func NewRequiredFormItem(label string, field fyne.CanvasObject) *widget.FormItem {
	return widget.NewFormItem(label+" *", field)
}

// showFilePickerButton creates a button that opens a file dialog.
func showFilePickerButton(
	w fyne.Window,
	title string,
	label *widget.Label,
	onSelected func(io.ReadCloser, string),
) fyne.CanvasObject {
	btn := widget.NewButtonWithIcon(title, theme.FileIcon(), func() {
		dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if rc == nil {
				// User cancelled
				return
			}
			label.SetText(rc.URI().Name())
			onSelected(rc, rc.URI().Name())
		}, w)
	})
	return btn
}
