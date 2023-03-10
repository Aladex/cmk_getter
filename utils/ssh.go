package utils

import (
	"bufio"
	"bytes"
	"cmk_getter/config"
	"cmk_getter/log"
	"crypto/md5"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"os"
	"sync"
	"time"
)

type CheckMkPlugin struct {
	Name        string `json:"name"`
	IsActual    bool   `json:"is_actual"`
	Url         string `json:",omitempty"`
	ByteContent []byte `json:",omitempty"`
}

type CheckMkNode struct {
	Host         string          `json:"host"`
	Port         string          `json:",omitempty"`
	PluginFolder string          `json:",omitempty"`
	Plugins      []CheckMkPlugin `json:"plugins"`
	// SSH is available only for the cmk_getter
	IsAvailable bool `json:"is_available"`
}

// PluginCheckerTrigger Channel for trigger for plugins check
var PluginCheckerTrigger = make(chan bool)

// GetPluginFolder Return default plugin folder
func (node CheckMkNode) GetPluginFolder() string {
	if node.PluginFolder == "" {
		return "/usr/lib/check_mk_agent/plugins"
	}
	return node.PluginFolder
}

// GetPort Return default port
func (node CheckMkNode) GetPort() string {
	if node.Port == "" {
		return "22"
	}
	return node.Port
}

// PluginUrlTemplate URL template for getting a plugin from the API
const PluginUrlTemplate = "https://%s/%s/check_mk/agents/plugins/%s"

func (c CheckMkPlugin) CreateUrl() string {
	return fmt.Sprintf(PluginUrlTemplate, config.ConfigCmkGetter.Domain, config.ConfigCmkGetter.Site, c.Name)
}

func GetPlugin(c *CheckMkPlugin) error {
	// Get the plugin from the API as []byte
	_, pluginResp, err := GetUrl("json", c.CreateUrl())
	if err != nil {
		log.Logger.Info("Error getting plugin from API")
		return err
	}
	// Set the byte content
	c.ByteContent = pluginResp
	return nil
}

// CalculateMd5 Calculate the md5 hash of the plugin
func (c CheckMkPlugin) CalculateMd5() string {
	// Calculate the md5 hash of the plugin content
	hashSum := md5.Sum(c.ByteContent)
	// Encode the hash to a string
	return fmt.Sprintf("%x", hashSum)
}

// ReadRSAKey Read the RSA key from .ssh folder
func ReadRSAKey() (ssh.Signer, error) {
	// Read the RSA key from .ssh folder
	key, err := os.ReadFile(config.ConfigCmkGetter.PathToIdRSA)
	if err != nil {
		return nil, err
	}
	// Parse the RSA key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

// CheckSsh Check if ssh is available
func (node CheckMkNode) CheckSsh() bool {
	// Create the ssh client with golang.org/x/crypto/ssh and ssh.Signer
	sshClient, err := node.CreateSshClient()
	if err != nil {
		return false
	}
	// Close the ssh client
	err = sshClient.Close()
	if err != nil {
		return false
	}
	return true
}

// CreateSshClient Create the ssh client with golang.org/x/crypto/ssh and ssh.Signer
func (node CheckMkNode) CreateSshClient() (*ssh.Client, error) {
	signer, err := ReadRSAKey()
	if err != nil {
		return nil, err
	}
	// Create the ssh client with golang.org/x/crypto/ssh and ssh.Signer
	sshConfig := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		Timeout: 5 * time.Second,
	}
	// Connect to the node
	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", node.Host, node.GetPort()), sshConfig)
	if err != nil {
		return nil, err
	}
	return sshClient, nil
}

// SendPlugin Send the plugin to the node with ssh if the md5 hash is different
func (node CheckMkNode) SendPlugin(c CheckMkPlugin) error {
	// Get the plugin from the API as []byte
	err := GetPlugin(&c)
	if err != nil {
		log.Logger.Debugln("Error getting plugin from API")
		return err
	}
	// Create the ssh client
	sshClient, err := node.CreateSshClient()
	if err != nil {
		log.Logger.Debugln("Error creating ssh client:", err)
		return err
	}
	defer func() {
		err := sshClient.Close()
		if err != nil {
			log.Logger.Traceln("Error closing ssh client:", err)
		}
	}()
	// Create the sftp client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		log.Logger.Debugln("Error creating sftp client:", err)
		return err
	}
	defer func() {
		err := sftpClient.Close()
		if err != nil {
			log.Logger.Debugln("Error closing sftp client:", err)
		}
	}()
	// Find the plugin file on the node
	pluginFile, err := sftpClient.Open(fmt.Sprintf("%s/%s", node.GetPluginFolder(), c.Name))
	if err != nil {
		log.Logger.Debugln("Error opening plugin file:", err)
		// Create the plugin file
		pluginFile, err = sftpClient.Create(fmt.Sprintf("%s/%s", node.GetPluginFolder(), c.Name))
	}
	// Convert *File object to []byte with reader and buffer
	reader := bufio.NewReader(pluginFile)
	buffer := bytes.NewBuffer(make([]byte, 0))
	_, err = buffer.ReadFrom(reader)
	if err != nil {
		log.Logger.Debugln("Error reading plugin file:", err)
		return err
	}
	// Calculate the md5 hash of the plugin file on the node
	hashSum := md5.Sum(buffer.Bytes())
	// Encode the hash to a string
	md5HashOnNode := fmt.Sprintf("%x", hashSum)
	// Check if the md5 hash of the plugin file on the node is different
	if md5HashOnNode != c.CalculateMd5() {
		// Remove the plugin file on the node
		err = sftpClient.Remove(fmt.Sprintf("%s/%s", node.GetPluginFolder(), c.Name))
		if err != nil {
			log.Logger.Debugln("Error removing plugin file:", err)
			return err
		}
		// Create the plugin file on the node
		pluginFile, err := sftpClient.Create(fmt.Sprintf("%s/%s", node.GetPluginFolder(), c.Name))
		if err != nil {
			log.Logger.Debugln("Error creating plugin file:", err)
			return err
		}
		// Set 755 permissions
		err = pluginFile.Chmod(0755)
		if err != nil {
			log.Logger.Debugln("Error setting permissions:", err)
			return err
		}
		// Write the plugin content to the plugin file
		_, err = pluginFile.Write(c.ByteContent)
		if err != nil {
			log.Logger.Debugln("Error writing plugin file:", err)
			return err
		}
		err = pluginFile.Close()
		if err != nil {
			log.Logger.Debugln("Error closing plugin file:", err)
			return err
		}
		log.Logger.Debugln("Plugin", c.Name, "sent to", node.Host)
		return nil
	}
	log.Logger.Debugln("Plugin", c.Name, "is actual on", node.Host)

	return nil
}

