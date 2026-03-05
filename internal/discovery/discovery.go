package discovery

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geraldc/workspace-cli/internal/config"
)

type AmbiguousMatchError struct {
	Query   string
	Matches []string
}

func (e *AmbiguousMatchError) Error() string {
	return fmt.Sprintf("multiple repos matched %q: %s", e.Query, strings.Join(e.Matches, ", "))
}

func FindRepos(cfg config.Config, refresh bool) ([]string, error) {
	cachePath := config.CacheFilePath()
	if !refresh {
		if repos, ok := readFreshCache(cachePath, cfg.CacheAgeSeconds); ok {
			return repos, nil
		}
	}

	repos := make([]string, 0, 256)
	for _, root := range cfg.RepoRoots {
		rootInfo, err := os.Stat(root)
		if err != nil || !rootInfo.IsDir() {
			continue
		}
		baseDepth := depth(root)
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			if depth(path)-baseDepth > cfg.MaxDepth {
				return filepath.SkipDir
			}
			if d.Name() == ".git" {
				repos = append(repos, filepath.Dir(path))
				return filepath.SkipDir
			}
			return nil
		})
	}

	sort.Strings(repos)
	repos = unique(repos)
	_ = writeCache(cachePath, repos)
	return repos, nil
}

func MatchQuery(repos []string, query string) (string, error) {
	if len(repos) == 0 {
		return "", errors.New("no repositories available")
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return "", errors.New("empty query")
	}

	exact := ""
	for _, repo := range repos {
		if strings.ToLower(filepath.Base(repo)) == query {
			if exact != "" {
				return "", &AmbiguousMatchError{Query: query, Matches: []string{exact, repo}}
			}
			exact = repo
		}
	}
	if exact != "" {
		return exact, nil
	}

	matches := make([]string, 0, 8)
	for _, repo := range repos {
		base := strings.ToLower(filepath.Base(repo))
		full := strings.ToLower(repo)
		if strings.Contains(base, query) || strings.Contains(full, query) {
			matches = append(matches, repo)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no repos matched %q", query)
	}
	if len(matches) > 1 {
		return "", &AmbiguousMatchError{Query: query, Matches: matches}
	}
	return matches[0], nil
}

func readFreshCache(path string, ageSeconds int) ([]string, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if time.Since(st.ModTime()) > time.Duration(ageSeconds)*time.Second {
		return nil, false
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	out := make([]string, 0, 256)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func writeCache(path string, repos []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data := strings.Join(repos, "\n")
	if data != "" {
		data += "\n"
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

func depth(path string) int {
	cleaned := filepath.Clean(path)
	if cleaned == string(filepath.Separator) {
		return 1
	}
	return len(strings.Split(cleaned, string(filepath.Separator)))
}

func unique(in []string) []string {
	if len(in) < 2 {
		return in
	}
	out := make([]string, 0, len(in))
	last := ""
	for _, item := range in {
		if item == last {
			continue
		}
		last = item
		out = append(out, item)
	}
	return out
}
