package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/urfave/cli/v2"
	"golang.org/x/sys/windows"
)

var (
	moduser32 = windows.NewLazySystemDLL("user32.dll")

	procEnumWindows              = moduser32.NewProc("EnumWindows")
	procIsWindowVisible          = moduser32.NewProc("IsWindowVisible")
	procGetWindowTextW           = moduser32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = moduser32.NewProc("GetWindowThreadProcessId")
	procSwitchToThisWindow       = moduser32.NewProc("SwitchToThisWindow")
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "m",
			},
			&cli.BoolFlag{
				Name: "l",
			},
			&cli.BoolFlag{
				Name: "lp",
			},
			&cli.StringFlag{
				Name: "a",
			},
			&cli.BoolFlag{
				Name: "i",
			},
		},
		Action: func(c *cli.Context) (err error) {
			if c.Bool("m") {
				fmt.Println("Name: win-dwm")
				return
			}
			if c.Bool("l") || c.Bool("lp") {
				err = listWindows(c)
				return
			}
			if c.String("a") != "" {
				err = switchToWindow(c)
				return
			}
			return
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalln(err)
	}
}

type windowInfo struct {
	HWND    windows.HWND
	Desktop int64
	PID     uint64
	Host    string
	Title   string
}

type enumWindowsParam struct {
	Title      string
	IncludePID bool
	Windows    []*windowInfo
}

func listWindowsCallback(hwnd windows.HWND, lparam uintptr) uintptr {
	param := (*enumWindowsParam)(unsafe.Pointer(lparam))
	var (
		r1 uintptr
		e1 error
	)
	r1, _, e1 = procIsWindowVisible.Call(uintptr(hwnd))
	visible := uint32(r1)
	if visible != 1 {
		return 1
	}
	bufLen := 512
	buf := make([]uint16, bufLen)
	r1, _, e1 = procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(bufLen))
	if e1 != nil {
		if errno, ok := e1.(syscall.Errno); ok {
			if errno != 0 {
				return 0
			}
		}
	}
	length := uint32(r1)
	if length == 0 {
		return 1
	}
	title := windows.UTF16ToString(buf)
	if title != "" && param.Title != "" {
		if !strings.Contains(title, param.Title) {
			return 1
		}
	}
	info := &windowInfo{
		HWND:  hwnd,
		Title: title,
	}
	if param.IncludePID {
		var pid uint64
		procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pid)))
		info.PID = pid
	}
	param.Windows = append(param.Windows, info)
	return 1
}

func listWindows(c *cli.Context) (err error) {
	includePID := c.Bool("lp")
	param := &enumWindowsParam{
		IncludePID: includePID,
	}
	hostname, err := os.Hostname()
	if err != nil {
		return
	}
	r1, _, e1 := procEnumWindows.Call(windows.NewCallback(listWindowsCallback), uintptr(unsafe.Pointer(param)))
	rv := uint32(r1)
	if rv != 1 {
		err = e1
		return
	}
	for _, w := range param.Windows {
		if includePID {
			fmt.Printf("0x%x %d %d %s %s\n", w.HWND, w.Desktop, w.PID, hostname, w.Title)
		} else {
			fmt.Printf("0x%x %d %s %s\n", w.HWND, w.Desktop, hostname, w.Title)
		}
	}
	return
}

func switchToWindow(c *cli.Context) (err error) {
	var hwnd uintptr
	value := c.String("a")
	isNumeric := c.Bool("i")
	if isNumeric {
		var h uint64
		h, err = strconv.ParseUint(value, 0, 64)
		if err != nil {
			return
		}
		hwnd = uintptr(h)
	} else {
		// TODO: find window by value
		hwnd = 0
	}
	r1, _, e1 := procSwitchToThisWindow.Call(hwnd)
	rv := int32(r1)
	if rv != 1 {
		err = e1
	}
	return
}
