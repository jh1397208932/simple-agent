package cmdUtil

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// CrossPlatformExec 跨平台执行命令 (同步执行)
func CrossPlatformExec(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("命令不能为空")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("zsh", "-c", command)
	case "linux":
		cmd = exec.Command("bash", "-c", command)
	case "windows":
		cmd = exec.Command("cmd.exe", "/C", command)
	default:
		return "", fmt.Errorf("未适配该平台: %s", runtime.GOOS)
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// StreamCommand 流式执行命令并将输出发送到通道
func StreamCommand(ctx context.Context, command string, output chan<- string) error {

	if command == "" {
		return fmt.Errorf("命令不能为空")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "zsh", "-c", command)
	case "linux":
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", command)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败")
	}
	defer stdout.Close()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdout.Close()
		return fmt.Errorf("创建 stderr 管道失败")
	}
	defer stderr.Close()

	if err := cmd.Start(); err != nil {
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("命令启动失败")
	}

	//stdoutScanner := bufio.NewScanner(stdout)
	//stderrScanner := bufio.NewScanner(stderr)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(stdout)
		buffer := make([]byte, 1024)

		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				select {
				case <-ctx.Done():

				default:
					output <- string(buffer[:n])
				}
			}
			if err != nil {
				fmt.Println("标准输出结束")
				if err == io.EOF {
					break
				}
				fmt.Println("Error:", err)
				break
			}
		}
		// for stdoutScanner.Scan() {
		// 	select {
		// 	case <-ctx.Done():
		// 		return
		// 	case output <- stdoutScanner.Text():
		// 	}
		// }
	}()
	wg.Add(1)
	go func() {

		defer wg.Done()
		reader := bufio.NewReader(stderr)
		buffer := make([]byte, 1024)

		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				select {
				case <-ctx.Done():

				default:
					output <- string(buffer[:n])
				}
			}
			if err != nil {
				fmt.Println("异常输出结束")
				if err == io.EOF {
					break
				}
				fmt.Println("Error:", err)
				break
			}
		}
		// for stderrScanner.Scan() {
		// 	select {
		// 	case <-ctx.Done():
		// 		return
		// 	case output <- stderrScanner.Text():
		// 	}
		// }
	}()

	// 监听上下文取消，强制终止命令
	go func() {

		<-ctx.Done()
		fmt.Println("上下文结束 优雅停止命令终端")
		if cmd.Process != nil {
			// 尝试优雅终止
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				// 如果失败，强制杀掉进程
				cmd.Process.Kill()
			}
		}

	}()

	err = cmd.Wait()
	//等待命令输出相应
	wg.Wait()

	if err != nil {
		select {
		case <-ctx.Done():

		default:
			output <- "命令执行失败"
		}

	} else {
		select {
		case <-ctx.Done():

		default:
			output <- "命令执行完成"
		}

	}
	fmt.Println("正常执行结束")
	return nil
}
