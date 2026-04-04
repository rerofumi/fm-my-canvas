package main

import (
	"embed"

	"fm-my-canvas/artifacts"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	artifactMgr, err := artifacts.NewManager()
	if err != nil {
		println("Failed to initialize artifact manager:", err.Error())
		return
	}

	server := artifacts.NewServer()

	chatService, err := NewChatService(artifactMgr, server)
	if err != nil {
		println("Failed to initialize chat service:", err.Error())
		return
	}
	app.SetChatService(chatService)

	err = wails.Run(&options.App{
		Title:  "fm-my-canvas",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			chatService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
