package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"WorkChatBot/getEnv"
	"WorkChatBot/logTool"
	"WorkChatBot/toolSDK"

	"github.com/go-sphere/wecom-aibot-go-sdk/aibot"
)

type chatJob struct {
	frame   *aibot.WsFrame
	content string
}

func main() {
	logTool.Init() // 加载日志配置（须在用日志前调用）
	logTool.Info("Starting WorkChatBot...")

	client := aibot.NewWSClient(aibot.WSClientOptions{
		BotID:  getEnv.EnvOpt.WEComBotID,
		Secret: getEnv.EnvOpt.WEComBotSecret,
	})

	jobs := make(chan chatJob, getEnv.EnvOpt.ChatJobCount) // 有界队列，大小为 ChatJobCount

	// worker pool：N 个协程并发
	var workerCount = getEnv.EnvOpt.WorkerCount
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				handleJob(client, job)
			}
		}()
	}

	// 回调只投递，不阻塞 SDK 分发协程
	client.OnMessageText(func(frame *aibot.WsFrame) {
		var msg aibot.TextMessage
		if err := aibot.ParseMessageBody(frame, &msg); err != nil {
			logTool.Warn("解析消息失败: %v", err)
			return
		}
		logTool.Info("收到: %s", msg.Text.Content)
		select {
		case jobs <- chatJob{frame: frame, content: msg.Text.Content}:
		default:
			_, _ = client.ReplyStream(frame, "stream_"+frame.Headers.ReqID, "当前繁忙，请稍后重试", true, nil, nil)
			logTool.Warn("任务队列已满，丢弃消息")
		}
	})

	client.OnEventEnterChat(func(frame *aibot.WsFrame) {
		_, _ = client.ReplyWelcome(frame, aibot.CreateTextReplyBody("您好，我是智能助手"))
	})

	client.OnConnected(func() { logTool.Info("连接已建立") })
	client.OnAuthenticated(func() { logTool.Info("认证成功") })
	client.OnDisconnected(func(r string) { logTool.Info("断开: %s", r) })

	client.Connect() // 内置自动认证 + 心跳 + 断线重连

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logTool.Info("收到退出信号，正在关闭...")

	close(jobs) // 停止投递
	wg.Wait()   // 等任务完成
	client.Disconnect()
	logTool.Info("程序已退出")
}

func handleJob(client *aibot.WSClient, job chatJob) {
	defer func() {
		if r := recover(); r != nil {
			logTool.Error("任务异常: %v", r)
			_, _ = client.ReplyStream(job.frame, "stream_"+job.frame.Headers.ReqID, "处理异常", true, nil, nil)
		}
	}()
	streamID := "stream_" + job.frame.Headers.ReqID
	answer := toolSDK.TextChat(job.content) // 非流式，等完整结果
	if _, err := client.ReplyStream(job.frame, streamID, answer, true, nil, nil); err != nil {
		logTool.Error("回复失败: %v", err)
	}
}
