package main

import (
	"context"
)

type App struct {
	ctx         context.Context
	chatService *ChatService
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.chatService != nil {
		a.chatService.SetContext(ctx)
	}
}

func (a *App) SetChatService(cs *ChatService) {
	a.chatService = cs
	if a.ctx != nil {
		cs.SetContext(a.ctx)
	}
}
