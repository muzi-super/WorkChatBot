package toolSDK

import (
	"github.com/ollama/ollama/api"
)

// ========== 配置结构体 ==========

// ChatMessageOption 聊天消息选项
type ChatMessageOption struct {
	SystemContent string          `json:"system_content"`
	UserContent   string          `json:"user_content"`
	Images        []api.ImageData `json:"images,omitempty"`
}

type ChatOptions func(*ChatMessageOption) error

func WithImages(images []api.ImageData) ChatOptions {
	return func(c *ChatMessageOption) error {
		c.Images = images
		return nil
	}
}
