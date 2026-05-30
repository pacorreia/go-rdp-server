//go:build windows

package broker

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	userPrivUser         = 1
	ufScript             = 0x0001
	ufDontExpirePassword = 0x10000
)

type userInfo1 struct {
	Name        *uint16
	Password    *uint16
	PasswordAge uint32
	Priv        uint32
	HomeDir     *uint16
	Comment     *uint16
	Flags       uint32
	ScriptPath  *uint16
}

type localGroupMembersInfo3 struct {
	DomainAndName *uint16
}

var (
	netapi32                   = syscall.NewLazyDLL("netapi32.dll")
	advapi32                   = syscall.NewLazyDLL("advapi32.dll")
	procNetUserAdd             = netapi32.NewProc("NetUserAdd")
	procNetUserDel             = netapi32.NewProc("NetUserDel")
	procNetLocalGroupAddMember = netapi32.NewProc("NetLocalGroupAddMembers")
	procLogonUserW             = advapi32.NewProc("LogonUserW")
)

func CreateTempUser(username, password string) error {
	namePtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	passwordPtr, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return err
	}

	user := userInfo1{
		Name:     namePtr,
		Password: passwordPtr,
		Priv:     userPrivUser,
		Flags:    ufScript | ufDontExpirePassword,
	}

	var parmErr uint32
	ret, _, _ := procNetUserAdd.Call(
		0,
		1,
		uintptr(unsafe.Pointer(&user)),
		uintptr(unsafe.Pointer(&parmErr)),
	)
	if ret != 0 {
		return fmt.Errorf("NetUserAdd failed: %w", windows.Errno(ret))
	}

	return nil
}

func DeleteTempUser(username string) error {
	namePtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	ret, _, _ := procNetUserDel.Call(0, uintptr(unsafe.Pointer(namePtr)))
	if ret != 0 {
		return fmt.Errorf("NetUserDel failed: %w", windows.Errno(ret))
	}
	return nil
}

func AddToRDPGroup(username string) error {
	groupPtr, err := windows.UTF16PtrFromString("Remote Desktop Users")
	if err != nil {
		return err
	}
	userPtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return err
	}

	member := localGroupMembersInfo3{DomainAndName: userPtr}
	ret, _, _ := procNetLocalGroupAddMember.Call(
		0,
		uintptr(unsafe.Pointer(groupPtr)),
		3,
		uintptr(unsafe.Pointer(&member)),
		1,
	)
	if ret != 0 {
		return fmt.Errorf("NetLocalGroupAddMembers failed: %w", windows.Errno(ret))
	}
	return nil
}

func ImpersonateUser(username, password string) (windows.Token, error) {
	usernamePtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return 0, err
	}
	passwordPtr, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return 0, err
	}
	var token windows.Token
	ret, _, callErr := procLogonUserW.Call(
		uintptr(unsafe.Pointer(usernamePtr)),
		0,
		uintptr(unsafe.Pointer(passwordPtr)),
		2, // LOGON32_LOGON_INTERACTIVE
		0, // LOGON32_PROVIDER_DEFAULT
		uintptr(unsafe.Pointer(&token)),
	)
	if ret == 0 {
		return 0, callErr
	}
	return token, nil
}
