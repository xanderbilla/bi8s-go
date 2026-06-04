package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func buildContentView(state *AppState) fyne.CanvasObject {
	detailContainer := container.NewMax()
	placeholder := widget.NewLabelWithStyle("Select a Content Item from the list to view details or manage assets.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	detailContainer.Objects = []fyne.CanvasObject{placeholder}

	var contentItems []ContentDetail

	// Local cache of attributes for the searchable dropdowns
	var cachedAttributes []AttributeDetail
	var cachedPeople []PersonDetail

	fetchCaches := func() {
		// Fetch attributes
		attrData, err := state.APIClient.request("GET", "/a/attributes/", nil, nil)
		if err == nil {
			var resp AttributeListResponse
			if json.Unmarshal(attrData, &resp) == nil {
				cachedAttributes = resp.Data
			}
		}

		// Fetch people
		peopleData, err := state.APIClient.request("GET", "/a/people/?limit=100", nil, nil)
		if err == nil {
			var resp PeopleListResponse
			if json.Unmarshal(peopleData, &resp) == nil {
				cachedPeople = resp.Data.Items
			}
		}
	}

	list := widget.NewList(
		func() int {
			return len(contentItems)
		},
		func() fyne.CanvasObject {
			title := widget.NewLabel("")
			title.TextStyle = fyne.TextStyle{Bold: true}
			meta := widget.NewLabel("")
			meta.TextStyle = fyne.TextStyle{Italic: true}
			return container.NewVBox(title, meta)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			meta := box.Objects[1].(*widget.Label)

			item := contentItems[id]
			title.SetText(item.Title)
			meta.SetText(fmt.Sprintf("%s | ID: %s", item.ContentType, item.ID))
		},
	)

	fetchContent := func(searchText string, sortMode string) {
		fetchCaches()

		searchPath := "/a/search?entity=content&limit=100&sort=" + url.QueryEscape(sortMode)
		if strings.TrimSpace(searchText) != "" {
			searchPath += "&q=" + url.QueryEscape(strings.TrimSpace(searchText))
		}

		data, err := state.APIClient.request("GET", searchPath, nil, nil)
		if err == nil {
			var resp AdminSearchResponse
			if json.Unmarshal(data, &resp) == nil && resp.Data.Content != nil {
				contentItems = resp.Data.Content.Items
				state.ContentList = contentItems
				list.Refresh()
				return
			}
		}

		fallbackData, fallbackErr := state.APIClient.request("GET", "/a/content/?limit=100", nil, nil)
		if fallbackErr != nil {
			dialog.ShowError(fmt.Errorf("failed to fetch content: %w", fallbackErr), state.Window)
			return
		}

		var fallbackResp ContentListResponse
		if err := json.Unmarshal(fallbackData, &fallbackResp); err != nil {
			dialog.ShowError(fmt.Errorf("failed to parse response: %w", err), state.Window)
			return
		}

		filtered := make([]ContentDetail, 0, len(fallbackResp.Data.Items))
		for _, item := range fallbackResp.Data.Items {
			if item.ContentType == "MOVIE" || item.ContentType == "TV" {
				if strings.TrimSpace(searchText) == "" || strings.Contains(strings.ToLower(item.Title+" "+item.Overview+" "+item.Tagline), strings.ToLower(strings.TrimSpace(searchText))) {
					filtered = append(filtered, item)
				}
			}
		}
		contentItems = filtered
		state.ContentList = contentItems
		list.Refresh()
	}

	state.RefreshContentList = func() { fetchContent("", "recent") }

	list.OnSelected = func(id widget.ListItemID) {
		selected := contentItems[id]
		state.SelectedContent = &selected

		// Load Full details
		fullData, err := state.APIClient.request("GET", fmt.Sprintf("/a/content/%s", selected.ID), nil, nil)
		if err != nil {
			dialog.ShowError(err, state.Window)
			return
		}
		var detailResp ContentResponse
		if err := json.Unmarshal(fullData, &detailResp); err != nil {
			dialog.ShowError(err, state.Window)
			return
		}

		detail := detailResp.Data
		state.SelectedContent = &detail

		// Show details card
		infoCard := widget.NewCard(detail.Title, fmt.Sprintf("%s Details (ID: %s)", detail.ContentType, detail.ID), container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Overview: %s", detail.Overview)),
			widget.NewLabel(fmt.Sprintf("Original Language: %s", detail.OriginalLanguage)),
			widget.NewLabel(fmt.Sprintf("Effective Date: %s", detail.EffectiveReleaseDate())),
			widget.NewLabel(fmt.Sprintf("Content Rating: %s", detail.ContentRating)),
			widget.NewLabel(fmt.Sprintf("Runtime: %d min", detail.Runtime)),
			widget.NewLabel(fmt.Sprintf("Status: %s", detail.Status)),
			widget.NewLabel(fmt.Sprintf("Visibility: %s", detail.Visibility)),
			widget.NewLabel(fmt.Sprintf("Adult: %v", detail.Adult)),
			widget.NewLabel(fmt.Sprintf("Poster Path: %s", detail.PosterPath)),
			widget.NewLabel(fmt.Sprintf("Backdrop Path: %s", detail.BackdropPath)),
		))

		// Relationships card
		genresStr := []string{}
		for _, g := range detail.Genres {
			genresStr = append(genresStr, g.Name)
		}
		castsStr := []string{}
		for _, c := range detail.Casts {
			castsStr = append(castsStr, c.Name)
		}
		tagsStr := []string{}
		for _, t := range detail.Tags {
			tagsStr = append(tagsStr, t.Name)
		}
		moodsStr := []string{}
		for _, m := range detail.MoodTags {
			moodsStr = append(moodsStr, m.Name)
		}
		studiosStr := []string{}
		for _, s := range detail.Studios {
			studiosStr = append(studiosStr, s.Name)
		}

		relCard := widget.NewCard("Relations", "Attributes & Cast mapping", container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Genres: %s", strings.Join(genresStr, ", "))),
			widget.NewLabel(fmt.Sprintf("Cast Members: %s", strings.Join(castsStr, ", "))),
			widget.NewLabel(fmt.Sprintf("Tags: %s", strings.Join(tagsStr, ", "))),
			widget.NewLabel(fmt.Sprintf("Mood Tags: %s", strings.Join(moodsStr, ", "))),
			widget.NewLabel(fmt.Sprintf("Studios: %s", strings.Join(studiosStr, ", "))),
		))

		titleEdit := widget.NewEntry()
		titleEdit.SetText(detail.Title)
		overviewEdit := widget.NewMultiLineEntry()
		overviewEdit.SetText(detail.Overview)
		taglineEdit := widget.NewEntry()
		taglineEdit.SetText(detail.Tagline)
		runtimeEdit := widget.NewEntry()
		runtimeEdit.SetText(strconv.Itoa(detail.Runtime))
		dateEdit := widget.NewEntry()
		dateEdit.SetText(detail.EffectiveReleaseDate())
		adultEdit := widget.NewCheck("Adult", nil)
		adultEdit.SetChecked(detail.Adult)

		ratingEdit := widget.NewSelect([]string{"", "18_PLUS", "21_PLUS"}, nil)
		ratingEdit.SetSelected(detail.ContentRating)
		statusEdit := widget.NewSelect([]string{"RUMORED", "PLANNED", "IN_PRODUCTION", "POST_PRODUCTION", "RELEASED", "ENDED", "RETURNING_SERIES", "CANCELED", "PILOT"}, nil)
		statusEdit.SetSelected(detail.Status)
		visibilityEdit := widget.NewSelect([]string{"PUBLIC", "PRIVATE"}, nil)
		visibilityEdit.SetSelected(detail.Visibility)
		originCountryEdit := widget.NewEntry()
		originCountryEdit.SetText(strings.Join(detail.OriginCountry, ","))

		coreUpdateBtn := widget.NewButtonWithIcon("Update Core Fields", theme.DocumentSaveIcon(), func() {
			runtime, err := strconv.Atoi(strings.TrimSpace(runtimeEdit.Text))
			if err != nil {
				dialog.ShowError(fmt.Errorf("runtime must be a number"), state.Window)
				return
			}
			payload := map[string]any{
				"title":            strings.TrimSpace(titleEdit.Text),
				"overview":         strings.TrimSpace(overviewEdit.Text),
				"adult":            adultEdit.Checked,
				"contentRating":    strings.TrimSpace(ratingEdit.Selected),
				"originalLanguage": detail.OriginalLanguage,
				"originCountry":    splitCSV(originCountryEdit.Text),
				"runtime":          runtime,
				"status":           strings.TrimSpace(statusEdit.Selected),
				"tagline":          strings.TrimSpace(taglineEdit.Text),
				"visibility":       strings.TrimSpace(visibilityEdit.Selected),
				"assets":           detail.Assets,
			}
			if detail.ContentType == "TV" {
				payload["firstAirDate"] = strings.TrimSpace(dateEdit.Text)
			} else {
				payload["releaseDate"] = strings.TrimSpace(dateEdit.Text)
			}

			if _, err := state.APIClient.updateContentCore(detail.ID, payload); err != nil {
				dialog.ShowError(err, state.Window)
				return
			}
			dialog.ShowInformation("Updated", "Content core fields updated", state.Window)
			list.Select(id)
		})

		coreUpdateCard := widget.NewCard("Update Content", "Updates only allowed core fields", container.NewVBox(
			widget.NewForm(
				widget.NewFormItem("Title", titleEdit),
				widget.NewFormItem("Overview", overviewEdit),
				widget.NewFormItem("Tagline", taglineEdit),
				widget.NewFormItem("Release/Air Date", dateEdit),
				widget.NewFormItem("Runtime", runtimeEdit),
				widget.NewFormItem("Content Rating", ratingEdit),
				widget.NewFormItem("Status", statusEdit),
				widget.NewFormItem("Visibility", visibilityEdit),
				widget.NewFormItem("Origin Countries", originCountryEdit),
				widget.NewFormItem("Adult", adultEdit),
			),
			coreUpdateBtn,
		))

		posterLabel := widget.NewLabel("No file selected")
		var posterReader io.ReadCloser
		var posterName string
		posterPicker := showFilePickerButton(state.Window, "Select Poster", posterLabel, func(r io.ReadCloser, name string) {
			posterReader = r
			posterName = name
		})
		posterUpdateBtn := widget.NewButtonWithIcon("Update Poster", theme.UploadIcon(), func() {
			if posterReader == nil {
				dialog.ShowError(fmt.Errorf("select a poster file first"), state.Window)
				return
			}
			_, err := state.APIClient.multipartRequest("PUT", fmt.Sprintf("/a/content/poster/%s", detail.ID), nil, []FormFile{{FieldName: "poster", FileName: posterName, Reader: posterReader}})
			if err != nil {
				dialog.ShowError(err, state.Window)
				return
			}
			dialog.ShowInformation("Updated", "Poster image updated", state.Window)
			list.Select(id)
		})

		backdropLabel := widget.NewLabel("No file selected")
		var backdropReader io.ReadCloser
		var backdropName string
		backdropPicker := showFilePickerButton(state.Window, "Select Backdrop", backdropLabel, func(r io.ReadCloser, name string) {
			backdropReader = r
			backdropName = name
		})
		backdropUpdateBtn := widget.NewButtonWithIcon("Update Backdrop", theme.UploadIcon(), func() {
			if backdropReader == nil {
				dialog.ShowError(fmt.Errorf("select a backdrop file first"), state.Window)
				return
			}
			_, err := state.APIClient.multipartRequest("PUT", fmt.Sprintf("/a/content/backdrop/%s", detail.ID), nil, []FormFile{{FieldName: "backdrop", FileName: backdropName, Reader: backdropReader}})
			if err != nil {
				dialog.ShowError(err, state.Window)
				return
			}
			dialog.ShowInformation("Updated", "Backdrop image updated", state.Window)
			list.Select(id)
		})

		imageUpdateCard := widget.NewCard("Update Images", "Poster and backdrop are updated via dedicated endpoints", container.NewVBox(
			container.NewHBox(posterPicker, posterLabel),
			posterUpdateBtn,
			widget.NewSeparator(),
			container.NewHBox(backdropPicker, backdropLabel),
			backdropUpdateBtn,
		))

		attributeOptions := make([]string, 0, len(cachedAttributes))
		attributeMap := map[string]string{}
		for _, a := range cachedAttributes {
			label := fmt.Sprintf("%s (%s)", a.Name, a.ID)
			attributeOptions = append(attributeOptions, label)
			attributeMap[label] = a.ID
		}
		attributeSelect := widget.NewSelect(attributeOptions, nil)

		castOptions := make([]string, 0, len(cachedPeople))
		castMap := map[string]string{}
		for _, p := range cachedPeople {
			label := fmt.Sprintf("%s (%s)", p.Name, p.ID)
			castOptions = append(castOptions, label)
			castMap[label] = p.ID
		}
		castSelect := widget.NewSelect(castOptions, nil)

		relationCard := widget.NewCard("Add / Remove Relations", "Attribute and cast mutation endpoints", container.NewVBox(
			widget.NewForm(
				widget.NewFormItem("Attribute", attributeSelect),
				widget.NewFormItem("Cast Person", castSelect),
			),
			container.NewHBox(
				widget.NewButton("Add Attribute", func() {
					attrID := attributeMap[attributeSelect.Selected]
					if attrID == "" {
						dialog.ShowError(fmt.Errorf("select an attribute first"), state.Window)
						return
					}
					if _, err := state.APIClient.mutateContentRelation(attrID, detail.ID, true); err != nil {
						dialog.ShowError(err, state.Window)
						return
					}
					dialog.ShowInformation("Updated", "Attribute added", state.Window)
					list.Select(id)
				}),
				widget.NewButton("Remove Attribute", func() {
					attrID := attributeMap[attributeSelect.Selected]
					if attrID == "" {
						dialog.ShowError(fmt.Errorf("select an attribute first"), state.Window)
						return
					}
					if _, err := state.APIClient.mutateContentRelation(attrID, detail.ID, false); err != nil {
						dialog.ShowError(err, state.Window)
						return
					}
					dialog.ShowInformation("Updated", "Attribute removed", state.Window)
					list.Select(id)
				}),
			),
			container.NewHBox(
				widget.NewButton("Add Cast", func() {
					personID := castMap[castSelect.Selected]
					if personID == "" {
						dialog.ShowError(fmt.Errorf("select a cast person first"), state.Window)
						return
					}
					if _, err := state.APIClient.mutateContentRelation(personID, detail.ID, true); err != nil {
						dialog.ShowError(err, state.Window)
						return
					}
					dialog.ShowInformation("Updated", "Cast member added", state.Window)
					list.Select(id)
				}),
				widget.NewButton("Remove Cast", func() {
					personID := castMap[castSelect.Selected]
					if personID == "" {
						dialog.ShowError(fmt.Errorf("select a cast person first"), state.Window)
						return
					}
					if _, err := state.APIClient.mutateContentRelation(personID, detail.ID, false); err != nil {
						dialog.ShowError(err, state.Window)
						return
					}
					dialog.ShowInformation("Updated", "Cast member removed", state.Window)
					list.Select(id)
				}),
			),
		))

		// Video Assets Management
		assetsBox := container.NewVBox(widget.NewLabelWithStyle("Video Assets", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, asset := range detail.Assets {
			for _, key := range asset.Keys {
				assetType := asset.Type
				keyID := key.ID
				val := key.Value
				row := container.NewHBox(
					widget.NewLabel(fmt.Sprintf("[%s] %s -> %s", assetType, keyID, val)),
					widget.NewButtonWithIcon("Delete Key", theme.DeleteIcon(), func() {
						dialog.ShowConfirm("Delete Asset Key", fmt.Sprintf("Delete key %s of type %s?", keyID, assetType), func(ok bool) {
							if ok {
								_, err := state.APIClient.request("DELETE", fmt.Sprintf("/a/content/%s/assets/%s/keys/%s", detail.ID, assetType, keyID), nil, nil)
								if err != nil {
									dialog.ShowError(err, state.Window)
								} else {
									dialog.ShowInformation("Asset Deleted", "Successfully deleted asset key", state.Window)
									list.Select(id) // reload details
								}
							}
						}, state.Window)
					}),
				)
				assetsBox.Add(row)
			}
		}

		// Upload Asset Card
		assetTypeEntry := widget.NewSelect([]string{"TRAILER", "TEASER", "CLIP", "PROMO", "BTS"}, nil)
		assetTypeEntry.SetSelected("TRAILER")
		selectedAssetFileLabel := widget.NewLabel("No file selected")
		var assetVideoReader io.ReadCloser
		var assetVideoName string

		uploadAssetPicker := showFilePickerButton(state.Window, "Select Video File", selectedAssetFileLabel, func(r io.ReadCloser, name string) {
			assetVideoReader = r
			assetVideoName = name
		})

		uploadBtn := widget.NewButtonWithIcon("Upload Video Asset", theme.UploadIcon(), func() {
			if assetVideoReader == nil {
				dialog.ShowError(fmt.Errorf("please select a video file first"), state.Window)
				return
			}
			fields := map[string]string{
				"contenttype": strings.ToLower(detail.ContentType),
				"assettype":   assetTypeEntry.Selected,
			}
			files := []FormFile{
				{FieldName: "video", FileName: assetVideoName, Reader: assetVideoReader},
			}
			_, err := state.APIClient.multipartRequest("POST", fmt.Sprintf("/a/content/%s", detail.ID), fields, files)
			if err != nil {
				dialog.ShowError(err, state.Window)
			} else {
				dialog.ShowInformation("Asset Uploaded", "Successfully uploaded asset!", state.Window)
				list.Select(id)
			}
		})

		uploadBox := widget.NewCard("Upload Asset", "Upload a video file for this content", container.NewVBox(
			widget.NewLabel("Asset Type:"),
			assetTypeEntry,
			container.NewHBox(uploadAssetPicker, selectedAssetFileLabel),
			uploadBtn,
		))

		// Delete Button
		deleteContentBtn := widget.NewButtonWithIcon("Delete Content Item", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Confirm Deletion", fmt.Sprintf("Are you sure you want to permanently delete content '%s'?", detail.Title), func(ok bool) {
				if ok {
					_, err := state.APIClient.request("DELETE", fmt.Sprintf("/a/content/%s", detail.ID), nil, nil)
					if err != nil {
						dialog.ShowError(err, state.Window)
					} else {
						dialog.ShowInformation("Deleted", "Content successfully deleted", state.Window)
						detailContainer.Objects = []fyne.CanvasObject{placeholder}
						detailContainer.Refresh()
						fetchContent("", "recent")
					}
				}
			}, state.Window)
		})

		rightLayout := container.NewScroll(container.NewVBox(
			infoCard,
			widget.NewSeparator(),
			relCard,
			widget.NewSeparator(),
			coreUpdateCard,
			widget.NewSeparator(),
			imageUpdateCard,
			widget.NewSeparator(),
			relationCard,
			widget.NewSeparator(),
			assetsBox,
			widget.NewSeparator(),
			uploadBox,
			widget.NewSeparator(),
			deleteContentBtn,
		))

		detailContainer.Objects = []fyne.CanvasObject{rightLayout}
		detailContainer.Refresh()
	}

	// Action to create content form directly in the detail pane
	showCreateForm := func() {
		refreshStatus := widget.NewLabel("Reference data cache is current")
		reloadLookupData := func() {
			fetchCaches()
			refreshStatus.SetText("Reference data reloaded")
		}
		newReloadButton := func() *widget.Button {
			btn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
				reloadLookupData()
			})
			btn.Importance = widget.LowImportance
			return btn
		}

		// Elements
		titleEntry := widget.NewEntry()
		titleEntry.PlaceHolder = "e.g., Inception"

		taglineEntry := widget.NewEntry()
		taglineEntry.PlaceHolder = "e.g., Your mind is the scene of the crime"

		overviewEntry := widget.NewMultiLineEntry()
		overviewEntry.PlaceHolder = "Enter plot summary (required, max 150 chars)"

		contentTypeSelect := widget.NewSelect([]string{"MOVIE", "TV"}, nil)
		contentTypeSelect.SetSelected("MOVIE")

		langSelect := widget.NewSelect([]string{"en", "hi", "ja", "ko", "fr", "es"}, nil)
		langSelect.SetSelected("en")

		ratingSelect := widget.NewSelect([]string{"18_PLUS", "21_PLUS"}, nil)
		ratingSelect.SetSelected("18_PLUS")

		dateEntry := widget.NewEntry()
		dateEntry.PlaceHolder = "YYYY-MM-DD"

		runtimeEntry := widget.NewEntry()
		runtimeEntry.PlaceHolder = "Minutes (e.g. 148)"

		statusSelect := widget.NewSelect([]string{"RUMORED", "PLANNED", "IN_PRODUCTION", "POST_PRODUCTION", "RELEASED", "ENDED", "RETURNING_SERIES", "CANCELED", "PILOT"}, nil)
		statusSelect.SetSelected("RELEASED")

		visSelect := widget.NewSelect([]string{"PUBLIC", "PRIVATE"}, nil)
		visSelect.SetSelected("PUBLIC")

		adultCheck := widget.NewCheck("Adult (18+ rating flag)", nil)
		ratingSelect.Options = []string{"18_PLUS", "21_PLUS", "None"}
		adultCheck.SetChecked(true)
		ratingSelect.SetSelected("18_PLUS")
		adultCheck.OnChanged = func(checked bool) {
			if checked {
				ratingSelect.Enable()
				if ratingSelect.Selected == "None" || ratingSelect.Selected == "" {
					ratingSelect.SetSelected("18_PLUS")
				}
				return
			}
			ratingSelect.SetSelected("None")
			ratingSelect.Disable()
		}

		countryEntry := widget.NewEntry()
		countryEntry.PlaceHolder = "e.g., US,UK"

		// Local search helper closures for relational dropdowns
		filterAttributes := func(query string, attrType string) []EntityRef {
			res := []EntityRef{}
			q := strings.ToLower(query)
			for _, attr := range cachedAttributes {
				matchesType := false
				for _, t := range attr.AttributeType {
					if t == attrType {
						matchesType = true
						break
					}
				}
				if matchesType && strings.Contains(strings.ToLower(attr.Name), q) {
					res = append(res, EntityRef{ID: attr.ID, Name: attr.Name})
				}
			}
			return res
		}

		filterPeople := func(query string) []EntityRef {
			res := []EntityRef{}
			q := strings.ToLower(query)
			for _, p := range cachedPeople {
				if strings.Contains(strings.ToLower(p.Name), q) {
					res = append(res, EntityRef{ID: p.ID, Name: p.Name})
				}
			}
			return res
		}

		// Relational multi-select chip UI widgets
		var selectedGenres []EntityRef
		genresSelector := NewMultiSelectChipSelector("Search genres...", func(q string) []EntityRef {
			return filterAttributes(q, "GENRE")
		}, func(refs []EntityRef) {
			selectedGenres = refs
		})

		var selectedTags []EntityRef
		tagsSelector := NewMultiSelectChipSelector("Search tags...", func(q string) []EntityRef {
			return filterAttributes(q, "TAG")
		}, func(refs []EntityRef) {
			selectedTags = refs
		})

		var selectedMoods []EntityRef
		moodsSelector := NewMultiSelectChipSelector("Search mood tags...", func(q string) []EntityRef {
			return filterAttributes(q, "MOOD")
		}, func(refs []EntityRef) {
			selectedMoods = refs
		})

		var selectedStudios []EntityRef
		studiosSelector := NewMultiSelectChipSelector("Search studios...", func(q string) []EntityRef {
			return filterAttributes(q, "STUDIO")
		}, func(refs []EntityRef) {
			selectedStudios = refs
		})

		var selectedCasts []EntityRef
		castsSelector := NewMultiSelectChipSelector("Search cast members...", func(q string) []EntityRef {
			return filterPeople(q)
		}, func(refs []EntityRef) {
			selectedCasts = refs
		})

		// File Upload components
		var posterReader io.ReadCloser
		var posterName string
		posterFileLabel := widget.NewLabel("No file selected")
		posterBtn := showFilePickerButton(state.Window, "Upload Poster File", posterFileLabel, func(r io.ReadCloser, name string) {
			posterReader = r
			posterName = name
		})

		var coverReader io.ReadCloser
		var coverName string
		coverFileLabel := widget.NewLabel("No file selected")
		coverBtn := showFilePickerButton(state.Window, "Upload Backdrop File", coverFileLabel, func(r io.ReadCloser, name string) {
			coverReader = r
			coverName = name
		})

		// Form Layout
		form := widget.NewForm(
			widget.NewFormItem("Title", titleEntry),
			widget.NewFormItem("Tagline", taglineEntry),
			NewRequiredFormItem("Overview", overviewEntry),
			widget.NewFormItem("Content Type", contentTypeSelect),
			NewRequiredFormItem("Original Language", langSelect),
			widget.NewFormItem("Content Rating", ratingSelect),
			widget.NewFormItem("Release / Air Date", dateEntry),
			widget.NewFormItem("Runtime (min)", runtimeEntry),
			widget.NewFormItem("Production Status", statusSelect),
			widget.NewFormItem("Visibility", visSelect),
			widget.NewFormItem("Adult Content", adultCheck),
			widget.NewFormItem("Origin Country (comma separated)", countryEntry),
			widget.NewFormItem("Genres (Search & Select)", container.NewBorder(nil, nil, nil, newReloadButton(), genresSelector.Container)),
			widget.NewFormItem("Tags (Search & Select)", container.NewBorder(nil, nil, nil, newReloadButton(), tagsSelector.Container)),
			widget.NewFormItem("Mood Tags (Search & Select)", container.NewBorder(nil, nil, nil, newReloadButton(), moodsSelector.Container)),
			widget.NewFormItem("Studios (Search & Select)", container.NewBorder(nil, nil, nil, newReloadButton(), studiosSelector.Container)),
			widget.NewFormItem("Cast Credits (Search & Select)", container.NewBorder(nil, nil, nil, newReloadButton(), castsSelector.Container)),
			widget.NewFormItem("Poster Image File", container.NewHBox(posterBtn, posterFileLabel)),
			widget.NewFormItem("Backdrop Image File", container.NewHBox(coverBtn, coverFileLabel)),
		)

		formatRefs := func(refs []EntityRef) string {
			parts := []string{}
			for _, r := range refs {
				parts = append(parts, fmt.Sprintf("%s:%s", r.ID, r.Name))
			}
			return strings.Join(parts, ",")
		}

		submitBtn := widget.NewButtonWithIcon("Save Content", theme.ConfirmIcon(), func() {
			if overviewEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("overview is a required field"), state.Window)
				return
			}

			contentRating := ratingSelect.Selected
			if !adultCheck.Checked || contentRating == "None" {
				contentRating = ""
			}

			fields := map[string]string{
				"title":             titleEntry.Text,
				"tagline":           taglineEntry.Text,
				"overview":          overviewEntry.Text,
				"content_type":      contentTypeSelect.Selected,
				"original_language": langSelect.Selected,
				"content_rating":    contentRating,
				"status":            statusSelect.Selected,
				"visibility":        visSelect.Selected,
				"adult":             strconv.FormatBool(adultCheck.Checked),
				"origin_country":    countryEntry.Text,
				"genres":            formatRefs(selectedGenres),
				"tags":              formatRefs(selectedTags),
				"mood_tags":         formatRefs(selectedMoods),
				"studios":           formatRefs(selectedStudios),
				"casts":             formatRefs(selectedCasts),
			}

			if contentTypeSelect.Selected == "MOVIE" {
				fields["release_date"] = dateEntry.Text
			} else {
				fields["first_air_date"] = dateEntry.Text
			}

			if runtimeEntry.Text != "" {
				fields["runtime"] = runtimeEntry.Text
			}

			// Add files
			var files []FormFile
			if posterReader != nil {
				files = append(files, FormFile{FieldName: "poster", FileName: posterName, Reader: posterReader})
			}
			if coverReader != nil {
				files = append(files, FormFile{FieldName: "cover", FileName: coverName, Reader: coverReader})
			}

			_, err := state.APIClient.multipartRequest("POST", "/a/content/", fields, files)
			if err != nil {
				dialog.ShowError(err, state.Window)
			} else {
				dialog.ShowInformation("Success", "Content successfully created!", state.Window)
				detailContainer.Objects = []fyne.CanvasObject{placeholder}
				detailContainer.Refresh()
				fetchContent("", "recent")
			}
		})

		cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
			detailContainer.Objects = []fyne.CanvasObject{placeholder}
			detailContainer.Refresh()
		})

		formCard := widget.NewCard("Create Content", "Relational fields are fully searchable from cached attributes.", container.NewVBox(
			container.NewHBox(widget.NewButtonWithIcon("Refresh Reference Data", theme.ViewRefreshIcon(), func() {
				reloadLookupData()
			}), refreshStatus),
			form,
			container.NewHBox(submitBtn, cancelBtn),
		))

		detailContainer.Objects = []fyne.CanvasObject{container.NewScroll(formCard)}
		detailContainer.Refresh()
	}

	createBtn := widget.NewButtonWithIcon("New Content", theme.ContentAddIcon(), func() {
		showCreateForm()
	})

	searchEntry := widget.NewEntry()
	searchEntry.PlaceHolder = "Search content by title, overview, tagline, cast, genres, tags, moods, studios..."
	sortSelect := widget.NewSelect([]string{"recent", "latest", "alpha_asc", "alpha_desc"}, nil)
	sortSelect.SetSelected("recent")

	applySearch := func() {
		fetchContent(searchEntry.Text, sortSelect.Selected)
	}
	searchEntry.OnChanged = func(_ string) {
		applySearch()
	}
	sortSelect.OnChanged = func(_ string) {
		applySearch()
	}

	fetchContent("", "recent")

	leftPane := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Content Database", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			searchEntry,
			sortSelect,
		),
		createBtn, nil, nil,
		list,
	)

	split := container.NewHSplit(leftPane, detailContainer)
	split.Offset = 0.35
	return split
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
