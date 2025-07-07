package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
)

// SSEEvent 表示一个SSE事件
type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry int
}

// SSEClient SSE客户端
type SSEClient struct {
	url         string
	formParams  map[string]string
	fileParams  map[string]string // filePath -> formField
	headers     map[string]string
	client      *http.Client
	eventChan   chan SSEEvent
	errorChan   chan error
	doneChan    chan struct{}
	mu          sync.Mutex
	connected   bool
	lastEventID string
}

// NewSSEClient 创建新的SSE客户端
func NewSSEClient(url string) *SSEClient {
	return &SSEClient{
		url:        url,
		formParams: make(map[string]string),
		fileParams: make(map[string]string),
		headers:    make(map[string]string),
		client: &http.Client{
			Timeout: 30 * time.Second, // 初始连接超时
		},
		eventChan: make(chan SSEEvent, 100),
		errorChan: make(chan error, 10),
		doneChan:  make(chan struct{}),
	}
}

// AddFormParam 添加表单参数
func (c *SSEClient) AddFormParam(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.formParams[key] = value
}

// AddFileParam 添加文件参数
func (c *SSEClient) AddFileParam(formField, filePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fileParams[filePath] = formField
}

// AddHeader 添加请求头
func (c *SSEClient) AddHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headers[key] = value
}

// SetLastEventID 设置最后事件ID (用于断线续传)
func (c *SSEClient) SetLastEventID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastEventID = id
}

// Connect 连接到SSE端点
func (c *SSEClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("client is already connected")
	}

	// 创建管道用于流式上传
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// 使用goroutine写入表单数据
	go func() {
		defer pw.Close()
		defer writer.Close()

		// 写入表单字段
		for key, value := range c.formParams {
			if err := writer.WriteField(key, value); err != nil {
				pw.CloseWithError(fmt.Errorf("写入表单字段失败: %w", err))
				return
			}
		}

		// 写入文件字段
		for filePath, formField := range c.fileParams {
			file, err := os.Open(filePath)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("打开文件失败: %w", err))
				return
			}
			defer file.Close()

			// 创建文件部分
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
					formField, filepath.Base(filePath)))
			h.Set("Content-Type", "application/octet-stream")

			part, err := writer.CreatePart(h)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("创建文件部分失败: %w", err))
				return
			}

			if _, err = io.Copy(part, file); err != nil {
				pw.CloseWithError(fmt.Errorf("写入文件内容失败: %w", err))
				return
			}
		}
	}()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, pr)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 添加自定义头
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// 设置Last-Event-ID
	if c.lastEventID != "" {
		req.Header.Set("Last-Event-ID", c.lastEventID)
	}

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("非200状态码: %d", resp.StatusCode)
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		resp.Body.Close()
		return fmt.Errorf("非SSE响应，Content-Type: %s", contentType)
	}

	c.connected = true

	// 启动goroutine处理SSE流
	go c.handleSSEStream(resp)

	return nil
}

// handleSSEStream 处理SSE事件流
func (c *SSEClient) handleSSEStream(resp *http.Response) {
	defer resp.Body.Close()

	// 仅在未关闭时才关闭通道
	if c.eventChan != nil {
		defer close(c.eventChan)
	}
	if c.errorChan != nil {
		defer close(c.errorChan)
	}

	reader := bufio.NewReader(resp.Body)
	var event SSEEvent

	for {
		select {
		case <-c.doneChan:
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					//log.Println("min")
					// 正常退出，不认为是错误
					c.Close()
					return
				}
				c.errorChan <- fmt.Errorf("读取错误: %w", err)
				return
			}

			// 处理事件行
			if len(line) <= 2 { // 空行表示事件结束
				if event.Data != "" {
					select {
					case c.eventChan <- event:
						// 更新最后事件ID
						if event.ID != "" {
							c.mu.Lock()
							c.lastEventID = event.ID
							c.mu.Unlock()
						}
					case <-time.After(100 * time.Millisecond):
						log.Println("事件通道已满，丢弃事件")
					}
					event = SSEEvent{} // 重置事件
				}
				continue
			}

			// 解析事件行
			c.parseEventLine(line, &event)
		}
	}
}

