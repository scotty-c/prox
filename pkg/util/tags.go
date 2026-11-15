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

// SplitTags turns a Proxmox tag list ("prod;web") into its tags. It is never
// nil, so it prints as [] rather than null in JSON.
func SplitTags(tags string) []string {
	list := []string{}
	for _, t := range strings.Split(tags, ";") {
		if t = strings.TrimSpace(t); t != "" {
			list = append(list, t)
		}
	}
	return list
}
