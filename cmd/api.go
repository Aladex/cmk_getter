package main

import (
	assets "cmk_getter"
	"cmk_getter/config"
	"cmk_getter/utils"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"net/http"
)

func RunAPI() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	err := r.SetTrustedProxies(nil)
	// Allow all CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	r.Use(cors.New(corsConfig))

	if err != nil {
		panic(err)
	}
	//
	// Set logger config
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s %s %s\n",
			param.TimeStamp.Format("2006/01/02 15:04:05"),
			param.Path,
			param.ClientIP,
		)
	}))

	// Serve static files
	// example: /public/assets/images/*
	r.StaticFS("/assets", mustFS())

	r.GET("/", func(c *gin.Context) {
		file, _ := assets.Assets.ReadFile("dist/index.html")
		c.Data(
			http.StatusOK,
			"text/html; charset=utf-8",
			file,
		)
	})

	r.GET("favicon.ico", func(c *gin.Context) {
		file, _ := assets.Assets.ReadFile("dist/favicon.ico")
		c.Data(
			http.StatusOK,
			"image/x-icon",
			file,
		)
	})

	// JSON endpoint with folders and files saved from Check_MK
	r.GET("/cmk-files", func(context *gin.Context) {
		// Get files from folders and return JSON
		FoldersResp := FoldersResponse{
			Version: utils.CurrentVersion,
		}
		folders := config.ConfigCmkGetter.Folders

		for _, folder := range folders {
			files, err := utils.GetFiles(folder)
			if err != nil {
				context.JSON(500, gin.H{
					"error": err,
				})
				return
			}
			folderFiles := []File{}
			for _, file := range files {
				md5, err := utils.GetMD5(folder + "/" + file)
				if err != nil {
					context.JSON(500, gin.H{
						"error": err,
					})
					return
				}
				size, err := utils.GetFileSize(folder + "/" + file)
				if err != nil {
					context.JSON(500, gin.H{
						"error": err,
					})
					return
				}
				folderFiles = append(folderFiles, File{
					Name: file,
					MD5:  md5,
					Date: utils.GetDate(folder + "/" + file),
					SIze: size,
				})
			}
			FoldersResp.Folders = append(FoldersResp.Folders, Folder{Name: folder, Files: folderFiles})
		}

		context.JSON(200, FoldersResp)
	})

	// JSON with ssh nodes
	r.GET("/ssh-nodes", func(context *gin.Context) {
		// Get nodes from CMK API
		nodes, err := utils.GetNodesList()
		if err != nil {
			context.JSON(500, gin.H{
				"error": err,
			})
			return
		}
		context.JSON(200, nodes)
	})

	// Start server
	defer func() {
		err := r.Run(fmt.Sprintf("%s:%d", config.ConfigCmkGetter.Listen, config.ConfigCmkGetter.Port))
		if err != nil {
			panic(err)
		}
	}()
}
