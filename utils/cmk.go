package utils

import (
	"cmk_getter/config"
	"cmk_getter/log"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func init() {
	// Get the current version of check_mk
	cmkVersionUrl := fmt.Sprintf(urlTemplate, config.ConfigCmkGetter.Domain, config.ConfigCmkGetter.Site, cmkVersionUrl)
	_, response, err := GetUrl("json", cmkVersionUrl)
	if err != nil {
		log.Logger.Fatalln(err)
	}
	cmkVersion, err := GetCmkVersion(response)
	if err != nil {
		log.Logger.Fatalln(err)
	}
	CurrentVersion = cmkVersion.CroppedVersion()
}

// CheckMkNodeMap Global struct CmkNodeMap for get and update nodes with mutex
var CheckMkNodeMap = &CmkNodeMap{
	Nodes: make(map[string]CheckMkNode),
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
	log.Logger.Infoln("Start CmkVersionChecker")
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
	downloadUrl := fmt.Sprintf(urlTemplate, cmkDomain, cmkSite, downloadUrlTemplate)
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
	log.Logger.Infoln("Start CmkVersionHandler")
	for {
		select {
		case versionChanges := <-channel:
			if versionChanges.TriggerDownload {
				// Log the version changes
				log.Logger.Infoln("New version of check_mk: %s", versionChanges.Version)
				// Download the new version
				err := versionChanges.DownloadCmk(versionChanges.Folder)
				if err != nil {
					log.Logger.Infoln(err)
				}
				// Log the download
				log.Logger.Infoln("Downloaded version: %s in folder %s", versionChanges.Version, versionChanges.Folder)
			}
			if versionChanges.ErrorString != "" {
				log.Logger.Infoln(versionChanges.ErrorString)
			}
		}
	}
}

// GenerateDefaultPlugins Generate default plugins list from config for CheckMkNode
func GenerateDefaultPlugins(c *CheckMkNode) {
	for _, plugin := range config.ConfigCmkGetter.Plugins {
		c.Plugins = append(c.Plugins, CheckMkPlugin{
			Name:     plugin,
			IsActual: false,
		})
	}
}

// GetNodesList get the list of nodes from the API with tag_check_mk-agent-conn = ssh
func GetNodesList() error {
	// Create the url
	nodesUrl := fmt.Sprintf(urlTemplate, cmkDomain, cmkSite, hostConfigUrl)
	// Get the nodes from the API
	_, nodesResp, err := GetUrl("json", nodesUrl)
	if err != nil {
		return err
	}
	// Convert []byte to CmkNodesResponse struct with json.Unmarshal
	var cmkNodeResp CmkHostConfigResponse

	err = json.Unmarshal(nodesResp, &cmkNodeResp)
	if err != nil {
		return err
	}
	// Iterate over the nodes
	for _, node := range cmkNodeResp.Value {
		// Check if the node has the tag_check_mk-agent-conn = ssh

		if node.Extensions.Attributes.TagCheckMkAgentConn == "ssh" {
			// Create a new CheckMkNode
			cmkNode := CheckMkNode{
				Host: node.Id,
			}
			// Generate the default plugins list
			GenerateDefaultPlugins(&cmkNode)
			// Add the node to the map if node is not in the map
			if _, ok := CheckMkNodeMap.Nodes[node.Id]; !ok {
				CheckMkNodeMap.Nodes[node.Id] = cmkNode
			}
		}
	}
	return nil
}

// Ticker Get the nodes list every 5 minutes
func GetNodesTicker() {
	for {
		err := GetNodesList()
		if err != nil {
			log.Logger.Infoln(err)
		}
		time.Sleep(5 * time.Minute)
	}
}

func SSHStatusUpdater() {
	for {
		// Check if the map is not empty
		if len(CheckMkNodeMap.Nodes) > 0 {
			// Create waitgroup for the goroutines
			var wg sync.WaitGroup
			for _, node := range CheckMkNodeMap.Nodes {
				// Add 1 to the waitgroup
				wg.Add(1)
				// Start the goroutine
				go func(node CheckMkNode) {
					// Defer the waitgroup Done
					defer wg.Done()
					// Get ssh status
					sshStatus := node.CheckSsh()
					if sshStatus != node.IsAvailable {
						// Lock the map
						CheckMkNodeMap.Mutex.Lock()
						defer CheckMkNodeMap.Mutex.Unlock()
						// Update the node
						node.IsAvailable = sshStatus
						CheckMkNodeMap.Nodes[node.Host] = node
					}
				}(node)
			}
			// Wait for the goroutines to finish and send true to the channel PluginCheckerTrigger and sleep 20 seconds
			wg.Wait()
			PluginCheckerTrigger <- true
			time.Sleep(20 * time.Second)
		}
		// Sleep 2 seconds
		time.Sleep(2 * time.Second)
	}
}
