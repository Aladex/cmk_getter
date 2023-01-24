package main

import (
	assets "cmk_getter"
	"cmk_getter/config"
	"cmk_getter/utils"
	"io/fs"
	"net/http"
	"time"
)

type File struct {
	Name string `json:"name"`
	MD5  string `json:"md5"`
	Date string `json:"date"`
	SIze int64  `json:"size"`
}

type Folder struct {
	Name  string `json:"name"`
	Files []File `json:"files"`
}

type FoldersResponse struct {
	Folders []Folder `json:"folders"`
	Version string   `json:"version"`
}

func Run() {
	// Create channel for goroutines
	channel := make(chan utils.CmkVersionChanges)

	// Create ticker
	// Create duration from config
	duration := time.Duration(config.ConfigCmkGetter.Polling) * time.Second
	ticker := time.NewTicker(duration)

	// Run goroutines
	go utils.CmkVersionChecker(ticker, channel)
	go utils.CmkVersionHandler(channel)
	// go utils.PluginCheckerTicker()
}

func mustFS() http.FileSystem {
	sub, err := fs.Sub(assets.Assets, "dist/assets")

	if err != nil {
		panic(err)
	}

	return http.FS(sub)
}

func main() {
	Run()
	// Run API
	RunAPI()
}
