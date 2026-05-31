//go:build !windows

package broker

import "errors"

func CreateTempUser(username, password string) error {
	return errors.New("temporary user creation is only supported on Windows")
}

func DeleteTempUser(username string) error {
	return nil
}

func AddToRDPGroup(username string) error {
	return errors.New("RDP group management is only supported on Windows")
}

// SetTempPassword is not supported on non-Windows platforms.
func SetTempPassword(username string) (password string, cleanup func(), err error) {
	return "", func() {}, errors.New("passwordless account workaround is only supported on Windows")
}
