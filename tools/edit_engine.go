package tools

import (
	"errors"
	"fmt"
	"strings"
)

const maxFileSize = 1024 * 1024

var (
	errNoMatch         = fmt.Errorf("search text not found in file")
	errMultipleMatches = fmt.Errorf("search text matches multiple locations in file")
	errEmptySearch     = fmt.Errorf("search text must not be empty")
	errFileSizeLimit   = fmt.Errorf("file size exceeds 1MB limit")
)

type EditEngine struct{}

func NewEditEngine() *EditEngine {
	return &EditEngine{}
}

func (e *EditEngine) Apply(content, search, replace string) (string, error) {
	if search == "" {
		return "", errEmptySearch
	}

	count := strings.Count(content, search)
	if count == 0 {
		return "", errNoMatch
	}
	if count > 1 {
		return "", errMultipleMatches
	}

	result := strings.Replace(content, search, replace, 1)

	if len(result) > maxFileSize {
		return "", errFileSizeLimit
	}

	return result, nil
}

func (e *EditEngine) FindMatchCount(content, search string) int {
	return strings.Count(content, search)
}

func IsNoMatch(err error) bool {
	return errors.Is(err, errNoMatch)
}

func IsMultipleMatches(err error) bool {
	return errors.Is(err, errMultipleMatches)
}

func IsEmptySearch(err error) bool {
	return errors.Is(err, errEmptySearch)
}

func IsFileSizeLimit(err error) bool {
	return errors.Is(err, errFileSizeLimit)
}
