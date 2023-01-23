package utils

import (
	"cmk_getter/config"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const cmkVersionUrl = "version"

var cmkSite = config.ConfigCmkGetter.Site
var cmkDomain = config.ConfigCmkGetter.Domain

const urlTemplate = "https://%s/%s/check_mk/api/1.0/%s"
const downloadUrlTemplate = "https://%s/%s/check_mk/api/1.0/domain-types/agent/actions/download/invoke?os_type=linux_deb"

var CurrentVersion string = ""

func init() {
	// Get the current version of check_mk
	cmkVersionUrl := fmt.Sprintf(urlTemplate, config.ConfigCmkGetter.Domain, config.ConfigCmkGetter.Site, cmkVersionUrl)
	_, response, err := GetUrl("json", cmkVersionUrl)
	if err != nil {
		log.Fatal(err)
	}
	cmkVersion, err := GetCmkVersion(response)
	if err != nil {
		log.Fatal(err)
	}
	CurrentVersion = cmkVersion.CroppedVersion()
}

type CmkVersionChanges struct {
	Version         string `json:"version"`
	ErrorString     string `json:"error_string"`
	TriggerDownload bool   `json:"trigger_download"`
	Folder          string `json:"folder"`
}

type CmkVersionResponse struct {
	Site    string `json:"site"`
	Group   string `json:"group"`
	RestApi struct {
		Revision string `json:"revision"`
	} `json:"rest_api"`
	Versions struct {
		Apache  []int  `json:"apache"`
		Checkmk string `json:"checkmk"`
		Python  string `json:"python"`
		ModWsgi []int  `json:"mod_wsgi"`
		Wsgi    []int  `json:"wsgi"`
	} `json:"versions"`
	Edition string `json:"edition"`
	Demo    bool   `json:"demo"`
}

func BearerToken() string {
	// Generate Bearer Token from Username and Password with base64
	username := config.ConfigCmkGetter.Username
	password := config.ConfigCmkGetter.Password
	return fmt.Sprintf("Bearer %s %s", username, password)
}

// GetUrl Get url from the API as []byte
func GetUrl(getType, url string) (http.Header, []byte, error) {
	// Create client
	client := &http.Client{}
	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	// Add Bearer Token to the request
	req.Header.Add("Authorization", BearerToken())
	// Set application/json to the request
	switch getType {
	case "json":
		req.Header.Add("Accept", "application/json")
	case "file":
		req.Header.Add("Accept", "application/octet-stream")
	}
	// Set User-Agent to the request
	req.Header.Add("User-Agent", "cmk_getter")

	// Get response
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	// Get status code
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return resp.Header, body, nil
}

// GetCmkVersion Get the current version of check_mk from the API
func GetCmkVersion(response []byte) (CmkVersionResponse, error) {
	// Create CmkVersionResponse struct
	var cmkVersion CmkVersionResponse

	// Decode response to CmkVersionResponse struct
	err := json.Unmarshal(response, &cmkVersion)
	if err != nil {
		return cmkVersion, err
	}

	return cmkVersion, nil
}

// CroppedVersion crop CmkVersionResponse.Edition from CmkVersionResponse.Version.Checkmk
func (c *CmkVersionResponse) CroppedVersion() string {
	cropString := "." + c.Edition
	return strings.Replace(c.Versions.Checkmk, cropString, "", 1)
}

// IsSameVersion Find the current version of check_mk from the API in files folder
// And return bool if the version is the same
func (c *CmkVersionResponse) IsSameVersion(folderPath string) (bool, error) {
	// Create folder if not exists
	err := os.MkdirAll(folderPath, 0755)
	if err != nil {
		return false, err
	}

	// Get the files names in the folder
	files, err := GetFiles(folderPath)
	if err != nil {
		return false, err
	}
	// Find CmkVersionResponse.Version.Checkmk in the file names
	for _, file := range files {
		// Check if CmkVersionResponse.Version.Checkmk contains the file name
		if strings.Contains(file, c.CroppedVersion()) {
			return true, nil
		}
	}
	return false, nil
}

func CreateSymlink(folderPath, currentVersion string) error {
	// Create symlink
	oldFilename := "check-mk-agent_" + currentVersion + "-1_all.deb"
	// Get absolute path of the file
	oldPath := folderPath + "/" + oldFilename
	// Convert to oldPath to absolute path
	oldPath, err := filepath.Abs(oldPath)
	if err != nil {
		return err
	}
	newPath := folderPath + "/check-mk-agent-latest.deb"
	// Check if symlink exists and link to the same file
	if _, err := os.Lstat(newPath); err == nil {
		// Check if the symlink is the same
		if link, err := os.Readlink(newPath); err == nil {
			if link == oldPath {
				return nil
			} else {
				// Remove the symlink
				err = os.Remove(newPath)
				if err != nil {
					return err
				}
			}
		}
	}
	// Create symlink
	err = os.Symlink(oldPath, newPath)
	if err != nil {
		return err
	}

	return nil
}

// CmkVersionChecker Ticker to check the current version of check_mk
// If the version is different from the current one, it will send a message to the channel
func CmkVersionChecker(ticker *time.Ticker, channel chan CmkVersionChanges) {
	log.Println("Start CmkVersionChecker")
	for {
		select {
		case <-ticker.C:
			// Create version url
			versionUrl := fmt.Sprintf(urlTemplate, cmkDomain, cmkSite, cmkVersionUrl)
			// Get the version from the API
			_, versionResp, err := GetUrl("json", versionUrl)

			if err != nil {
				channel <- CmkVersionChanges{
					Version:         "",
					ErrorString:     "Error getting current version of check_mk",
					TriggerDownload: false,
				}
			}
			// Convert []byte to CmkVersionResponse struct
			cmkVersion, err := GetCmkVersion(versionResp)
			if err != nil {
				channel <- CmkVersionChanges{
					Version:         "",
					ErrorString:     "Response from check_mk API is not valid",
					TriggerDownload: false,
				}
			}

			// Check if the version is the same in all folders
			for _, folder := range config.ConfigCmkGetter.Folders {
				// Check if the version is the same
				isSame, err := cmkVersion.IsSameVersion(folder)
				if err != nil {
					channel <- CmkVersionChanges{
						Version:         "",
						ErrorString:     "Error checking the current version of check_mk",
						TriggerDownload: false,
					}
				}
				// Set version to the CurrentVersion
				CurrentVersion = cmkVersion.CroppedVersion()
				// Create symlink
				err = CreateSymlink(folder, cmkVersion.CroppedVersion())
				if err != nil {
					channel <- CmkVersionChanges{
						Version:         "",
						ErrorString:     "Error creating symlink",
						TriggerDownload: false,
					}
				}
				// If the version is not the same, send a message to the channel
				if !isSame {
					channel <- CmkVersionChanges{
						Version:         cmkVersion.CroppedVersion(),
						ErrorString:     "",
						TriggerDownload: true,
						Folder:          folder,
					}
				}
			}
		}
	}
}

// DownloadCmk Download the check_mk version from the API
func (c *CmkVersionChanges) DownloadCmk(folderPath string) error {
	// Create folder if not exists
	err := os.MkdirAll(folderPath, 0755)
	if err != nil {
		return err
	}
	// Create the url
	downloadUrl := fmt.Sprintf(downloadUrlTemplate, cmkDomain, cmkSite)
	// Get the file from the API
	respHeader, file, err := GetUrl("file", downloadUrl)
	if err != nil {
		return err
	}
	// Filename from the Content-Disposition header
	contentDispHeader := respHeader.Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(contentDispHeader)
	if err != nil {
		return err
	}
	filename := params["filename"]

	// Create the file
	f, err := os.Create(fmt.Sprintf("%s/%s", folderPath, filename))
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	// Write the file
	_, err = f.Write(file)
	if err != nil {
		return err
	}

	return nil
}

// CmkVersionHandler Handle the version changes
func CmkVersionHandler(channel chan CmkVersionChanges) {
	log.Println("Start CmkVersionHandler")
	for {
		select {
		case versionChanges := <-channel:
			if versionChanges.TriggerDownload {
				// Log the version changes
				log.Printf("New version of check_mk: %s", versionChanges.Version)
				// Download the new version
				err := versionChanges.DownloadCmk(versionChanges.Folder)
				if err != nil {
					log.Println(err)
				}
				// Log the download
				log.Printf("Downloaded version: %s in folder %s", versionChanges.Version, versionChanges.Folder)
			}
			if versionChanges.ErrorString != "" {
				log.Println(versionChanges.ErrorString)
			}
		}
	}
}