// parseEventLine 解析SSE事件行
func (c *SSEClient) parseEventLine(line []byte, event *SSEEvent) {
	line = bytes.TrimRight(line, "\r\n")
	parts := bytes.SplitN(line, []byte(":"), 2)
	if len(parts) < 2 {
		return
	}

	field, value := string(parts[0]), string(bytes.TrimSpace(parts[1]))

	switch field {
	case "event":
		event.Event = value
	case "data":
		event.Data += value + "\n"
	case "id":
		event.ID = value
	case "retry":
		var retry int
		if _, err := fmt.Sscanf(value, "%d", &retry); err == nil {
			event.Retry = retry
		}
	}
}

// Events 返回事件通道
func (c *SSEClient) Events() <-chan SSEEvent {
	return c.eventChan
}

// Errors 返回错误通道
func (c *SSEClient) Errors() <-chan error {
	return c.errorChan
}

// Close 关闭连接
func (c *SSEClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return
	}

	// 关闭 doneChan 和清理通道
	select {
	case <-c.doneChan: // 如果已经关闭，避免重复关闭
	default:
		close(c.doneChan)
	}

	// 关闭客户端连接池
	c.client.CloseIdleConnections()
	c.connected = false
}

// Reconnect 重新连接
func (c *SSEClient) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果之前的连接已关闭，则直接返回错误
	if !c.connected {
		return fmt.Errorf("client is already disconnected")
	}

	// 关闭当前连接并重建
	c.Close()
	return c.Connect(ctx)
}

func sendPostSseToUrl(url string, cmd string) {
	auth, err := AesEncryptGCM(AesPassword, AesKey)
	if err != nil {
		fmt.Println("创建密钥失败")
		return
	}
	//req.Header.Set("Auth", auth)
	// 创建客户端
	client := NewSSEClient("http://" + url)
	client.headers = map[string]string{"Auth": auth}
	// 添加表单参数
	client.AddFormParam("command", cmd)

	// 设置上下文和信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("接收到终止信号，关闭连接...")
		cancel()
		client.Close()
	}()

	// 连接服务器
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer client.Close()

	//log.Println("已连接，等待事件...")

	// 处理事件和错误
	for {
		select {
		case event, ok := <-client.Events():
			if !ok {
				log.Println("=====远程命令执行结束=====")
				return
			}
			printEvent(event)

		case err, ok := <-client.Errors():
			if !ok {
				//log.Println("错误通道已关闭")
				return
			}
			log.Printf("发生错误: %v", err)

			// 尝试自动重连
			log.Println("尝试重新连接...")
			if err := client.Reconnect(ctx); err != nil {
				log.Printf("重连失败: %v", err)
				return
			}
			log.Println("重连成功")

		case <-ctx.Done():
			log.Println("====上下文取消，退出======")
			return
		}
	}
}

// printEvent 打印SSE事件到控制台
func printEvent(event SSEEvent) {
	if event.Event == "data" {
		fmt.Print(event.Data)
	}

}

type Resp struct {
	Code    int    `json:"code"`    // 响应码
	Data    string `json:"data"`    // 响应数据
	Message string `json:"message"` // 响应消息

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
		//close(output)
		return err
	} else {
		output <- "命令执行完成，退出状态: 0"
		//close(output)
	}
	return nil
}

func RunLocalCmd(command string) {
	rootCtx := context.Background()
	// 创建上下文用于超时和取消控制
	ctx, cancel := context.WithTimeout(rootCtx, 10*time.Minute)
	// 确保命令执行完成后取消上下文
	defer cancel()

	// 创建输出通道
	output := make(chan string, 100)
	// 启动命令执行
	go func() {
		StreamCommand(ctx, command, output)
	}()
	// if err := ; err != nil {
	// 	log.Printf("命令执行错误: %v", err)
	// 	sseWriter.WriteEvent("error", fmt.Sprintf("命令执行错误: %v", err))
	// }

	// 监听上下文取消和输出通道
	for {
		select {
		case <-ctx.Done():
			// 上下文被取消，结束流
			fmt.Println("======命令执行已终止======")
			return
		case line, ok := <-output:
			if !ok {
				// 输出通道已关闭，结束流
				fmt.Println("======命令执行完成======")
				return
			}
			// 发送输出行
			fmt.Println(line)
		}
	}
}

type FlowItem struct {
	Type  string `json:"type"`  //localCmd 本机命令  up 上传到ip指定目录   cmd 远程命令
	Data  string `json:"data"`  //执行命令 或 上传文件本地路径
	Data2 string `json:"data2"` // 上传远端 全路径
}
type ServiceFlow struct {
	Ip   []string   `json:"ip"`   //远端ip地址
	Flow []FlowItem `json:"flow"` //执行流程
}

