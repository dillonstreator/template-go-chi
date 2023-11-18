package main

import "fmt"

func errWrap(err error, msg string) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", msg, err)
}

func errWrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	return errWrap(err, fmt.Sprintf(format, args...))
}
