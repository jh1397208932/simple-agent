package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/google/uuid"
)

// ANSI颜色常量
const (
	Reset       = "\033[0m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Cyan        = "\033[36m"
	Grey        = "\033[90m"
	EmojiRed    = "🟧"
	EmojiGreen  = "✅"
	EmojiYellow = "🟨"
	EmojiBlue   = "🔵"
)

var HmacKey = "tesw-dadad0-pm2-pp9"

// ================= 日志工具 ===================
func now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func Info(msg string) {
	fmt.Fprintf(os.Stdout, "%s %s🔵 %s%s\n", Cyan, now(), msg, Reset)
}

func Warn(msg string) {
	fmt.Fprintf(os.Stdout, "%s %s⚠️  %s%s\n", Yellow, now(), msg, Reset)
}

func ErrorLog(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s❌ %s%s\n", Red, now(), msg, Reset)
}

func Success(msg string) {
	fmt.Fprintf(os.Stdout, "%s %s✅ %s%s\n", Green, now(), msg, Reset)
}

func Step(msg string) {
	fmt.Fprintf(os.Stdout, "\n%s %s🟢 ===== %s =====%s\n", Cyan, now(), msg, Reset)
}

func End(msg string) {
	fmt.Fprintf(os.Stdout, "\n%s %s🏁 %s 🏁%s\n", Green, now(), msg, Reset)
}

func Bye(msg string) {
	fmt.Fprintf(os.Stdout, "%s %s👋 %s%s\n", Grey, now(), msg, Reset)
}

// ================= SSE 结构 ===================
type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry int
}

type SSEClient struct {
	url         string
	formParams  map[string]string
	fileParams  map[string]string
	headers     map[string]string
	client      *http.Client
	eventChan   chan SSEEvent
	errorChan   chan error
	doneChan    chan struct{}
	mu          sync.Mutex
	connected   bool
	lastEventID string
}

func NewSSEClient(url string) *SSEClient {
	return &SSEClient{
		url:        url,
		formParams: make(map[string]string),
		fileParams: make(map[string]string),
		headers:    make(map[string]string),
		client: &http.Client{
			Timeout: 10 * 60 * time.Second,
		},
		eventChan: make(chan SSEEvent, 100),
		errorChan: make(chan error, 10),
		doneChan:  make(chan struct{}),
	}
}

func (c *SSEClient) AddFormParam(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.formParams[key] = value
}

func (c *SSEClient) AddFileParam(formField, filePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fileParams[filePath] = formField
}

func (c *SSEClient) AddHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headers[key] = value
}

func (c *SSEClient) SetLastEventID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastEventID = id
}