func main() {

	//检查当前程序目录下有没有配置文件
	configByteArray, err := os.ReadFile("config.json")
	if err != nil {
		fmt.Println("读取配置文件失败")
	}
	jsonObject := make(map[string]ServiceFlow)
	//解析json
	json.Unmarshal(configByteArray, &jsonObject)
	serviceNameSlice := make([]string, 0, len(jsonObject))
	if len(jsonObject) == 0 {
		panic("配置文件中没有找到参数")
	}

	//开始解析json 参数检查
	for k1, v1 := range jsonObject {
		//fmt.Printf("配置服务: %s \n", k1)
		serviceNameSlice = append(serviceNameSlice, k1)
		if v1.Ip == nil || len(v1.Ip) == 0 {
			//未配置ip地址
			panic(k1 + "-->未配置ip地址")
		}
		if v1.Flow == nil || len(v1.Flow) == 0 {
			panic(k1 + "-->未配置流程")
		}

		for _, item := range v1.Flow {
			if item.Type != "localCmd" && item.Type != "up" && item.Type != "cmd" {
				panic(fmt.Sprintf("流程 : %s 不支持的类型", item.Type))
			}
			if (item.Type == "localCmd" || item.Type == "cmd") && (item.Data == "") {
				panic(fmt.Sprintf("类型: %s 必须填写data", item.Type))
			}
			if item.Type == "up" && (item.Data == "" || item.Data2 == "") {
				panic(fmt.Sprintf("类型: %s 必须填写data和data2", item.Type))
			}
		}

	}
	sort.Strings(serviceNameSlice)
	var isFirst bool = true
	for {
		if !isFirst {
			outSelect := &survey.Select{
				Message: "退出还是继续",
				Options: []string{"继续", "不继续"},
			}
			var outSelecta string
			survey.AskOne(outSelect, &outSelecta)
			if "不继续" == outSelecta {
				os.Exit(0)
			}
		}
		isFirst = false
		//fmt.Println("=========service===============")
		//fmt.Println(serviceNameSlice)
		//fmt.Println("=========service===============")

		var selectedService []string
		prompt := &survey.MultiSelect{
			Message: "请选择需要执行的服务:",
			Options: serviceNameSlice,
		}
		survey.AskOne(prompt, &selectedService)
		// fmt.Println("======选择了======")
		// for _, val := range selectedService {
		// 	fmt.Println(val)
		// }
		// fmt.Println("=================")
		prompt2 := &survey.Select{
			Message: "请再次确认需要是否正确:",
			Options: []string{"正确", "错误"},
		}
		var yesOrNoSelect string
		survey.AskOne(prompt2, &yesOrNoSelect)
		// fmt.Println("======选择了======")
		// fmt.Println(yesOrNoSelect)
		// fmt.Println("=================")
		if yesOrNoSelect == "错误" {
			fmt.Println("选择错误 重新选择")
			continue
		}
		if len(selectedService) == 0 {
			fmt.Println("未选择服务 无法执行")
			continue
		}
		fmt.Println("===================开始自动执行===================")

		for _, selectServiceItem := range selectedService {
			fmt.Printf("=============服务 %s 开始执行自动流程==========", selectServiceItem)
			sf := jsonObject[selectServiceItem]
			//ip := sf.Ip
			for _, flowItem := range sf.Flow {
				switch flowItem.Type {
				case "localCmd":
					fmt.Printf("-----------------------------开始本机执行命令---------------\n")
					fmt.Printf("命令为: %s", flowItem.Data)
					//执行本机命令
					RunLocalCmd(flowItem.Data)
					fmt.Printf("-------本机命令执行结束--------\n")
				case "up":
					fmt.Println("---------------------------开始上传文件-------------------------\n")
					//上传本地文件到远端目录
					for _, ip := range sf.Ip {
						fmt.Printf("ip: %s 开始上传文件 从本机: %s  to  远端: %s\n", ip, flowItem.Data, flowItem.Data2)
						fmt.Println()
						result, err := streamUploadWithParams(fmt.Sprintf("%s/i/file/uploadToPathReplace", ip), flowItem.Data, map[string]string{"toFilePath": flowItem.Data2})
						fmt.Println()
						if err != nil {
							fmt.Printf("ip: %s 上传失败原因为: %s \n", ip, err.Error())
						}
						if result.Code != 200 {
							fmt.Printf("ip: %s 执行失败原因为 %s \n", ip, result.Message)
						}
						fmt.Printf("ip: %s 上传文件结束\n", ip)
						fmt.Println()
						fmt.Println()
					}
					//uploadFile(fmt.Sprintf("%s/i/file/uploadToPathReplace"), flowItem.Data, flowItem.Data2)
					fmt.Println("-------上传文件结束-----------\n")
				case "cmd":
					fmt.Println("-------------------------开始执行远端命令-----------\n")
					//执行远程命令
					for _, ip := range sf.Ip {
						fmt.Printf("*********ip: %s 开始执行命令: %s **********\n", ip, flowItem.Data)
						fmt.Println()
						sendPostSseToUrl(fmt.Sprintf("%s/i/cmd/executeCmdSse", ip), flowItem.Data)
						fmt.Println()
						fmt.Printf("***ip: %s 执行命令结束 ******8\n", ip)
						fmt.Println()
						fmt.Println()
					}
					fmt.Println("------执行远端命令结束-----------\n")
				default:
					fmt.Println("无效指令类型\n")
				}
			}
			fmt.Printf("=============服务 %s 自动流程结束==========", selectServiceItem)
		}

	}

}

