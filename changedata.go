package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
)

// Helper: extract old project name and package from zip
func DetectOldProjectData(templateZipPath string) (oldProjectName, oldPackage, oldAppLabel string, err error) {
	r, err := zip.OpenReader(templateZipPath)
	if err != nil {
		return "", "", "", err
	}
	defer r.Close()

	// Heuristics: look for settings.gradle, build.gradle, AndroidManifest.xml, res/values/strings.xml
	for _, f := range r.File {
		lower := strings.ToLower(f.Name)
		if strings.Contains(lower, "settings.gradle") || strings.Contains(lower, "build.gradle") || strings.Contains(lower, "androidmanifest.xml") || strings.Contains(lower, "res/values/strings.xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, _ := io.ReadAll(rc)
			rc.Close()
			text := string(content)

			// Try to find project name (settings.gradle: rootProject.name = 'DemoProject')
			if oldProjectName == "" {
				re := regexp.MustCompile(`rootProject\.name\s*=\s*['"]([^'"]+)['"]`)
				if m := re.FindStringSubmatch(text); len(m) > 1 {
					oldProjectName = m[1]
				}
			}
			// Try to find package (AndroidManifest.xml: package="com.example.demo")
			if oldPackage == "" {
				re := regexp.MustCompile(`package\s*=\s*['"]([^'"]+)['"]`)
				if m := re.FindStringSubmatch(text); len(m) > 1 {
					oldPackage = m[1]
				}
			}
			// Try to find applicationId (build.gradle: applicationId "com.example.demo")
			if oldPackage == "" {
				re := regexp.MustCompile(`applicationId\s*['"]([^'"]+)['"]`)
				if m := re.FindStringSubmatch(text); len(m) > 1 {
					oldPackage = m[1]
				}
			}
			// Find <string name="app_name">BLE APP</string>
			if oldAppLabel == "" {
				re := regexp.MustCompile(`<string\s+name="app_name"\s*>([^<]+)</string>`)
				if m := re.FindStringSubmatch(text); len(m) > 1 {
					oldAppLabel = m[1]
				}
			}
		}
	}
	return oldProjectName, oldPackage, oldAppLabel, nil
}

// Unzip and replace using only new data, old data is detected automatically
func UnzipAndReplaceAuto(
	templateZipPath string,
	targetDir string,
	newProjectName string,
	newPackage string,
) error {
	oldProjectName, oldPackage, oldAppLabel, err := DetectOldProjectData(templateZipPath)
	if err != nil {
		return err
	}
	if oldProjectName == "" || oldPackage == "" {
		return os.ErrInvalid
	}
	return UnzipAndReplaceFull(templateZipPath, targetDir, oldProjectName, newProjectName, oldPackage, newPackage, oldAppLabel, newProjectName)
}