// CheckPluginsBySSH Check the plugins on the nodes and set the status is actual or not
func CheckPluginsBySSH(node CheckMkNode) (CheckMkNode, error) {
	// Create the ssh client
	sshClient, err := node.CreateSshClient()
	if err != nil {
		log.Logger.Debugln("Error creating ssh client:", err)
		return CheckMkNode{}, err
	}
	defer func() {
		err := sshClient.Close()
		if err != nil {
			log.Logger.Debugln("Error closing ssh client:", err)
		}
	}()
	// Create the sftp client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		log.Logger.Debugln("Error creating sftp client:", err)
		return CheckMkNode{}, err
	}
	defer func() {
		err := sftpClient.Close()
		if err != nil {
			log.Logger.Debugln("Error closing sftp client:", err)
		}
	}()
	// Iterate over the plugins
	for _, plugin := range node.Plugins {
		err := GetPlugin(&plugin)
		if err != nil {
			log.Logger.Debugln("Error getting plugin:", err)
			plugin.IsActual = false
			continue
		}
		// Find the plugin file on the node
		pluginFile, err := sftpClient.Open(fmt.Sprintf("%s/%s", node.GetPluginFolder(), plugin.Name))
		if err != nil {
			log.Logger.Debugln("Error opening plugin file:", err)
			plugin.IsActual = false
			continue
		}
		// Convert *File object to []byte with reader and buffer
		reader := bufio.NewReader(pluginFile)
		buffer := bytes.NewBuffer(make([]byte, 0))
		_, err = buffer.ReadFrom(reader)
		if err != nil {
			log.Logger.Debugln("Error reading plugin file:", err)
			plugin.IsActual = false
			continue
		}
		// Calculate the md5 hash of the plugin file on the node
		hashSum := md5.Sum(buffer.Bytes())
		// Encode the hash to a string
		md5HashOnNode := fmt.Sprintf("%x", hashSum)
		// Check if the md5 hash of the plugin file on the node is different
		if md5HashOnNode != plugin.CalculateMd5() {
			plugin.IsActual = false
			log.Logger.Debugln("Plugin", plugin.Name, "is not actual on", node.Host)
			continue
		}
		plugin.IsActual = true
		// Change plugin in the node plugins list
		for i, nodePlugin := range node.Plugins {
			if nodePlugin.Name == plugin.Name {
				node.Plugins[i] = plugin
			}
		}
	}
	return node, nil
}

// PluginChecker Check the plugins on the nodes and set the status is actual or not
func PluginChecker() {
	// Create wait group
	var wg sync.WaitGroup
	// Defer wait group
	defer wg.Wait()

	// Iterate over the nodes
	for _, node := range CheckMkNodeMap.Nodes {
		// Check if the node is available
		if !node.IsAvailable {
			continue
		}
		log.Logger.Debugln("Check plugins on", node.Host)
		// Add 1 to wait group
		wg.Add(1)
		// Run the check plugins by ssh in goroutine
		go func(node CheckMkNode) {
			// Defer wait group done
			defer wg.Done()
			// Check the plugins on the node
			_, err := CheckPluginsBySSH(node)
			if err != nil {
				log.Logger.Debugln("Error checking plugins by ssh:", err)
				return
			}
			// Update the node in the map
			// Lock the map
			CheckMkNodeMap.Mutex.Lock()
			// Defer unlock the map
			defer CheckMkNodeMap.Mutex.Unlock()
			// Update the node in the map
			CheckMkNodeMap.Nodes[node.Host] = node
		}(node)
	}
}

// CheckPlugins Listen channel for trigger the plugin checker
func CheckPlugins() {
	for {
		// Wait for the trigger
		<-PluginCheckerTrigger
		// Log info that the plugin checker is running
		log.Logger.Infoln("Run plugin checker")
		// Run the plugin checker
		PluginChecker()
	}
}

func PluginCheckerTicker() {
	// Send the trigger to the channel every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	for {
		<-ticker.C
		PluginCheckerTrigger <- true
	}
}
