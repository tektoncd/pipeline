// Package pageant provides an interface to PyTTY pageant.exe utility.
// This package is windows-only
package pageant

// see https://github.com/Yasushi/putty/blob/master/windows/winpgntc.c#L155
// see https://github.com/paramiko/paramiko/blob/master/paramiko/win_pageant.py

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// MaxMessageLen defines maximum size of message can be sent to pageant
const MaxMessageLen = 8192

var (
	// ErrPageantNotFound returns when pageant process not found
	ErrPageantNotFound = errors.New("pageant process not found")
	// ErrSendMessage returns when message to pageant cannt be sent
	ErrSendMessage = errors.New("error sending message")

	// ErrMessageTooLong returns when message is too long (see MaxMessageLen)
	ErrMessageTooLong = errors.New("message too long")
	// ErrInvalidMessageFormat returns when message have invalid fomat
	ErrInvalidMessageFormat = errors.New("invalid message format")
	// ErrResponseTooLong returns when response from pageant is too long
	ErrResponseTooLong = errors.New("response too long")
)

/////////////////////////

const (
	agentCopydataID = 0x804e50ba
	wmCopydata      = 74
)

type copyData struct {
	dwData uintptr
	cbData uint32
	lpData unsafe.Pointer
}

var (
	lock sync.Mutex

	winFindWindow         = winAPI("user32.dll", "FindWindowW")
	winGetCurrentThreadID = winAPI("kernel32.dll", "GetCurrentThreadId")
	winSendMessage        = winAPI("user32.dll", "SendMessageW")
)

func winAPI(dllName, funcName string) func(...uintptr) (uintptr, uintptr, error) {
	proc := syscall.MustLoadDLL(dllName).MustFindProc(funcName)
	return func(a ...uintptr) (uintptr, uintptr, error) { return proc.Call(a...) }
}

// Available returns true if Pageant is started
func Available() bool { return pageantWindow() != 0 }

// Query sends message msg to Pageant and returns response or error.
// 'msg' is raw agent request with length prefix
// Response is raw agent response with length prefix
func query(msg []byte) ([]byte, error) {
	if len(msg) > MaxMessageLen {
		return nil, ErrMessageTooLong
	}

	msgLen := binary.BigEndian.Uint32(msg[:4])
	if len(msg) != int(msgLen)+4 {
		return nil, ErrInvalidMessageFormat
	}

	lock.Lock()
	defer lock.Unlock()

	paWin := pageantWindow()

	if paWin == 0 {
		return nil, ErrPageantNotFound
	}

	thID, _, _ := winGetCurrentThreadID()
	mapName := fmt.Sprintf("PageantRequest%08x", thID)
	pMapName, _ := syscall.UTF16PtrFromString(mapName)

	mmap, err := syscall.CreateFileMapping(syscall.InvalidHandle, nil, syscall.PAGE_READWRITE, 0, MaxMessageLen+4, pMapName)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(mmap)

	ptr, err := syscall.MapViewOfFile(mmap, syscall.FILE_MAP_WRITE, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.UnmapViewOfFile(ptr)

	mmSlice := (*(*[MaxMessageLen]byte)(unsafe.Pointer(ptr)))[:]

	copy(mmSlice, msg)

	mapNameBytesZ := append([]byte(mapName), 0)

	cds := copyData{
		dwData: agentCopydataID,
		cbData: uint32(len(mapNameBytesZ)),
		lpData: unsafe.Pointer(&(mapNameBytesZ[0])),
	}

	resp, _, _ := winSendMessage(paWin, wmCopydata, 0, uintptr(unsafe.Pointer(&cds)))

	if resp == 0 {
		return nil, ErrSendMessage
	}

	respLen := binary.BigEndian.Uint32(mmSlice[:4])
	if respLen > MaxMessageLen-4 {
		return nil, ErrResponseTooLong
	}

	respData := make([]byte, respLen+4)
	copy(respData, mmSlice)

	return respData, nil
}

func pageantWindow() uintptr {
	nameP, _ := syscall.UTF16PtrFromString("Pageant")
	h, _, _ := winFindWindow(uintptr(unsafe.Pointer(nameP)), uintptr(unsafe.Pointer(nameP)))
	return h
}