// Unzip and replace project name and package in all files and folders
func UnzipAndReplace(
	templateZipPath string,
	targetDir string,
	oldProjectName string,
	newProjectName string,
	oldPackage string,
	newPackage string,
) error {
	r, err := zip.OpenReader(templateZipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Replace project name in file/folder names
		newName := strings.ReplaceAll(f.Name, oldProjectName, newProjectName)
		newName = strings.ReplaceAll(newName, oldPackage, newPackage)
		destPath := filepath.Join(targetDir, newName)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		// Create parent dirs if needed
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		// Read file content and replace project/package names
		content, err := io.ReadAll(rc)
		if err != nil {
			return err
		}
		s := string(content)
		s = replaceAllCaseInsensitive(s, oldProjectName, newProjectName)
		s = replaceAllCaseInsensitive(s, oldPackage, newPackage)

		// Write replaced content
		if err := os.WriteFile(destPath, []byte(s), f.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func UnzipAndReplaceFull(
	templateZipPath string,
	targetDir string,
	oldProjectName string,
	newProjectName string,
	oldPackage string,
	newPackage string,
	oldAppLabel string,
	newAppLabel string,
) error {
	r, err := zip.OpenReader(templateZipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		newName := strings.ReplaceAll(f.Name, oldProjectName, newProjectName)
		newName = strings.ReplaceAll(newName, oldPackage, newPackage)
		destPath := filepath.Join(targetDir, newName)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		content, err := io.ReadAll(rc)
		if err != nil {
			return err
		}
		s := string(content)
		s = replaceAllCaseInsensitive(s, oldProjectName, newProjectName)
		s = replaceAllCaseInsensitive(s, oldPackage, newPackage)
		if oldAppLabel != "" {
			s = strings.ReplaceAll(s, oldAppLabel, newAppLabel)
		}

		// Papildomai: jei tai AndroidManifest.xml, keisk android:label
		if strings.HasSuffix(strings.ToLower(f.Name), "androidmanifest.xml") {
			// icon
			reIcon := regexp.MustCompile(`android:icon\s*=\s*["'][^"']*["']`)
			s = reIcon.ReplaceAllString(s, `android:icon="@mipmap/ic_launcher"`)

			// roundIcon
			reRound := regexp.MustCompile(`android:roundIcon\s*=\s*["'][^"']*["']`)
			s = reRound.ReplaceAllString(s, `android:roundIcon="@mipmap/ic_launcher_round"`)

			// android:label
			reLabel := regexp.MustCompile(`android:label\s*=\s*["'][^"']*["']`)
			s = reLabel.ReplaceAllString(s, `android:label="`+newAppLabel+`"`)

			// versionCode +1
			reVC := regexp.MustCompile(`versionCode\s*=\s*["'](\d+)["']`)
			s = reVC.ReplaceAllStringFunc(s, func(m string) string {
				num, _ := strconv.Atoi(reVC.FindStringSubmatch(m)[1])
				return fmt.Sprintf(`versionCode="%d"`, num+1)
			})
		}

		if err := os.WriteFile(destPath, []byte(s), f.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func replaceAllCaseInsensitive(s, old, new string) string {
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(old))
	return re.ReplaceAllString(s, new)
}

// Generate and replace Android icons in the project
func ReplaceAndroidIcons(projectDir string, iconPath string) error {
	// Android icon sizes
	type iconDef struct {
		folder string
		size   int
	}
	icons := []iconDef{
		{"mipmap-mdpi", 48},
		{"mipmap-hdpi", 72},
		{"mipmap-xhdpi", 96},
		{"mipmap-xxhdpi", 144},
		{"mipmap-xxxhdpi", 192},
	}

	img, err := imaging.Open(iconPath)
	if err != nil {
		return err
	}

	iconNames := []string{"ic_launcher", "ic_launcher_round"}

	for _, ic := range icons {
		iconDir := filepath.Join(projectDir, "app", "src", "main", "res", ic.folder)
		if err := os.MkdirAll(iconDir, 0755); err != nil {
			return err
		}
		for _, n := range iconNames {
			resized := imaging.Resize(img, ic.size, ic.size, imaging.Lanczos)
			iconFile := filepath.Join(iconDir, n+".png")
			if err := imaging.Save(resized, iconFile); err != nil {
				return err
			}
			// Pašalinam .webp dublikatą
			webpFile := filepath.Join(iconDir, n+".webp")
			_ = os.Remove(webpFile)
		}
	}

	// Pašalinam visus ic_launcher.xml ir ic_launcher_round.xml iš mipmap-* ir drawable*
	globs := []string{
		"app/src/main/res/mipmap-*/ic_launcher.xml",
		"app/src/main/res/mipmap-*/ic_launcher_round.xml",
		"app/src/main/res/drawable*/ic_launcher.xml",
		"app/src/main/res/drawable*/ic_launcher_round.xml",
	}
	for _, pattern := range globs {
		files, _ := filepath.Glob(filepath.Join(projectDir, pattern))
		for _, f := range files {
			_ = os.Remove(f)
		}
	}

	return nil
}

// Call this after UnzipAndReplaceFull and ReplaceAndroidIcons
func EnsureManifestIcons(manifestPath string) error {
	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	s := string(data)

	// Jei nėra android:icon, įterpiam
	if !strings.Contains(s, `android:icon="`) {
		s = strings.Replace(s, "<application", `<application android:icon="@mipmap/ic_launcher"`, 1)
	}
	// Jei nėra android:roundIcon, įterpiam
	if !strings.Contains(s, `android:roundIcon="`) {
		s = strings.Replace(s, "<application", `<application android:roundIcon="@mipmap/ic_launcher_round"`, 1)
	}

	return ioutil.WriteFile(manifestPath, []byte(s), 0644)
}

// Rekursyviai perkelia Java package katalogą pagal naują ID
func RenameJavaPackageDir(projectDir, oldPackage, newPackage string) error {
	oldPath := filepath.Join(projectDir, "app", "src", "main", "java", filepath.FromSlash(strings.ReplaceAll(oldPackage, ".", "/")))
	newPath := filepath.Join(projectDir, "app", "src", "main", "java", filepath.FromSlash(strings.ReplaceAll(newPackage, ".", "/")))

	// Jei keliai sutampa, nieko nedarom
	if oldPath == newPath {
		return nil
	}

	// Sukuriam naują katalogą (jei reikia)
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}

	// Perkeliam visą katalogą (su visais failais ir poaplankiais)
	err := os.Rename(oldPath, newPath)
	if err != nil {
		return err
	}

	// Pašalinam tuščius tėvinius katalogus, jei reikia
	parent := filepath.Dir(oldPath)
	for parent != filepath.Join(projectDir, "app", "src", "main", "java") {
		os.Remove(parent)
		parent = filepath.Dir(parent)
	}

	return nil
}

// Po katalogo pervadinimo, atnaujina package eilutę visuose .kt ir .java failuose
func FixJavaKotlinPackage(projectDir, newPackage string) error {
	packageDir := filepath.Join(projectDir, "app", "src", "main", "java")
	return filepath.Walk(packageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".java") {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			s := string(data)
			// Pakeičiam package eilutę į naują
			re := regexp.MustCompile(`(?m)^package\s+[\w\.]+`)
			s = re.ReplaceAllString(s, "package "+newPackage)
			return ioutil.WriteFile(path, []byte(s), info.Mode())
		}
		return nil
	})
}
