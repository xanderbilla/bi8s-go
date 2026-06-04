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

func buildPeopleView(state *AppState) fyne.CanvasObject {
	detailContainer := container.NewMax()
	placeholder := widget.NewLabelWithStyle("Select a Person from the list to view profile details or view content portfolio.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	detailContainer.Objects = []fyne.CanvasObject{placeholder}

	var peopleItems []PersonDetail

	// Local cache of attributes for the searchable dropdowns
	var cachedAttributes []AttributeDetail

	fetchCaches := func() {
		attrData, err := state.APIClient.request("GET", "/a/attributes/", nil, nil)
		if err == nil {
			var resp AttributeListResponse
			if json.Unmarshal(attrData, &resp) == nil {
				cachedAttributes = resp.Data
			}
		}
	}

	list := widget.NewList(
		func() int {
			return len(peopleItems)
		},
		func() fyne.CanvasObject {
			name := widget.NewLabel("")
			name.TextStyle = fyne.TextStyle{Bold: true}
			roles := widget.NewLabel("")
			roles.TextStyle = fyne.TextStyle{Italic: true}
			return container.NewVBox(name, roles)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			name := box.Objects[0].(*widget.Label)
			roles := box.Objects[1].(*widget.Label)

			item := peopleItems[id]
			name.SetText(item.Name)
			roles.SetText(fmt.Sprintf("%s | ID: %s", strings.Join(item.Roles, ", "), item.ID))
		},
	)

	fetchPeople := func(searchText string, sortMode string) {
		fetchCaches()
		searchPath := "/a/search?entity=people&limit=100&sort=" + url.QueryEscape(sortMode)
		if strings.TrimSpace(searchText) != "" {
			searchPath += "&q=" + url.QueryEscape(strings.TrimSpace(searchText))
		}

		data, err := state.APIClient.request("GET", searchPath, nil, nil)
		if err == nil {
			var resp AdminSearchResponse
			if json.Unmarshal(data, &resp) == nil && resp.Data.People != nil {
				peopleItems = resp.Data.People.Items
				state.PeopleList = peopleItems
				list.Refresh()
				return
			}
		}

		fallbackData, fallbackErr := state.APIClient.request("GET", "/a/people/?limit=100", nil, nil)
		if fallbackErr != nil {
			dialog.ShowError(fmt.Errorf("failed to fetch people: %w", fallbackErr), state.Window)
			return
		}

		var fallbackResp PeopleListResponse
		if err := json.Unmarshal(fallbackData, &fallbackResp); err != nil {
			dialog.ShowError(fmt.Errorf("failed to parse response: %w", err), state.Window)
			return
		}

		filtered := make([]PersonDetail, 0, len(fallbackResp.Data.Items))
		q := strings.ToLower(strings.TrimSpace(searchText))
		for _, item := range fallbackResp.Data.Items {
			if q == "" || strings.Contains(strings.ToLower(item.Name+" "+item.LegalName+" "+item.StageName+" "+item.Bio+" "+item.Nationality+" "+item.BirthPlace), q) {
				filtered = append(filtered, item)
			}
		}
		peopleItems = filtered
		state.PeopleList = peopleItems
		list.Refresh()
	}

	state.RefreshPeopleList = func() { fetchPeople("", "recent") }

	list.OnSelected = func(id widget.ListItemID) {
		selected := peopleItems[id]
		state.SelectedPerson = &selected

		// Load Full Admin Detail
		fullData, err := state.APIClient.request("GET", fmt.Sprintf("/a/people/%s", selected.ID), nil, nil)
		if err != nil {
			dialog.ShowError(err, state.Window)
			return
		}
		var detailResp PersonResponse
		if err := json.Unmarshal(fullData, &detailResp); err != nil {
			dialog.ShowError(err, state.Window)
			return
		}
		detail := detailResp.Data
		state.SelectedPerson = &detail

		// Fetch credits
		creditsText := "Content Credits:\n"
		creditsData, err := state.APIClient.request("GET", fmt.Sprintf("/a/people/%s/content", detail.ID), nil, nil)
		if err == nil {
			var creditsResp ContentListResponse
			if json.Unmarshal(creditsData, &creditsResp) == nil {
				if len(creditsResp.Data.Items) == 0 {
					creditsText += "  - None registered"
				}
				for _, credit := range creditsResp.Data.Items {
					creditsText += fmt.Sprintf("  - %s (ID: %s)\n", credit.Title, credit.ID)
				}
			}
		}

		infoCard := widget.NewCard(detail.Name, fmt.Sprintf("Person Detail (ID: %s)", detail.ID), container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Legal Name: %s", detail.LegalName)),
			widget.NewLabel(fmt.Sprintf("Stage Name: %s", detail.StageName)),
			widget.NewLabel(fmt.Sprintf("Roles: %s", strings.Join(detail.Roles, ", "))),
			widget.NewLabel(fmt.Sprintf("Gender: %s", detail.Gender)),
			widget.NewLabel(fmt.Sprintf("Career Status: %s", detail.CareerStatus)),
			widget.NewLabel(fmt.Sprintf("Birth Date: %s", detail.BirthDate)),
			widget.NewLabel(fmt.Sprintf("Birth Place: %s", detail.BirthPlace)),
			widget.NewLabel(fmt.Sprintf("Bio: %s", detail.Bio)),
			widget.NewLabel(fmt.Sprintf("Height: %d cm | Weight: %d kg", detail.Height, detail.Weight)),
			widget.NewLabel(fmt.Sprintf("Active: %v | Verified: %v", detail.Active, detail.Verified)),
			widget.NewLabel(fmt.Sprintf("Profile Path: %s", detail.ProfilePath)),
			widget.NewLabel(fmt.Sprintf("Backdrop Path: %s", detail.BackdropPath)),
			widget.NewSeparator(),
			widget.NewLabel(creditsText),
		))

		// Display relations
		tagsStr := []string{}
		for _, t := range detail.Tags {
			tagsStr = append(tagsStr, t.Name)
		}
		catsStr := []string{}
		for _, c := range detail.Categories {
			catsStr = append(catsStr, c.Name)
		}
		specsStr := []string{}
		for _, s := range detail.Specialties {
			specsStr = append(specsStr, s.Name)
		}

		relCard := widget.NewCard("Relations", "Attribute references mapping", container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Tags: %s", strings.Join(tagsStr, ", "))),
			widget.NewLabel(fmt.Sprintf("Categories: %s", strings.Join(catsStr, ", "))),
			widget.NewLabel(fmt.Sprintf("Specialties: %s", strings.Join(specsStr, ", "))),
		))

		deleteBtn := widget.NewButtonWithIcon("Delete Person Profile", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Confirm Profile Deletion", fmt.Sprintf("Are you sure you want to delete profile for '%s'?", detail.Name), func(ok bool) {
				if ok {
					_, err := state.APIClient.request("DELETE", fmt.Sprintf("/a/people/%s", detail.ID), nil, nil)
					if err != nil {
						dialog.ShowError(err, state.Window)
					} else {
						dialog.ShowInformation("Deleted", "Person profile deleted successfully", state.Window)
						detailContainer.Objects = []fyne.CanvasObject{placeholder}
						detailContainer.Refresh()
						fetchPeople("", "recent")
					}
				}
			}, state.Window)
		})

		detailScroll := container.NewScroll(container.NewVBox(infoCard, relCard, deleteBtn))
		detailContainer.Objects = []fyne.CanvasObject{detailScroll}
		detailContainer.Refresh()
	}

	// Action to create person inline
	showCreateForm := func() {
		nameEntry := widget.NewEntry()
		nameEntry.PlaceHolder = "e.g., Tim Robbins"

		legalEntry := widget.NewEntry()
		legalEntry.PlaceHolder = "Legal Full Name"

		stageEntry := widget.NewEntry()
		stageEntry.PlaceHolder = "Screen or Stage Alias"

		rolesSelect := widget.NewSelect([]string{"PERFORMER", "CONTENT_CREATOR", "PERFORMER,CONTENT_CREATOR"}, nil)
		rolesSelect.SetSelected("PERFORMER")

		genderSelect := widget.NewSelect([]string{"Male", "Female", "Trans"}, nil)
		genderSelect.SetSelected("Female")

		careerSelect := widget.NewSelect([]string{"Active", "Retired", "Hiatus"}, nil)
		careerSelect.SetSelected("Active")

		bioEntry := widget.NewMultiLineEntry()
		bioEntry.PlaceHolder = "Enter brief biography (max 150 chars)"

		birthDateEntry := widget.NewEntry()
		birthDateEntry.PlaceHolder = "YYYY-MM-DD"

		birthPlaceEntry := widget.NewEntry()
		birthPlaceEntry.PlaceHolder = "e.g., Los Angeles, CA"

		nationalityEntry := widget.NewEntry()
		nationalityEntry.PlaceHolder = "e.g., American"

		heightEntry := widget.NewEntry()
		heightEntry.PlaceHolder = "Height in cm (e.g. 182)"

		weightEntry := widget.NewEntry()
		weightEntry.PlaceHolder = "Weight in kg (e.g. 78)"

		activeCheck := widget.NewCheck("Is Active Profile", nil)
		activeCheck.Checked = true
		verifiedCheck := widget.NewCheck("Is Profile Verified", nil)

		debutEntry := widget.NewEntry()
		debutEntry.PlaceHolder = "Year (e.g. 1994)"

		aliasesEntry := widget.NewEntry()
		aliasesEntry.PlaceHolder = "Alternative alias names (comma separated)"

		// Measurements
		bustEntry := widget.NewEntry()
		bustEntry.PlaceHolder = "Bust size"
		waistEntry := widget.NewEntry()
		waistEntry.PlaceHolder = "Waist size"
		hipsEntry := widget.NewEntry()
		hipsEntry.PlaceHolder = "Hips size"
		unitSelect := widget.NewSelect([]string{"inches", "cm"}, nil)
		unitSelect.SetSelected("inches")
		bodyTypeEntry := widget.NewEntry()
		bodyTypeEntry.PlaceHolder = "e.g., Athletic"
		eyeEntry := widget.NewEntry()
		eyeEntry.PlaceHolder = "e.g., Blue"
		hairEntry := widget.NewEntry()
		hairEntry.PlaceHolder = "e.g., Blonde"

		// Local search helper
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

		// Relational multi-select chip UI widgets
		var selectedTags []EntityRef
		tagsSelector := NewMultiSelectChipSelector("Search tags...", func(q string) []EntityRef {
			return filterAttributes(q, "TAG")
		}, func(refs []EntityRef) {
			selectedTags = refs
		})

		var selectedCats []EntityRef
		catsSelector := NewMultiSelectChipSelector("Search categories...", func(q string) []EntityRef {
			return filterAttributes(q, "CATEGORY")
		}, func(refs []EntityRef) {
			selectedCats = refs
		})

		var selectedSpecs []EntityRef
		specsSelector := NewMultiSelectChipSelector("Search specialties...", func(q string) []EntityRef {
			return filterAttributes(q, "SPECIALITY")
		}, func(refs []EntityRef) {
			selectedSpecs = refs
		})

		// Image upload variables
		var profileReader io.ReadCloser
		var profileName string
		profileLabel := widget.NewLabel("No file selected")
		profileBtn := showFilePickerButton(state.Window, "Upload Profile Image", profileLabel, func(r io.ReadCloser, name string) {
			profileReader = r
			profileName = name
		})

		var backdropReader io.ReadCloser
		var backdropName string
		backdropLabel := widget.NewLabel("No file selected")
		backdropBtn := showFilePickerButton(state.Window, "Upload Backdrop Image", backdropLabel, func(r io.ReadCloser, name string) {
			backdropReader = r
			backdropName = name
		})

		// Form items
		form := widget.NewForm(
			NewRequiredFormItem("Name", nameEntry),
			widget.NewFormItem("Legal Name", legalEntry),
			widget.NewFormItem("Stage Name", stageEntry),
			NewRequiredFormItem("Roles Selection", rolesSelect),
			NewRequiredFormItem("Gender", genderSelect),
			NewRequiredFormItem("Career Status", careerSelect),
			widget.NewFormItem("Biography", bioEntry),
			widget.NewFormItem("Birth Date", birthDateEntry),
			widget.NewFormItem("Birth Place", birthPlaceEntry),
			widget.NewFormItem("Nationality", nationalityEntry),
			widget.NewFormItem("Height (cm)", heightEntry),
			widget.NewFormItem("Weight (kg)", weightEntry),
			widget.NewFormItem("Status Flags", container.NewVBox(activeCheck, verifiedCheck)),
			widget.NewFormItem("Debut Year", debutEntry),
			widget.NewFormItem("Aliases", aliasesEntry),
			widget.NewFormItem("Bust Size", bustEntry),
			widget.NewFormItem("Waist Size", waistEntry),
			widget.NewFormItem("Hips Size", hipsEntry),
			widget.NewFormItem("Unit", unitSelect),
			widget.NewFormItem("Body Details (Type/Eye/Hair)", container.NewVBox(bodyTypeEntry, eyeEntry, hairEntry)),
			widget.NewFormItem("Tags (Search & Select)", tagsSelector.Container),
			widget.NewFormItem("Categories (Search & Select)", catsSelector.Container),
			widget.NewFormItem("Specialties (Search & Select)", specsSelector.Container),
			widget.NewFormItem("Profile Picture", container.NewHBox(profileBtn, profileLabel)),
			widget.NewFormItem("Cover Backdrop Banner", container.NewHBox(backdropBtn, backdropLabel)),
		)

		formatRefs := func(refs []EntityRef) string {
			parts := []string{}
			for _, r := range refs {
				parts = append(parts, fmt.Sprintf("%s:%s", r.ID, r.Name))
			}
			return strings.Join(parts, ",")
		}

		submitBtn := widget.NewButtonWithIcon("Save Person Profile", theme.ConfirmIcon(), func() {
			if nameEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("name is a required field"), state.Window)
				return
			}

			fields := map[string]string{
				"name":          nameEntry.Text,
				"legal_name":    legalEntry.Text,
				"stage_name":    stageEntry.Text,
				"roles":         rolesSelect.Selected,
				"gender":        genderSelect.Selected,
				"career_status": careerSelect.Selected,
				"bio":           bioEntry.Text,
				"birth_date":    birthDateEntry.Text,
				"birth_place":   birthPlaceEntry.Text,
				"nationality":   nationalityEntry.Text,
				"active":        strconv.FormatBool(activeCheck.Checked),
				"verified":      strconv.FormatBool(verifiedCheck.Checked),
				"aliases":       aliasesEntry.Text,
				"tags":          formatRefs(selectedTags),
				"categories":    formatRefs(selectedCats),
				"specialties":   formatRefs(selectedSpecs),
			}

			if heightEntry.Text != "" {
				fields["height"] = heightEntry.Text
			}
			if weightEntry.Text != "" {
				fields["weight"] = weightEntry.Text
			}
			if debutEntry.Text != "" {
				fields["debut_year"] = debutEntry.Text
			}

			// Add measurements if present
			if bustEntry.Text != "" {
				fields["measurements_bust"] = bustEntry.Text
			}
			if waistEntry.Text != "" {
				fields["measurements_waist"] = waistEntry.Text
			}
			if hipsEntry.Text != "" {
				fields["measurements_hips"] = hipsEntry.Text
			}
			fields["measurements_unit"] = unitSelect.Selected
			fields["measurements_body_type"] = bodyTypeEntry.Text
			fields["measurements_eye_color"] = eyeEntry.Text
			fields["measurements_hair_color"] = hairEntry.Text

			// Prepare files
			var files []FormFile
			if profileReader != nil {
				files = append(files, FormFile{FieldName: "profile", FileName: profileName, Reader: profileReader})
			}
			if backdropReader != nil {
				files = append(files, FormFile{FieldName: "backdrop", FileName: backdropName, Reader: backdropReader})
			}

			_, err := state.APIClient.multipartRequest("POST", "/a/people/", fields, files)
			if err != nil {
				dialog.ShowError(err, state.Window)
			} else {
				dialog.ShowInformation("Success", "Person successfully created!", state.Window)
				detailContainer.Objects = []fyne.CanvasObject{placeholder}
				detailContainer.Refresh()
				fetchPeople("", "recent")
			}
		})

		cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
			detailContainer.Objects = []fyne.CanvasObject{placeholder}
			detailContainer.Refresh()
		})

		formCard := widget.NewCard("Create Person", "Relational fields are fully searchable from cached attributes.", container.NewVBox(
			form,
			container.NewHBox(submitBtn, cancelBtn),
		))

		detailContainer.Objects = []fyne.CanvasObject{container.NewScroll(formCard)}
		detailContainer.Refresh()
	}

	createBtn := widget.NewButtonWithIcon("New Person", theme.ContentAddIcon(), func() {
		showCreateForm()
	})

	searchEntry := widget.NewEntry()
	searchEntry.PlaceHolder = "Search people by name, legal name, stage name, bio, roles, tags..."
	sortSelect := widget.NewSelect([]string{"recent", "latest", "alpha_asc", "alpha_desc"}, nil)
	sortSelect.SetSelected("recent")
	applySearch := func() {
		fetchPeople(searchEntry.Text, sortSelect.Selected)
	}
	searchEntry.OnChanged = func(_ string) {
		applySearch()
	}
	sortSelect.OnChanged = func(_ string) {
		applySearch()
	}

	fetchPeople("", "recent")

	leftPane := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("People Directory", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
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
