package main

import (
	"fmt"
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
	procShowWindow               = moduser32.NewProc("ShowWindow")
)

const (
	SW_SHOWNORMAL  = 1
	SW_SHOW        = 5
	SW_RESTORE     = 9
	SW_SHOWDEFAULT = 10
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
				Name: "p",
			},
			&cli.StringFlag{
				Name: "a",
			},
			&cli.BoolFlag{
				Name: "i",
			},
			&cli.IntFlag{
				Name:  "show",
				Value: SW_RESTORE,
			},
		},
		Action: func(c *cli.Context) (err error) {
			if c.Bool("m") {
				fmt.Println("Name: win-dwm")
				return
			}
			if c.Bool("l") {
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
		fmt.Fprintln(os.Stderr, err)
	}
}

type windowInfo struct {
	HWND    windows.HWND
	Desktop int64 // TODO: obtain deskop index
	PID     uint64
	Host    string
	Title   string
}

type enumWindowsParam struct {
	ByTitle    string
	ByPID      uint64
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
	if title != "" && param.ByTitle != "" {
		if !strings.Contains(title, param.ByTitle) {
			return 1
		}
	}
	info := &windowInfo{
		HWND:  hwnd,
		Title: title,
	}
	if param.IncludePID || param.ByPID > 0 {
		var tid uint64
		r1, _, e1 = procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&tid)))
		if e1 != nil {
			if errno, ok := e1.(syscall.Errno); ok {
				if errno != 0 {
					return 0
				}
			}
		}
		// FIXME: weird
		// pid := uint64(r1)
		if param.ByPID > 0 && tid != param.ByPID {
			return 1
		}
		info.PID = tid
	}
	param.Windows = append(param.Windows, info)
	return 1
}

func listWindows(c *cli.Context) (err error) {
	includePID := c.Bool("p")
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
	if c.Bool("i") {
		var h uint64
		h, err = strconv.ParseUint(value, 0, 64)
		if err != nil {
			return
		}
		hwnd = uintptr(h)
	} else {
		param := &enumWindowsParam{}
		if c.Bool("p") {
			var pid uint64
			pid, err = strconv.ParseUint(value, 0, 64)
			if err != nil {
				return
			}
			param.ByPID = pid
		} else {
			param.ByTitle = value
		}
		r1, _, e1 := procEnumWindows.Call(windows.NewCallback(listWindowsCallback), uintptr(unsafe.Pointer(param)))
		rv := uint32(r1)
		if rv != 1 {
			err = e1
			return
		}
		if len(param.Windows) == 0 {
			return fmt.Errorf("can not find window with title contains: %s", value)
		}
		hwnd = uintptr(param.Windows[0].HWND)
	}
	r1, _, e1 := procSwitchToThisWindow.Call(hwnd)
	rv := int32(r1)
	if rv != 1 {
		err = e1
		return
	}
	show := c.Int("show")
	procShowWindow.Call(hwnd, uintptr(show))
	return
}