func (c *SSEClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("client is already connected")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		for key, value := range c.formParams {
			if err := writer.WriteField(key, value); err != nil {
				pw.CloseWithError(fmt.Errorf("写入表单字段失败: %w", err))
				return
			}
		}

		for filePath, formField := range c.fileParams {
			file, err := os.Open(filePath)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("打开文件失败: %w", err))
				return
			}
			defer file.Close()

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

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, pr)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	if c.lastEventID != "" {
		req.Header.Set("Last-Event-ID", c.lastEventID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("非200状态码: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		resp.Body.Close()
		return fmt.Errorf("非SSE响应，Content-Type: %s", contentType)
	}

	c.connected = true
	go c.handleSSEStream(resp)
	return nil
}

func (c *SSEClient) handleSSEStream(resp *http.Response) {
	defer resp.Body.Close()
	if c.eventChan != nil {
		defer close(c.eventChan)
	}

	reader := bufio.NewReader(resp.Body)
	buffer := make([]byte, 1024)
	var event SSEEvent

	for {
		select {
		case <-c.doneChan:
			Warn("[SSE] 收到关闭信号，退出处理循环")
			return
		default:
			offSet, err := reader.Read(buffer)
			if err != nil {
				if errors.Is(err, io.EOF) {
					if event.Data != "" {
						event.Data = strings.TrimSuffix(event.Data, "\n")
						select {
						case c.eventChan <- event:
							if event.ID != "" {
								c.mu.Lock()
								c.lastEventID = event.ID
								c.mu.Unlock()
							}
						case <-time.After(100 * time.Millisecond):
							Warn("事件通道已满，丢弃最后一个事件")
						}
					}
					c.Close()
					return
				}
				c.errorChan <- fmt.Errorf("读取错误: %w", err)
				return
			}

			str := string(buffer[:offSet])
			str = strings.ReplaceAll(str, "event: heartbeat\ndata: 1", "")
			// 原先直接打印原始SSE数据，这里美化输出
			clean := strings.ReplaceAll(str, "event: data\ndata: ", "")
			Info(clean)
		}
	}
}

func (c *SSEClient) parseEventLine(line []byte, event *SSEEvent) {
	line = bytes.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return
	}

	if line[0] == ':' {
		return
	}

	colonIndex := bytes.IndexByte(line, ':')
	if colonIndex == -1 {
		event.Data += string(line) + "\n"
		return
	}

	field := string(line[:colonIndex])
	var value string

	if colonIndex+1 < len(line) {
		if line[colonIndex+1] == ' ' {
			value = string(line[colonIndex+2:])
		} else {
			value = string(line[colonIndex+1:])
		}
	}

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

func (c *SSEClient) Events() <-chan SSEEvent {
	return c.eventChan
}

func (c *SSEClient) Errors() <-chan error {
	return c.errorChan
}

func (c *SSEClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return
	}

	select {
	case <-c.doneChan:
	default:
		close(c.doneChan)
	}

	c.client.CloseIdleConnections()
	c.connected = false
}

func (c *SSEClient) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("client is already disconnected")
	}

	c.Close()
	return c.Connect(ctx)
}

func sendPostSseToUrl(url string, cmd string) {
	auth, err := AesEncryptGCM(AesPassword, AesKey)
	if err != nil {
		ErrorLog(fmt.Sprintf("创建鉴权token失败: %v", err))
		return
	}

	client := NewSSEClient("http://" + url)
	client.headers = map[string]string{"Auth": auth}
	timestamp := time.Now().Unix()
	nonce := uuid.New().String()
	signature := GenerateHMACSignature(timestamp, nonce, auth, HmacKey)

	client.headers["Timestamp"] = strconv.FormatInt(timestamp, 10)
	client.headers["Nonce"] = nonce
	client.headers["Signature"] = signature
	client.AddFormParam("command", cmd)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		Warn("接收到终止信号，正在取消所有操作...")
		cancel()
		client.Close()
	}()

	if err := client.Connect(ctx); err != nil {
		ErrorLog(fmt.Sprintf("连接失败: %v", err))
		// 保持原有行为：fatal 等价处理（退出）
		os.Exit(1)
	}
	defer client.Close()

	for {
		select {
		case event, ok := <-client.Events():
			if !ok {
				Success("===== 远程命令执行结束 =====")
				return
			}
			printEvent(event)

		case err, ok := <-client.Errors():
			if !ok {
				return
			}
			ErrorLog(fmt.Sprintf("发生错误: %v", err))

		case <-ctx.Done():
			Warn("==== 上下文取消，退出 ====")
			return
		}
	}
}

func printEvent(event SSEEvent) {
	if event.Event == "data" {
		// 保持事件原始内容，但使用统一的视觉输出
		// 如果是多行，逐行输出以便每行都有时间戳
		lines := strings.Split(strings.TrimSuffix(event.Data, "\n"), "\n")
		for _, ln := range lines {
			if strings.TrimSpace(ln) == "" {
				continue
			}
			Info(ln)
		}
	}
}

type Resp struct {
	Code    int    `json:"code"`
	Data    string `json:"data"`
	Message string `json:"message"`
}

