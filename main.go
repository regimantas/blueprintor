package main

import (
	"archive/zip"
	"embed"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

//go:embed res/*
var resFS embed.FS

func main() {
	// Create "templates" folder if it does not exist
	err := os.MkdirAll("templates", 0755)
	if err != nil {
		panic(err)
	}

	a := app.New()
	w := a.NewWindow("Android Template Helper")

	data, err := resFS.ReadFile("res/folder-add-line.png")
	if err != nil {
		panic(err)
	}
	iconRes := fyne.NewStaticResource("folder-add-line.png", data)

	data, err = resFS.ReadFile("res/folder-reduce-line.png")
	if err != nil {
		panic(err)
	}
	removeIconRes := fyne.NewStaticResource("folder-reduce-line.png", data)

	defaultIconData, err := resFS.ReadFile("res/folder-image-line.png")
	if err != nil {
		panic(err)
	}
	defaultIconRes := fyne.NewStaticResource("folder-image-line.png", defaultIconData)

	hammerIconData, err := resFS.ReadFile("res/hammer-line.png")
	if err != nil {
		panic(err)
	}
	hammerIconRes := fyne.NewStaticResource("hammer-line.png", hammerIconData)

	settingsIconData, err := resFS.ReadFile("res/settings-3-line.png")
	if err != nil {
		panic(err)
	}
	settingsIconRes := fyne.NewStaticResource("settings-3-line.png", settingsIconData)

	// Templates slice
	templates := []string{}
	selectedIndex := -1

	// Read all .zip files from templates folder and add names (without .zip) to the list
	files, err := ioutil.ReadDir("templates")
	if err == nil {
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".zip" {
				name := f.Name()
				templates = append(templates, name[:len(name)-4]) // remove ".zip"
			}
		}
	}

	templateList := widget.NewList(
		func() int { return len(templates) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i int, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(templates[i])
		},
	)
	templateList.OnSelected = func(id int) {
		selectedIndex = id
	}
	templateList.OnUnselected = func(id int) {
		selectedIndex = -1
	}

	btn := widget.NewButtonWithIcon("Template", iconRes, func() {
		home, _ := os.UserHomeDir()
		androidProjects := filepath.Join(home, "AndroidStudioProjects")
		startDir := storage.NewFileURI(androidProjects)
		lister, err := storage.ListerForURI(startDir)
		if err != nil {
			lister = nil // jei neranda, atidarys default
		}
		folderDialog := dialog.NewFolderOpen(
			func(list fyne.ListableURI, err error) {
				if err != nil || list == nil {
					return
				}
				folderPath := list.Path()
				baseName := filepath.Base(folderPath)
				zipPath := filepath.Join("templates", baseName+".zip")

				// Create ZIP file
				zipFile, err := os.Create(zipPath)
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				defer zipFile.Close()

				zipWriter := zip.NewWriter(zipFile)
				err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					relPath, err := filepath.Rel(folderPath, path)
					if err != nil {
						return err
					}
					if info.IsDir() {
						if relPath == "." {
							return nil
						}
						_, err := zipWriter.Create(relPath + "/")
						return err
					}
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()
					wr, err := zipWriter.Create(relPath)
					if err != nil {
						return err
					}
					_, err = io.Copy(wr, file)
					return err
				})
				if err != nil {
					dialog.ShowError(err, w)
					zipWriter.Close()
					return
				}
				zipWriter.Close()

				// Add to list (show only name, not .zip)
				templates = append(templates, baseName)
				templateList.Refresh()
				dialog.ShowInformation("Template Added", "Template zipped and saved to templates/"+baseName+".zip", w)
			},
			w,
		)
		folderDialog.SetLocation(lister)
		folderDialog.Show()
	})

	removeBtn := widget.NewButtonWithIcon("Delete", removeIconRes, func() {
		if selectedIndex >= 0 && selectedIndex < len(templates) {
			name := templates[selectedIndex]
			dialog.ShowConfirm("Delete", "Are you sure you want to delete \""+name+"\"?", func(confirm bool) {
				if confirm {
					// Remove from slice
					templates = append(templates[:selectedIndex], templates[selectedIndex+1:]...)
					templateList.Refresh()
					// Remove zip file
					zipPath := filepath.Join("templates", name+".zip")
					_ = os.Remove(zipPath)
					selectedIndex = -1
				}
			}, w)
		}
	})

	var chosenIconPath string // globalus kintamasis

	iconBtn := widget.NewButtonWithIcon("Choose icon", defaultIconRes, nil)
	iconBtn.OnTapped = func() {
		home, _ := os.UserHomeDir()
		startDir := storage.NewFileURI(home)
		lister, err := storage.ListerForURI(startDir)
		if err != nil {
			lister = nil // jei neranda, atidarys default
		}
		fileDialog := dialog.NewFileOpen(
			func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil {
					return
				}
				defer reader.Close()
				data, err := ioutil.ReadAll(reader)
				if err != nil {
					return
				}
				chosenRes := fyne.NewStaticResource(reader.URI().Name(), data)
				iconBtn.SetIcon(chosenRes)
				chosenIconPath = reader.URI().Path() // išsaugome kelią
			},
			w,
		)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
		fileDialog.SetLocation(lister)
		fileDialog.Show()
	}

	projectNameEntry := widget.NewEntry()
	projectNameEntry.SetText("DemoProject")

	packageNameEntry := widget.NewEntry()
	packageNameEntry.SetText("com.example.demo")

	// Generate new project button with icon
	generateBtn := widget.NewButtonWithIcon("Generate Project", hammerIconRes, func() {
		if selectedIndex < 0 || selectedIndex >= len(templates) {
			dialog.ShowError(fmt.Errorf("no template selected: please select a template from the list"), w)
			return
		}
		home, _ := os.UserHomeDir()
		androidProjects := filepath.Join(home, "AndroidStudioProjects")
		zipName := templates[selectedIndex] + ".zip"
		zipPath := filepath.Join("templates", zipName)
		targetDir := filepath.Join(androidProjects, projectNameEntry.Text)
		oldProjectName, oldPackage, oldAppLabel, _ := DetectOldProjectData(zipPath)
		err := UnzipAndReplaceFull(
			zipPath,
			targetDir,
			oldProjectName,
			projectNameEntry.Text,
			oldPackage,
			packageNameEntry.Text,
			oldAppLabel,
			projectNameEntry.Text,
		)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		// Perkeliam katalogą pagal naują package ID
		if oldPackage != "" && oldPackage != packageNameEntry.Text {
			_ = RenameJavaPackageDir(targetDir, oldPackage, packageNameEntry.Text)
			_ = FixJavaKotlinPackage(targetDir, packageNameEntry.Text)
		}

		// Jei pasirinkta ikona, generuojame Android ikonų dydžius
		if chosenIconPath != "" {
			err = ReplaceAndroidIcons(targetDir, chosenIconPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("nepavyko pakeisti ikonų: %v", err), w)
				return
			}
		}
		manifestPath := filepath.Join(targetDir, "app", "src", "main", "AndroidManifest.xml")
		_ = EnsureManifestIcons(manifestPath)
		dialog.ShowInformation("Project Generated", "Project "+projectNameEntry.Text+" created successfully in "+androidProjects, w)
	})

	column1 := container.NewBorder(
		btn,
		removeBtn,
		nil,
		nil,
		container.NewVScroll(templateList),
	)

	column2 := container.NewVBox(
		widget.NewLabel("Project name"),
		projectNameEntry,
		widget.NewLabel("Package name"),
		packageNameEntry,
		widget.NewLabel("Select icon"),
		iconBtn,
		layout.NewSpacer(),
		generateBtn,
	)

	column3 := container.NewVBox(
		widget.NewButtonWithIcon("Settings", settingsIconRes, func() {
			dialog.ShowInformation("Settings", "Settings dialog would appear here.", w)
		}),
	)

	content := container.NewGridWithColumns(3,
		column1,
		column2,
		column3,
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(600, 450))
	w.ShowAndRun()
}
