package cmdutil

import (
	"fmt"
	"strconv"
)

// ParseCloneArgs reads the arguments after the source of a clone command:
// an optional new ID followed by an optional name. A number is taken as the
// ID and anything else as the name, so "web2" and "200 web2" both work. An ID
// of 0 means "use the next free one".
func ParseCloneArgs(rest []string) (newID int, name string, err error) {
	if len(rest) > 0 {
		if n, convErr := strconv.Atoi(rest[0]); convErr == nil {
			if n <= 0 {
				return 0, "", fmt.Errorf("invalid new ID '%s': must be a positive number", rest[0])
			}
			newID, rest = n, rest[1:]
		}
	}
	if len(rest) > 0 {
		name, rest = rest[0], rest[1:]
	}
	if len(rest) > 0 {
		return 0, "", fmt.Errorf("unexpected argument '%s': expected [new_id] [name]", rest[0])
	}
	return newID, name, nil
}
