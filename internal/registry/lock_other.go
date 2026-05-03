//go:build windows

package registry

import "errors"

func lockRegistry(_ string, _ bool) (func(), error) {
	return nil, errors.New("sessions registry locking is not supported on Windows in v0")
}

func syncDir(_ string) error {
	return nil
}
