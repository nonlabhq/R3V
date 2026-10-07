package keyring

import (
	"syscall"
	"unsafe"
)

// Generic credentials in Windows Credential Manager, kept for this user on
// this computer.

var (
	advapi32    = syscall.NewLazyDLL("advapi32.dll")
	credWriteW  = advapi32.NewProc("CredWriteW")
	credReadW   = advapi32.NewProc("CredReadW")
	credDeleteW = advapi32.NewProc("CredDeleteW")
	credFree    = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	errorNotFound           = 1168
)

type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func set(target, user, secret string) error {
	t, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	u, err := syscall.UTF16PtrFromString(user)
	if err != nil {
		return err
	}
	blob := []byte(secret)
	c := credential{
		Type:               credTypeGeneric,
		TargetName:         t,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistLocalMachine,
		UserName:           u,
	}
	if r, _, err := credWriteW.Call(uintptr(unsafe.Pointer(&c)), 0); r == 0 {
		return err
	}
	return nil
}

func get(target string) (string, error) {
	t, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return "", err
	}
	var p *credential
	if r, _, err := credReadW.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&p))); r == 0 {
		if errno, ok := err.(syscall.Errno); ok && errno == errorNotFound {
			return "", ErrNotFound
		}
		return "", err
	}
	defer credFree.Call(uintptr(unsafe.Pointer(p)))
	return string(unsafe.Slice(p.CredentialBlob, p.CredentialBlobSize)), nil
}

func del(target string) error {
	t, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	if r, _, err := credDeleteW.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0); r == 0 {
		if errno, ok := err.(syscall.Errno); ok && errno == errorNotFound {
			return nil
		}
		return err
	}
	return nil
}
