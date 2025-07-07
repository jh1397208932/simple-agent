
## 简介
 在工作时总会碰到在开发测试环境 ,需要频繁的执行命令和发包 ,但是没有构建服务器 ,服务本身没办法请求外部下载依赖.
 本地编译还需要频繁连接堡垒机,特别是mac环境下, 很多堡垒机不支持mac ,会造成开发人员的一些重复 繁琐的发布流程.
 这个demo程序就是为了解决这样的问题 ,未经过严格测试,请勿在生产环境使用
## 功能
日常工作中发布流程:
    1. 本地将需求分支合并到开发分支
    2. 编译项目
    3. 打开堡垒机
    4. 找到服务对应的ip地址
    5. 打开xshell xftp 
    6. 上传文件
    7. 启动项目
    8. 查看日志

这个程序能干什么:
    1.本地自动执行命令  切换分支 编译代码
    2.上传本地文件到服务器
    3.服务器端执行命令 并且返回结果

用了以后流程是什么:
    1.配置代理端ip+端口  
    2.配置本机需要执行的命令 (多个服务)
    3.需要发布的时候 执行client程序 选择要发布的服务,自动上传

## 架构
    随手写的demo 没啥架构
    一个client端 在你电脑上  一个agent端放在测试环境服务器上 
## 编译
### 环境配置
* 安装golang
* (可选) 安装交叉编译依赖库 在lib.mac.arm文件夹中
* (可选) 编译平台依赖库 配置环境变量
#### agent端
 cgo编译
```sh
   CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=x86_64-unknown-linux-musl-gcc go build -o target/simple_ag_v2_linux_x86_64 main.go
```
 关闭cgo编译
```sh
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o target/simple-agent-v2-linux-x86_64 main.go
```
#### client
windows
```sh
GOOS=windows GOARCH=amd64 go build -o target/windows3.exe main.go
```

#### 常用架构编译参数
```sh
# 编译 Windows 64 位程序 (.exe)
GOOS=windows GOARCH=amd64 go build -o output.exe

# 编译 Windows 32 位程序
GOOS=windows GOARCH=386 go build -o output32.exe

# 编译 Linux 64 位
GOOS=linux GOARCH=amd64 go build -o output-linux

# 编译 macOS Intel 芯片
GOOS=darwin GOARCH=amd64 go build -o output-mac-intel

# 编译 macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o output-mac-m1


```