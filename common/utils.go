package common

import "regexp"

func CheckVideoRegex(videoTitle string, filters []string) bool {
	for _, filter := range filters {
		if regexp.MustCompile(filter).MatchString(videoTitle) {
			return true
		}
	}
	return false
}
