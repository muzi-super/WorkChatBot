package getEnv

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// envOptions 用 tag 指定环境变量名；required 表示必填
type envOptions struct {
	WEComBotID         string          `env:"WE_COM_BOT_ID,required"`
	WEComBotSecret     string          `env:"WE_COM_BOT_SECRET,required"`
	OllamaURL          url.URL         `env:"OLLAMA_URL,required"`
	OllamaTimeout      time.Duration   `env:"OLLAMA_TIMEOUT,required"`
	ModelOptions       map[string]any  `env:"MODEL_OPTIONS"`
	Formats            json.RawMessage `env:"FORMATS"`
	KeepAlive          time.Duration   `env:"KEEP_ALIVE,required"`
	TextModelName      string          `env:"TEXT_MODEL_NAME,required"`
	ImageModelName     string          `env:"IMAGE_MODEL_NAME,required"`
	Think              bool            `env:"THINK"`
	WorkerCount        int             `env:"WORKER_COUNT"`
	LogLevel           string          `env:"LOG_LEVEL"`
	LogFile            string          `env:"LOG_FILE"`
	LogFormat          string          `env:"LOG_FORMAT"`
	LogRotate          string          `env:"LOG_ROTATE"`
	ModelSystemContent string          `env:"MODEL_SYSTEM_CONTENT"`
	ChatJobCount       int             `env:"CHAT_JOB_COUNT"`
}

// envOptionsParser 为 map[string]any / json.RawMessage 提供自定义解析
var envOptionsParser = env.Options{
	FuncMap: map[reflect.Type]env.ParserFunc{
		reflect.TypeOf(map[string]any{}): func(value string) (any, error) {
			var m map[string]any
			if err := json.Unmarshal([]byte(value), &m); err != nil {
				return nil, err
			}
			return m, nil
		},
		reflect.TypeOf(json.RawMessage{}): func(value string) (any, error) {
			if !json.Valid([]byte(value)) {
				return nil, fmt.Errorf("非法 JSON: %s", value)
			}
			return json.RawMessage(value), nil
		},
	},
}

// getOption 加载 .env 并按类型解析环境变量
func getOption() (*envOptions, error) {
	if err := godotenv.Load(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "警告: 未找到 .env:", err)
	}
	cfg, err := env.ParseAsWithOptions[envOptions](envOptionsParser)
	if err != nil {
		return nil, err
	}
	if cfg.OllamaTimeout == 0 {
		return nil, fmt.Errorf("OLLAMA_TIMEOUT 不能为 0")
	}

	return &cfg, nil
}

// EnvOpt 环境变量选项，包初始化时解析；对外只读访问
var EnvOpt, envError = getOption()

func init() {
	if envError != nil {
		_, _ = fmt.Fprintln(os.Stderr, "获取环境变量失败:", envError)
		os.Exit(1)
	}
}