var AesPassword []byte = []byte("meiyoumima")
var AesKey = []byte("*1'Z;XLCZ(*^#^@*()212oawePJ[,23]")

func uploadFile(url, filePath, toFilePath string) (Resp, error) {
	var result Resp
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return result, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 创建multipart writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 创建文件表单字段
	part, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		return result, fmt.Errorf("创建表单文件失败: %v", err)
	}
	writer.WriteField("toFilePath", toFilePath)

	// 将文件内容拷贝到表单
	_, err = io.Copy(part, file)
	if err != nil {
		return result, fmt.Errorf("拷贝文件内容失败: %v", err)
	}

	// 关闭writer以完成写入
	err = writer.Close()
	if err != nil {
		return result, fmt.Errorf("关闭writer失败: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return result, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置Content-Type头
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("读取响应失败: %v", err)
	}

	fmt.Printf("响应状态: %d\n响应内容: %s\n", resp.StatusCode, respBody)

	json.Unmarshal(respBody, &result)
	return result, nil
}
func streamUploadWithParams(url, filePath string, params map[string]string) (Resp, error) {
	var result Resp
	// 创建管道
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// 使用goroutine写入数据
	go func() {
		defer pw.Close()
		defer writer.Close()

		// 1. 添加表单参数
		for key, value := range params {
			if err := writer.WriteField(key, value); err != nil {
				pw.CloseWithError(fmt.Errorf("写入字段 %s 失败: %v", key, err))
				return
			}
		}

		// 2. 添加文件部分
		file, err := os.Open(filePath)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("打开文件失败: %v", err))
			return
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file", file.Name())
		if err != nil {
			pw.CloseWithError(fmt.Errorf("创建文件表单字段失败: %v", err))
			return
		}

		// 流式拷贝文件内容
		if _, err = io.Copy(part, file); err != nil {
			pw.CloseWithError(fmt.Errorf("拷贝文件内容失败: %v", err))
			return
		}
	}()

	// 创建请求
	req, err := http.NewRequest("POST", "http://"+url, pr)
	if err != nil {
		return result, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置Content-Type头
	req.Header.Set("Content-Type", writer.FormDataContentType())
	auth, err := AesEncryptGCM(AesPassword, AesKey)
	if err != nil {
		return result, fmt.Errorf("创建鉴权token失败: %v", err)
	}
	req.Header.Set("Auth", auth)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("读取响应失败: %v", err)
	}

	fmt.Printf("响应状态: %d\n响应内容: %s\n", resp.StatusCode, body)
	json.Unmarshal(body, &result)
	return result, nil
}

func AesEncryptGCM(plaintext, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func AesDecryptGCM(ciphertext string, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	if len(decoded) < gcm.NonceSize() {
		return nil, errors.New("密文太短")
	}

	nonce := decoded[:gcm.NonceSize()]
	decoded = decoded[gcm.NonceSize():]

	return gcm.Open(nil, nonce, decoded, nil)
}
