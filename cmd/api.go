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

	// Create /api endpoint
	api := r.Group("/api")

	// Serve index.html on all other routes
	r.NoRoute(func(c *gin.Context) {
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
	api.GET("/cmk-files", func(context *gin.Context) {
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

	// API endpoint to trigger deploy plugin to node
	api.POST("/deploy-plugin", func(context *gin.Context) {
		// Get node name from request
		nodeName := context.PostForm("node")
		pluginName := context.PostForm("plugin")
		// Get node from map
		node, ok := utils.CheckMkNodeMap.Nodes[nodeName]
		if !ok {
			context.JSON(404, gin.H{
				"error": "Node not found",
			})
			return
		}
		// Deploy plugin to node via SendPlugin
		err := node.SendPlugin(utils.CheckMkPlugin{
			Name: pluginName,
		})
		if err != nil {
			context.JSON(500, gin.H{
				"error": err,
			})
			return
		}

		// Send update plugin trigger to channel
		utils.PluginCheckerTrigger <- true

		context.JSON(200, gin.H{
			"message": "Plugin deployed",
		})
	})

	// JSON with ssh nodes
	api.GET("/ssh-nodes", func(context *gin.Context) {
		// Get nodes from CMK API
		context.JSON(200, utils.CheckMkNodeMap.Nodes)
	})

	// Start server
	defer func() {
		err := r.Run(fmt.Sprintf("%s:%d", config.ConfigCmkGetter.Listen, config.ConfigCmkGetter.Port))
		if err != nil {
			panic(err)
		}
	}()
}
