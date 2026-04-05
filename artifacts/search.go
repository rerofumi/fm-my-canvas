package artifacts

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxSearchResults = 50
const maxSearchFileSize = 1 * 1024 * 1024

type SearchResult struct {
	File    string
	Line    int
	Content string
}

func (m *Manager) SearchFiles(sessionID, pattern string, filePattern string) ([]SearchResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	if filePattern != "" && strings.ContainsAny(filePattern, "/\\") {
		return nil, fmt.Errorf("file_pattern must not contain path separators: %q", filePattern)
	}

	dir := m.WorkspaceDir(sessionID)
	evalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate workspace symlinks: %w", err)
	}

	var results []SearchResult
	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		evalPath, lerr := filepath.EvalSymlinks(path)
		if lerr != nil {
			return nil
		}
		if !strings.HasPrefix(evalPath, evalDir+string(os.PathSeparator)) {
			return nil
		}

		if filePattern != "" {
			matched, _ := filepath.Match(filePattern, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		if info.Size() > maxSearchFileSize {
			return nil
		}

		f, oerr := os.Open(path)
		if oerr != nil {
			return nil
		}
		defer f.Close()

		buf := make([]byte, 512)
		n, rer := f.Read(buf)
		if rer != nil && n == 0 {
			return nil
		}
		if strings.Contains(string(buf[:n]), "\x00") {
			return nil
		}

		if _, serr := f.Seek(0, 0); serr != nil {
			return nil
		}

		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return nil
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				results = append(results, SearchResult{
					File:    rel,
					Line:    lineNum,
					Content: line,
				})
				if len(results) >= maxSearchResults {
					return nil
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].File != results[j].File {
			return results[i].File < results[j].File
		}
		return results[i].Line < results[j].Line
	})

	return results, nil
}
