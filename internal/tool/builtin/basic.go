package builtin

import (
	"agent-demo/internal/tool"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"time"
)

const maxOutputBytes = 32 << 10

func CurrentTime() tool.Tool {
	type args struct {
		Timezone string `json:"timezone"`
	}
	return tool.NewFuncTool("get_current_time",
		"获取当前日期和时间。可以指定 IANA 时区（如 Asia/Shanghai），不指定则使用本地时区。",
		`{"type":"object","properties":{"timezone":{"type":"string","description":"IANA 时区名，例如 Asia/Shanghai、America/New_York"}}}`,
		func(_ context.Context, a args) (string, error) {
			loc := time.Local
			if a.Timezone != "" {
				var err error
				if loc, err = time.LoadLocation(a.Timezone); err != nil {
					return "", fmt.Errorf("unknown timezone %q", a.Timezone)
				}
			}
			return time.Now().In(loc).Format("2006-01-02 15:04:05 MST Monday"), nil
		})
}

func Calculator() tool.Tool {
	type args struct {
		Expression string `json:"expression"`
	}
	return tool.NewFuncTool("calculator",
		"计算数学表达式，支持 + - * / % ^ 和括号。大模型心算不可靠，任何数值计算都应使用本工具。",
		`{"type":"object","properties":{"expression":{"type":"string","description":"数学表达式，例如 (1+2)*3^2"}},"required":["expression"]}`,
		func(_ context.Context, a args) (string, error) {
			v, err := Eval(a.Expression)
			if err != nil {
				return "", err
			}
			return strconv.FormatFloat(v, 'g', -1, 64), nil
		})
}

// HTTPGet 请求任意 URL。生产环境必须防范 SSRF：模型可能被诱导去访问内网地址。
func HTTPGet() tool.Tool {
	type args struct {
		URL string `json:"url"`
	}
	client := &http.Client{Timeout: 15 * time.Second}
	return tool.NewFuncTool("http_get",
		"发送 HTTP GET 请求并返回状态码和响应内容（最多 32KB）。适合调用公开 API 或获取网页原文。",
		`{"type":"object","properties":{"url":{"type":"string","description":"完整的 http 或 https 地址"}},"required":["url"]}`,
		func(ctx context.Context, a args) (string, error) {
			u, err := url.Parse(a.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return "", fmt.Errorf("invalid url %q: only http and https are supported", a.URL)
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
			if err != nil {
				return "", err
			}
			req.Header.Set("User-Agent", "go-agent/0.1")

			resp, err := client.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, maxOutputBytes+1))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("HTTP %s\nContent-Type: %s\n\n%s",
				resp.Status, resp.Header.Get("Content-Type"), tool.Truncate(string(body), maxOutputBytes)), nil
		})
}

// RunCommand 在工作目录中执行 shell 命令。os.Root 限制不了 shell，
// 命令可以访问整台机器，因此这个工具必须经过人工确认。
func RunCommand(dir string) tool.Tool {
	type args struct {
		Command string `json:"command"`
	}
	return tool.NewFuncTool("run_command",
		"在工作目录中执行 shell 命令，返回合并后的 stdout 和 stderr。每次执行都需要用户确认。",
		`{"type":"object","properties":{"command":{"type":"string","description":"要执行的 shell 命令"}},"required":["command"]}`,
		func(ctx context.Context, a args) (string, error) {
			cmd := exec.CommandContext(ctx, "sh", "-c", a.Command)
			cmd.Dir = dir
			// 超时杀掉 sh 后，它启动的子进程可能仍占用输出管道，WaitDelay 防止一直阻塞
			cmd.WaitDelay = time.Second
			out, err := cmd.CombinedOutput()
			result := tool.Truncate(string(out), maxOutputBytes)
			if err != nil {
				// 命令失败也把输出交给模型，它通常能根据报错信息自行修正
				return fmt.Sprintf("%s\n[command failed: %v]", result, err), nil
			}
			return result, nil
		}).WithApproval()
}
