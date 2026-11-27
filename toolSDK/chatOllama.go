package toolSDK

import (
	"WorkChatBot/getEnv"
	"WorkChatBot/logTool"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ollama/ollama/api"
)

// clientCache 用于缓存客户端
var clientCache sync.Map

// getClient 从缓存获取客户端如果存在, 不存在则新建一个客户端并保存到缓存
func getClient() *api.Client {
	// 从环境变量获取 Ollama URL
	urlEnv := &getEnv.EnvOpt.OllamaURL
	// 转为 URL
	var urlStr = urlEnv.String()

	// 从缓存获取客户端如果存在, 不存在则新建一个客户端并保存到缓存
	if v, ok := clientCache.Load(urlStr); ok {
		return v.(*api.Client)
	}

	// 新建一个客户端
	clientNew := api.NewClient(urlEnv, &http.Client{})
	clientCache.Store(urlStr, clientNew)

	return clientNew
}

// responseChat 复用 client（getClient 缓存），带 ctx 超时，调用并记录耗时。
func responseChat(ctx context.Context, messages []api.Message) (string, error) {
	keepAlive := getEnv.EnvOpt.KeepAlive
	kaDur := api.Duration{Duration: keepAlive}

	req := &api.ChatRequest{
		Model:     getEnv.EnvOpt.TextModelName,
		Messages:  messages,
		Options:   getEnv.EnvOpt.ModelOptions,
		Format:    getEnv.EnvOpt.Formats, // 直接透传，不再拼接
		KeepAlive: &kaDur,
		Stream:    new(bool),
		Think:     &api.ThinkValue{Value: getEnv.EnvOpt.Think},
	}

	// 调用
	var finalResp api.ChatResponse
	err := getClient().Chat(ctx, req, func(resp api.ChatResponse) error {
		finalResp = resp
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("调用 Ollama 失败: %w", err)
	}

	// 耗时（纳秒 → 毫秒）
	m := finalResp.Metrics
	loadMs := float64(finalResp.LoadDuration) / float64(time.Millisecond)
	promptEvalMs := float64(m.PromptEvalDuration) / float64(time.Millisecond)
	genMs := float64(m.EvalDuration) / float64(time.Millisecond)
	totalMs := float64(finalResp.TotalDuration) / float64(time.Millisecond)
	promptEvalCount := m.PromptEvalCount
	genCount := m.EvalCount

	if promptEvalCount > 0 {
		perTokMs := promptEvalMs / float64(promptEvalCount)
		logTool.Info("耗时 | 总耗时=%.1fms 模型加载=%.1fms 提示词计算=%.1fms(提示词token数=%d, 每token=%.3fms) 输出生成=%.1fms(输出token数=%d)",
			totalMs, loadMs, promptEvalMs, promptEvalCount, perTokMs, genMs, genCount)
	} else {
		logTool.Info("耗时 | 总耗时=%.1fms 模型加载=%.1fms 输出生成=%.1fms", totalMs, loadMs, genMs)
	}

	if finalResp.Message.Thinking != "" {
		logTool.Debug("thinking: %s", finalResp.Message.Thinking)
	}
	logTool.Debug("content: %s", finalResp.Message.Content)

	return finalResp.Message.Content, nil
}

func TextChat(testMsg string) string {
	logTool.Info("TextChat...")

	resp, resErr := responseChat(context.Background(), []api.Message{
		{Role: "system", Content: getEnv.EnvOpt.ModelSystemContent},
		{Role: "user", Content: testMsg},
	})
	if resErr != nil {
		logTool.Error("TextChat error: %v", resErr)
		return resErr.Error()
	}

	return resp
}