// StreamCommand 流式执行命令并将输出发送到通道
func StreamCommand(ctx context.Context, command string, output chan<- string) error {
	defer close(output)

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
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	stdoutScanner := bufio.NewScanner(stdout)
	stderrScanner := bufio.NewScanner(stderr)

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

	if err := cmd.Wait(); err != nil {
		output <- fmt.Sprintf("命令执行完成，退出状态: %v", err)
		return err
	} else {
		output <- "命令执行完成，退出状态: 0"
	}
	return nil
}

func RunLocalCmd(command string) {
	rootCtx := context.Background()
	ctx, cancel := context.WithTimeout(rootCtx, 10*time.Minute)
	defer cancel()

	output := make(chan string, 100)
	go func() {
		StreamCommand(ctx, command, output)
	}()

	for {
		select {
		case <-ctx.Done():
			Warn("命令执行已终止")
			return
		case line, ok := <-output:
			if !ok {
				Success("命令执行完成")
				return
			}
			if strings.HasPrefix(line, "[ERROR]") {
				// 去掉前缀再输出
				ErrorLog(strings.TrimPrefix(line, "[ERROR] "))
			} else {
				Info(line)
			}
		}
	}
}

type FlowItem struct {
	Type  string `json:"type"`
	Data  string `json:"data"`
	Data2 string `json:"data2"`
}
type ServiceFlow struct {
	Ip   []string   `json:"ip"`
	Flow []FlowItem `json:"flow"`
}

func main() {
	// 读取配置文件
	configByteArray, err := os.ReadFile("config.json")
	if err != nil {
		ErrorLog(fmt.Sprintf("读取配置文件失败: %v", err))
		return
	}
	jsonObject := make(map[string]ServiceFlow)
	if err := json.Unmarshal(configByteArray, &jsonObject); err != nil {
		ErrorLog(fmt.Sprintf("解析配置文件失败: %v", err))
		return
	}

	serviceNameSlice := make([]string, 0, len(jsonObject))
	if len(jsonObject) == 0 {
		ErrorLog("配置文件中没有找到参数")
		return
	}

	// 参数检查
	for k1, v1 := range jsonObject {
		serviceNameSlice = append(serviceNameSlice, k1)
		if v1.Ip == nil || len(v1.Ip) == 0 {
			ErrorLog(fmt.Sprintf("服务 %s --> 未配置ip地址", k1))
			return
		}
		if v1.Flow == nil || len(v1.Flow) == 0 {
			ErrorLog(fmt.Sprintf("服务 %s --> 未配置流程", k1))
			return
		}

		for _, item := range v1.Flow {
			if item.Type != "localCmd" && item.Type != "up" && item.Type != "cmd" {
				ErrorLog(fmt.Sprintf("服务 %s 流程 : %s 不支持的类型", k1, item.Type))
				return
			}
			if (item.Type == "localCmd" || item.Type == "cmd") && (item.Data == "") {
				ErrorLog(fmt.Sprintf("服务 %s 类型: %s 必须填写data", k1, item.Type))
				return
			}
			if item.Type == "up" && (item.Data == "" || item.Data2 == "") {
				ErrorLog(fmt.Sprintf("服务 %s 类型: %s 必须填写data和data2", k1, item.Type))
				return
			}
		}
	}

	sort.Strings(serviceNameSlice)
	isFirst := true

	for {
		if !isFirst {
			outSelect := &survey.Select{
				Message: "退出还是继续",
				Options: []string{"继续", "不继续"},
			}
			var outSelecta string
			survey.AskOne(outSelect, &outSelecta)
			if "不继续" == outSelecta {
				Bye("程序已终止，再见!")
				return
			}
		}
		isFirst = false

		var selectedService []string
		prompt := &survey.MultiSelect{
			Message: "请选择需要执行的服务:",
			Options: serviceNameSlice,
		}
		survey.AskOne(prompt, &selectedService)

		prompt2 := &survey.Select{
			Message: "请再次确认需要是否正确:",
			Options: []string{"正确", "错误"},
		}
		var yesOrNoSelect string
		survey.AskOne(prompt2, &yesOrNoSelect)

		if yesOrNoSelect == "错误" {
			Warn("选择错误，重新选择")
			continue
		}
		if len(selectedService) == 0 {
			Warn("未选择服务，无法执行")
			continue
		}

		Step("开始自动执行")

		for _, selectServiceItem := range selectedService {
			sf := jsonObject[selectServiceItem]
			Step(fmt.Sprintf("服务 %s 开始执行自动流程", selectServiceItem))

			for _, flowItem := range sf.Flow {
				switch flowItem.Type {
				case "localCmd":
					Info("开始本机执行命令")
					Info(fmt.Sprintf("命令为: %s", flowItem.Data))
					RunLocalCmd(flowItem.Data)
					Success("本机命令执行结束")

				case "up":
					Info("开始上传文件")
					for _, ip := range sf.Ip {
						Info(fmt.Sprintf("上传到 %s\n  本机: %s\n  远端: %s", ip, flowItem.Data, flowItem.Data2))
						result, err := streamUploadWithParams(fmt.Sprintf("%s/i/file/uploadToPathReplace", ip), flowItem.Data, map[string]string{"toFilePath": flowItem.Data2})
						if err != nil {
							ErrorLog(fmt.Sprintf("%s 上传失败 原因: %s", ip, err.Error()))
						} else if result.Code != 200 {
							ErrorLog(fmt.Sprintf("%s 执行失败 原因: %s", ip, result.Message))
						} else {
							Success(fmt.Sprintf("上传完成 (%s)", ip))
						}
					}
					Success("上传文件结束")

				case "cmd":
					Info("开始执行远端命令")
					for _, ip := range sf.Ip {
						Info(fmt.Sprintf("[%s] 开始执行命令: %s", ip, flowItem.Data))
						sendPostSseToUrl(fmt.Sprintf("%s/i/cmd/executeCmdSse", ip), flowItem.Data)
						Success(fmt.Sprintf("[%s] 执行命令结束", ip))
					}
					Success("执行远端命令结束")

				default:
					Warn("无效指令类型")
				}
			}
			Success(fmt.Sprintf("服务 %s 自动流程结束", selectServiceItem))
		}

		End("所有选择的服务均已执行完毕")
	}
}

