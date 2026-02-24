package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
)

const (
	ServerURL = "https://www.xxx.com/api.php" // 接口地址
	LocalPort = "12378"                       // 一般不需要改，占用了再改
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procSetWinEventHook          = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent           = user32.NewProc("UnhookWinEvent")
	procGetMessage               = user32.NewProc("GetMessageW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")

	AppStartTime = time.Now().Format(time.RFC3339)
	IsRunning    = true
)

const (
	WINEVENT_OUTOFCONTEXT   = 0
	EVENT_SYSTEM_FOREGROUND = 0x0003
)

type Payload struct {
	ProcessName  string `json:"process_name"`
	AppStartTime string `json:"app_start_time"`
	EventTime    string `json:"event_time"`
	Status       string `json:"status"`
}

func getActiveProcessName() string {
	hwnd, _, _ := user32.NewProc("GetForegroundWindow").Call()
	if hwnd == 0 {
		return "Idle"
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return "Unknown"
	}
	name, _ := p.Name()
	return name
}

func sendToAPI(name string, status string) {
	data := Payload{
		ProcessName:  name,
		AppStartTime: AppStartTime,
		EventTime:    time.Now().Format(time.RFC3339),
		Status:       status,
	}
	b, _ := json.Marshal(data)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(ServerURL, "application/json", bytes.NewBuffer(b))
	if err == nil {
		resp.Body.Close()
	}
}

func startLocalServer() {
	l, err := net.Listen("tcp", "127.0.0.1:"+LocalPort)
	if err != nil {
		return
	}
	for {
		conn, _ := l.Accept()
		buf := make([]byte, 128)
		n, _ := conn.Read(buf)
		cmd := string(buf[:n])
		switch cmd {
		case "on":
			IsRunning = true
			sendToAPI(getActiveProcessName(), "active")
		case "off":
			IsRunning = false
			sendToAPI("None", "off")
		case "exit":
			sendToAPI("None", "exit")
			os.Exit(0)
		}
		conn.Close()
	}
}

func main() {
	// 客户端控制模式
	if len(os.Args) > 1 {
		cmd := os.Args[1]
		conn, err := net.Dial("tcp", "127.0.0.1:"+LocalPort)
		if err == nil {
			conn.Write([]byte(cmd))
			conn.Close()
		}
		return
	}

	// 心跳
	go func() {
		for {
			if IsRunning {
				sendToAPI(getActiveProcessName(), "active")
			}
			time.Sleep(10 * time.Second)
		}
	}()

	// 本地指令监听
	go startLocalServer()

	// 退出
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-c
		sendToAPI("None", "exit")
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	// 事件钩子
	callback := syscall.NewCallback(func(hWinEventHook uintptr, event uint32, hwnd uintptr, idObject int32, idChild int32, idEventThread uint32, dwmsEventTime uint32) uintptr {
		if event == EVENT_SYSTEM_FOREGROUND && IsRunning {
			sendToAPI(getActiveProcessName(), "active")
		}
		return 0
	})
	hook, _, _ := procSetWinEventHook.Call(EVENT_SYSTEM_FOREGROUND, EVENT_SYSTEM_FOREGROUND, 0, callback, 0, 0, WINEVENT_OUTOFCONTEXT)
	defer procUnhookWinEvent.Call(hook)

	var msg struct {
		hwnd    uintptr
		message uint32
		wParam  uintptr
		lParam  uintptr
		time    uint32
		pt      struct{ x, y int32 }
	}
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret <= 0 {
			break
		}
	}
}
