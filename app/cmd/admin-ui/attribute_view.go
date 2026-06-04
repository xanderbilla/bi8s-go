package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type attributeListRow struct {
	IsHeader bool
	Header   string
	Item     AttributeDetail
}

func buildAttributesView(state *AppState) fyne.CanvasObject {
	detailContainer := container.NewMax()
	placeholder := widget.NewLabelWithStyle("Select an Attribute from the list to view metadata properties.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	detailContainer.Objects = []fyne.CanvasObject{placeholder}

	var attrItems []AttributeDetail
	var attrRows []attributeListRow

	buildGroupedRows := func(items []AttributeDetail) []attributeListRow {
		grouped := map[string][]AttributeDetail{}
		for _, item := range items {
			group := "UNSPECIFIED"
			if len(item.AttributeType) > 0 {
				group = strings.ToUpper(item.AttributeType[0])
			}
			grouped[group] = append(grouped[group], item)
		}

		groupKeys := make([]string, 0, len(grouped))
		for key := range grouped {
			groupKeys = append(groupKeys, key)
		}
		sort.Strings(groupKeys)

		rows := make([]attributeListRow, 0, len(items)+len(groupKeys))
		for _, key := range groupKeys {
			rows = append(rows, attributeListRow{IsHeader: true, Header: key})
			groupItems := grouped[key]
			sort.SliceStable(groupItems, func(i, j int) bool {
				left := strings.ToLower(groupItems[i].Name)
				right := strings.ToLower(groupItems[j].Name)
				if left == right {
					return groupItems[i].ID < groupItems[j].ID
				}
				return left < right
			})
			for _, item := range groupItems {
				rows = append(rows, attributeListRow{Item: item})
			}
		}
		return rows
	}

	list := widget.NewList(
		func() int {
			return len(attrRows)
		},
		func() fyne.CanvasObject {
			name := widget.NewLabel("")
			name.TextStyle = fyne.TextStyle{Bold: true}
			types := widget.NewLabel("")
			types.TextStyle = fyne.TextStyle{Italic: true}
			return container.NewVBox(name, types)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			name := box.Objects[0].(*widget.Label)
			types := box.Objects[1].(*widget.Label)

			row := attrRows[id]
			if row.IsHeader {
				name.TextStyle = fyne.TextStyle{Bold: true}
				name.SetText("[" + row.Header + "]")
				types.SetText("")
				return
			}

			item := row.Item
			name.TextStyle = fyne.TextStyle{Bold: false}
			name.SetText(item.Name)
			types.SetText(fmt.Sprintf("%s | ID: %s", strings.Join(item.AttributeType, ", "), item.ID))
		},
	)

	fetchAttrs := func(searchText string, sortMode string, attrType string) {
		searchPath := "/a/search?entity=attributes&limit=300&sort=" + url.QueryEscape(sortMode)
		if strings.TrimSpace(searchText) != "" {
			searchPath += "&q=" + url.QueryEscape(strings.TrimSpace(searchText))
		}
		if strings.TrimSpace(attrType) != "" && attrType != "ALL" {
			searchPath += "&attributeType=" + url.QueryEscape(strings.TrimSpace(attrType))
		}

		data, err := state.APIClient.request("GET", searchPath, nil, nil)
		if err == nil {
			var resp AdminSearchResponse
			if json.Unmarshal(data, &resp) == nil && resp.Data.Attributes != nil {
				attrItems = resp.Data.Attributes.Items
				state.AttributesList = attrItems
				attrRows = buildGroupedRows(attrItems)
				list.Refresh()
				return
			}
		}

		fallbackData, fallbackErr := state.APIClient.request("GET", "/a/attributes/", nil, nil)
		if fallbackErr != nil {
			dialog.ShowError(fmt.Errorf("failed to fetch attributes: %w", fallbackErr), state.Window)
			return
		}

		var fallbackResp AttributeListResponse
		if err := json.Unmarshal(fallbackData, &fallbackResp); err != nil {
			dialog.ShowError(fmt.Errorf("failed to parse response: %w", err), state.Window)
			return
		}

		q := strings.ToLower(strings.TrimSpace(searchText))
		filtered := make([]AttributeDetail, 0, len(fallbackResp.Data))
		for _, item := range fallbackResp.Data {
			if strings.TrimSpace(attrType) != "" && attrType != "ALL" {
				foundType := false
				for _, t := range item.AttributeType {
					if strings.EqualFold(t, attrType) {
						foundType = true
						break
					}
				}
				if !foundType {
					continue
				}
			}
			if q == "" || strings.Contains(strings.ToLower(item.Name+" "+item.Logo+" "+item.SVG+" "+item.ID+" "+strings.Join(item.AttributeType, " ")), q) {
				filtered = append(filtered, item)
			}
		}

		attrItems = filtered
		state.AttributesList = attrItems
		attrRows = buildGroupedRows(attrItems)
		list.Refresh()
	}

	state.RefreshAttrList = func() { fetchAttrs("", "alpha_asc", "ALL") }

	list.OnSelected = func(id widget.ListItemID) {
		row := attrRows[id]
		if row.IsHeader {
			return
		}
		selected := row.Item
		state.SelectedAttr = &selected

		fullData, err := state.APIClient.request("GET", fmt.Sprintf("/a/attributes/%s", selected.ID), nil, nil)
		if err != nil {
			dialog.ShowError(err, state.Window)
			return
		}
		var detailResp AttributeResponse
		if err := json.Unmarshal(fullData, &detailResp); err != nil {
			dialog.ShowError(err, state.Window)
			return
		}
		detail := detailResp.Data
		state.SelectedAttr = &detail

		infoCard := widget.NewCard(detail.Name, fmt.Sprintf("Attribute Properties (ID: %s)", detail.ID), container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Types: %s", strings.Join(detail.AttributeType, ", "))),
			widget.NewLabel(fmt.Sprintf("Active: %v", detail.Active)),
			widget.NewLabel(fmt.Sprintf("Logo: %s", detail.Logo)),
			widget.NewLabel(fmt.Sprintf("SVG Content: %s", detail.SVG)),
		))

		deleteBtn := widget.NewButtonWithIcon("Delete Attribute", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Confirm Deletion", fmt.Sprintf("Are you sure you want to delete attribute '%s'?", detail.Name), func(ok bool) {
				if ok {
					_, err := state.APIClient.request("DELETE", fmt.Sprintf("/a/attributes/%s", detail.ID), nil, nil)
					if err != nil {
						dialog.ShowError(err, state.Window)
					} else {
						dialog.ShowInformation("Deleted", "Attribute deleted successfully", state.Window)
						detailContainer.Objects = []fyne.CanvasObject{placeholder}
						detailContainer.Refresh()
						fetchAttrs("", "alpha_asc", "ALL")
					}
				}
			}, state.Window)
		})

		detailContainer.Objects = []fyne.CanvasObject{container.NewVBox(infoCard, deleteBtn)}
		detailContainer.Refresh()
	}

	showCreateForm := func() {
		nameEntry := widget.NewEntry()
		nameEntry.PlaceHolder = "e.g., Action"

		tagCheck := widget.NewCheck("TAG", nil)
		moodCheck := widget.NewCheck("MOOD", nil)
		genreCheck := widget.NewCheck("GENRE", nil)
		categoryCheck := widget.NewCheck("CATEGORY", nil)
		specialityCheck := widget.NewCheck("SPECIALITY", nil)
		studioCheck := widget.NewCheck("STUDIO", nil)
		socialCheck := widget.NewCheck("SOCIAL", nil)
		platformCheck := widget.NewCheck("PLATFORM", nil)

		typesContainer := container.NewGridWithColumns(3,
			tagCheck, moodCheck, genreCheck,
			categoryCheck, specialityCheck, studioCheck,
			socialCheck, platformCheck,
		)

		activeCheck := widget.NewCheck("Is Active Attribute", nil)
		activeCheck.Checked = true

		logoEntry := widget.NewEntry()
		logoEntry.PlaceHolder = "Image logo URL (Optional for SOCIAL/PLATFORM)"

		svgEntry := widget.NewMultiLineEntry()
		svgEntry.PlaceHolder = "<svg>...</svg> raw markup content (Optional for SOCIAL/PLATFORM)"

		logoItem := widget.NewFormItem("Logo URL", logoEntry)
		svgItem := widget.NewFormItem("SVG Markup", svgEntry)

		updateBrandingFieldVisibility := func() {
			show := socialCheck.Checked || platformCheck.Checked
			if show {
				logoEntry.Enable()
				svgEntry.Enable()
				logoEntry.Show()
				svgEntry.Show()
				return
			}
			logoEntry.SetText("")
			svgEntry.SetText("")
			logoEntry.Disable()
			svgEntry.Disable()
			logoEntry.Hide()
			svgEntry.Hide()
		}

		socialCheck.OnChanged = func(_ bool) { updateBrandingFieldVisibility() }
		platformCheck.OnChanged = func(_ bool) { updateBrandingFieldVisibility() }
		updateBrandingFieldVisibility()

		form := widget.NewForm(
			NewRequiredFormItem("Name", nameEntry),
			NewRequiredFormItem("Attribute Types (Select all that apply)", typesContainer),
			widget.NewFormItem("Active", activeCheck),
			logoItem,
			svgItem,
		)

		submitBtn := widget.NewButtonWithIcon("Save Attribute", theme.ConfirmIcon(), func() {
			if nameEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("name is a required field"), state.Window)
				return
			}

			cleanedTypes := []string{}
			if tagCheck.Checked {
				cleanedTypes = append(cleanedTypes, "TAG")
			}
			if moodCheck.Checked {
				cleanedTypes = append(cleanedTypes, "MOOD")
			}
			if genreCheck.Checked {
				cleanedTypes = append(cleanedTypes, "GENRE")
			}
			if categoryCheck.Checked {
				cleanedTypes = append(cleanedTypes, "CATEGORY")
			}
			if specialityCheck.Checked {
				cleanedTypes = append(cleanedTypes, "SPECIALITY")
			}
			if studioCheck.Checked {
				cleanedTypes = append(cleanedTypes, "STUDIO")
			}
			if socialCheck.Checked {
				cleanedTypes = append(cleanedTypes, "SOCIAL")
			}
			if platformCheck.Checked {
				cleanedTypes = append(cleanedTypes, "PLATFORM")
			}

			if len(cleanedTypes) == 0 {
				dialog.ShowError(fmt.Errorf("at least one attribute type must be selected"), state.Window)
				return
			}

			allowBranding := socialCheck.Checked || platformCheck.Checked
			fields := map[string]string{
				"name":           nameEntry.Text,
				"attribute_type": strings.Join(cleanedTypes, ","),
				"active":         strconv.FormatBool(activeCheck.Checked),
				"logo":           "",
				"svg":            "",
			}
			if allowBranding {
				fields["logo"] = logoEntry.Text
				fields["svg"] = svgEntry.Text
			}

			_, err := state.APIClient.multipartRequest("POST", "/a/attributes/", fields, nil)
			if err != nil {
				dialog.ShowError(err, state.Window)
			} else {
				dialog.ShowInformation("Success", "Attribute successfully created!", state.Window)
				detailContainer.Objects = []fyne.CanvasObject{placeholder}
				detailContainer.Refresh()
				fetchAttrs("", "alpha_asc", "ALL")
			}
		})

		cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
			detailContainer.Objects = []fyne.CanvasObject{placeholder}
			detailContainer.Refresh()
		})

		formCard := widget.NewCard("Create Attribute", "SOCIAL/PLATFORM attributes can include Logo and SVG branding.", container.NewVBox(
			form,
			container.NewHBox(submitBtn, cancelBtn),
		))

		detailContainer.Objects = []fyne.CanvasObject{container.NewScroll(formCard)}
		detailContainer.Refresh()
	}

	createBtn := widget.NewButtonWithIcon("New Attribute", theme.ContentAddIcon(), func() {
		showCreateForm()
	})

	searchEntry := widget.NewEntry()
	searchEntry.PlaceHolder = "Search attributes by name, type, logo, svg, id..."
	sortSelect := widget.NewSelect([]string{"alpha_asc", "alpha_desc", "recent", "latest"}, nil)
	sortSelect.SetSelected("alpha_asc")
	typeFilterSelect := widget.NewSelect([]string{"ALL", "GENRE", "TAG", "MOOD", "STUDIO", "CATEGORY", "SPECIALITY", "SOCIAL", "PLATFORM"}, nil)
	typeFilterSelect.SetSelected("ALL")

	applySearch := func() {
		fetchAttrs(searchEntry.Text, sortSelect.Selected, typeFilterSelect.Selected)
	}
	searchEntry.OnChanged = func(_ string) {
		applySearch()
	}
	sortSelect.OnChanged = func(_ string) {
		applySearch()
	}
	typeFilterSelect.OnChanged = func(_ string) {
		applySearch()
	}

	fetchAttrs("", "alpha_asc", "ALL")

	leftPane := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Attributes Library", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			searchEntry,
			sortSelect,
			typeFilterSelect,
		),
		createBtn, nil, nil,
		list,
	)

	split := container.NewHSplit(leftPane, detailContainer)
	split.Offset = 0.35
	return split
}
