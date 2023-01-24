package utils

import (
	"bufio"
	"bytes"
	"cmk_getter/config"
	"crypto/md5"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"log"
	"os"
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
		log.Println("Error getting plugin from API")
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
	// Create the ssh client
	sshClient, err := node.CreateSshClient()
	if err != nil {
		log.Println("Error creating ssh client:", err)
		return err
	}
	defer func() {
		err := sshClient.Close()
		if err != nil {
			log.Println("Error closing ssh client:", err)
		}
	}()
	// Create the sftp client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		log.Println("Error creating sftp client:", err)
		return err
	}
	defer func() {
		err := sftpClient.Close()
		if err != nil {
			log.Println("Error closing sftp client:", err)
		}
	}()
	// Find the plugin file on the node
	pluginFile, err := sftpClient.Open(fmt.Sprintf("%s/%s", node.GetPluginFolder(), c.Name))
	if err != nil {
		log.Println("Error opening plugin file:", err)
		// Create the plugin file
		pluginFile, err = sftpClient.Create(fmt.Sprintf("%s/%s", node.GetPluginFolder(), c.Name))
	}
	// Convert *File object to []byte with reader and buffer
	reader := bufio.NewReader(pluginFile)
	buffer := bytes.NewBuffer(make([]byte, 0))
	_, err = buffer.ReadFrom(reader)
	if err != nil {
		log.Println("Error reading plugin file:", err)
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
			log.Println("Error removing plugin file:", err)
			return err
		}
		// Create the plugin file on the node
		pluginFile, err := sftpClient.Create(fmt.Sprintf("%s/%s", node.GetPluginFolder(), c.Name))
		if err != nil {
			log.Println("Error creating plugin file:", err)
			return err
		}
		// Set 755 permissions
		err = pluginFile.Chmod(0755)
		if err != nil {
			log.Println("Error setting permissions:", err)
			return err
		}
		// Write the plugin content to the plugin file
		_, err = pluginFile.Write(c.ByteContent)
		if err != nil {
			log.Println("Error writing plugin file:", err)
			return err
		}
		err = pluginFile.Close()
		if err != nil {
			log.Println("Error closing plugin file:", err)
			return err
		}
		log.Println("Plugin", c.Name, "sent to", node.Host)
		return nil
	}
	log.Println("Plugin", c.Name, "is actual on", node.Host)

	return nil
}

// PluginChecker Check the plugins on the nodes and send the plugins if the md5 hash is different
//func PluginChecker() {
//	// Get the plugins from config
//	plugins := config.ConfigCmkGetter.Plugins
//	nodes, err := GetNodesList()
//	if err != nil {
//		log.Println("Error getting nodes list:", err)
//		panic(err)
//	}
//
//	// Iterate over the plugins
//	for _, plugin := range plugins {
//		log.Println("Checking plugin", plugin)
//		// Get the plugin from the API
//		cmkPlugin := CheckMkPlugin{Name: plugin}
//		err := GetPlugin(&cmkPlugin)
//		if err != nil {
//			log.Println("Error getting plugin from API:", err)
//			continue
//		}
//		log.Println("Plugin", plugin, "got from API")
//		// Iterate over the nodes
//		for _, node := range nodes {
//			log.Println("Checking node", node.Host)
//			// Send the plugin to the node
//			err := node.SendPlugin(cmkPlugin)
//			if err != nil {
//				log.Println("Error sending plugin to node:", err)
//				continue
//			}
//			log.Println("Plugin", plugin, "checked on node", node.Host)
//		}
//	}
//}

// PluginCheckerTicker Create ticker for the plugin checker
//func PluginCheckerTicker() {
//	// Create ticker for the plugin checker
//	log.Println("Plugin checker started")
//	ticker := time.NewTicker(time.Duration(config.ConfigCmkGetter.Polling) * time.Second)
//	go func() {
//		for range ticker.C {
//			PluginChecker()
//		}
//	}()
//}