var AesPassword []byte = []byte("meiyoumima")
var AesKey = []byte("*1'Z;XLCZ(*^#^@*()212oawePJ[,23]")

func streamUploadWithParams(url, filePath string, params map[string]string) (Resp, error) {
	var result Resp

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		for key, value := range params {
			if err := writer.WriteField(key, value); err != nil {
				pw.CloseWithError(fmt.Errorf("写入字段 %s 失败: %v", key, err))
				return
			}
		}

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

		if _, err = io.Copy(part, file); err != nil {
			pw.CloseWithError(fmt.Errorf("拷贝文件内容失败: %v", err))
			return
		}
	}()

	req, err := http.NewRequest("POST", "http://"+url, pr)
	if err != nil {
		return result, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	auth, err := AesEncryptGCM(AesPassword, AesKey)
	if err != nil {
		return result, fmt.Errorf("创建鉴权token失败: %v", err)
	}
	req.Header.Set("Auth", auth)

	timestamp := time.Now().Unix()
	nonce := uuid.New().String()
	signature := GenerateHMACSignature(timestamp, nonce, auth, HmacKey)

	req.Header.Set("Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("Nonce", nonce)
	req.Header.Set("Signature", signature)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("读取响应失败: %v", err)
	}

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

func GenerateHMACSignature(timestamp int64, nonce string, data string, secretKey string) string {
	signStr := fmt.Sprintf("%d:%s:%s", timestamp, nonce, data)
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(signStr))
	return hex.EncodeToString(h.Sum(nil))
}
