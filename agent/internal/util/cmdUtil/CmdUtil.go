package cmdUtil

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/jhUtil/simple-agent-go/internal/util/errorUtil"
)

// CrossPlatformExec 跨平台执行命令 (同步执行 全部执行完在整体响应)
func CrossPlatformExec(command string) (string, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("zsh", "-c", command)
	case "linux":
		cmd = exec.Command("bash", "-c", command)
	case "windows":
		cmd = exec.Command("cmd.exe", "/C", command)
	default:
		return "", errorUtil.NewF("未适配该平台: %s", runtime.GOOS)
	}

	// 捕获输出
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// CommandExecutor 跨平台命令执行器
// type CommandExecutor struct{}

// NewCommandExecutor 创建新的命令执行器
// func NewCommandExecutor() *CommandExecutor {
// 	return &CommandExecutor{}
// }

// StreamCommand 流式执行命令并将输出发送到通道
func StreamCommand(ctx context.Context, command string, output chan<- string) error {
	defer close(output)

	// 确定平台和对应的shell
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.CommandContext(ctx, "zsh", "-c", command)
	case "linux":
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", command)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// 获取标准输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	// 获取标准错误管道
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %v", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	// 创建扫描器读取输出
	stdoutScanner := bufio.NewScanner(stdout)
	stderrScanner := bufio.NewScanner(stderr)

	// 创建协程处理标准输出
	go func() {
		for stdoutScanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				output <- stdoutScanner.Text()
			}
		}
	}()

	// 创建协程处理标准错误
	go func() {
		for stderrScanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				output <- "[ERROR] " + stderrScanner.Text()
			}
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		output <- fmt.Sprintf("命令执行完成，退出状态: %v", err)
		return err
	} else {
		output <- "命令执行完成，退出状态: 0"
	}
	return nil
}
