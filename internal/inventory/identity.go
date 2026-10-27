package inventory

import (
	"errors"
	"strconv"
	"strings"
)

// OwnerUID reads the current owner rather than trusting a previous listener scan.
func (inspector Inspector) OwnerUID(pid int) (int, error) {
	if pid <= 1 {
		return 0, errors.New("Invalid process identity.")
	}

	output, err := inspector.Run(utility("ps", "/bin/ps"), "-p", strconv.Itoa(pid), "-o", "uid=")
	if err != nil {
		return 0, err
	}

	uid, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil || uid < 0 {
		return 0, errors.New("Cannot verify the process owner.")
	}

	return uid, nil
}
