package util

import "strings"

// HasTag reports whether tag is in a Proxmox tag list, which is a
// semicolon-separated string such as "prod;web"
func HasTag(tags, tag string) bool {
	for _, t := range strings.Split(tags, ";") {
		if strings.TrimSpace(t) == tag {
			return tag != ""
		}
	}
	return false
}
