//go:build windows

package broker

import (
	"fmt"
	"log"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	userPrivilegeUser    = 1
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

type userInfo1003 struct {
	Password *uint16
}

var (
	netapi32                    = syscall.NewLazyDLL("netapi32.dll")
	advapi32                    = syscall.NewLazyDLL("advapi32.dll")
	procNetUserAdd              = netapi32.NewProc("NetUserAdd")
	procNetUserDel              = netapi32.NewProc("NetUserDel")
	procNetUserSetInfo          = netapi32.NewProc("NetUserSetInfo")
	procNetLocalGroupAddMembers = netapi32.NewProc("NetLocalGroupAddMembers")
	procLogonUserW              = advapi32.NewProc("LogonUserW")
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
		Priv:     userPrivilegeUser,
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
		if parmErr != 0 {
			return fmt.Errorf("NetUserAdd failed: %w (invalid parameter index: %d)", windows.Errno(ret), parmErr)
		}
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
	ret, _, _ := procNetLocalGroupAddMembers.Call(
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

// SetTempPassword temporarily assigns a random password to an existing Windows
// account. This is the workaround for passwordless accounts: RDP requires a
// non-empty password, so we set one for the duration of the session.
// The returned cleanup function resets the password to empty (restoring the
// passwordless state); it is safe to call more than once.
func SetTempPassword(username string) (password string, cleanup func(), err error) {
	password, err = generatePassword()
	if err != nil {
		return "", func() {}, fmt.Errorf("generate temp password: %w", err)
	}
	if err = setUserPassword(username, password); err != nil {
		return "", func() {}, fmt.Errorf("set temp password: %w", err)
	}
	var once sync.Once
	cleanup = func() {
		once.Do(func() {
			if err := setUserPassword(username, ""); err != nil {
				log.Printf("broker: failed to restore empty password for %q: %v", username, err)
			}
		})
	}
	return password, cleanup, nil
}

// setUserPassword calls NetUserSetInfo(level=1003) to update the password for
// an existing local account. An empty password string clears the password.
func setUserPassword(username, password string) error {
	namePtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	passPtr, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return err
	}
	info := userInfo1003{Password: passPtr}
	var parmErr uint32
	ret, _, _ := procNetUserSetInfo.Call(
		0,
		uintptr(unsafe.Pointer(namePtr)),
		1003,
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Pointer(&parmErr)),
	)
	if ret != 0 {
		return fmt.Errorf("NetUserSetInfo failed: %w", windows.Errno(ret))
	}
	return nil
}

// ImpersonateUser calls LogonUserW to obtain an interactive logon token for
// the given local account. The caller MUST close the returned token with
// token.Close() when it is no longer needed to prevent a handle leak.
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
