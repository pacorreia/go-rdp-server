//go:build !windows

package broker

import "errors"

func CreateTempUser(username, password string) error {
	return errors.New("temporary user creation is only supported on windows")
}

func DeleteTempUser(username string) error {
	return nil
}

func AddToRDPGroup(username string) error {
	return errors.New("rdp group management is only supported on windows")
}
