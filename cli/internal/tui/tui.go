// Package tui provides terminal UI components for yapi.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"yapi.run/cli/internal/tui/selector"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/mattn/go-isatty"
)

// getTTY returns input and output file handles for interactive TUI.
// On Unix, it tries /dev/tty first to work when stdout is piped.
// On Windows, it uses stdin/stdout directly.
// Returns nil, nil if no TTY is available.
func getTTY() (in, out *os.File, cleanup func()) {
	cleanup = func() {} // no-op by default

	// On Unix, try /dev/tty for piped scenarios
	if runtime.GOOS != "windows" {
		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err == nil {
			return tty, tty, func() { _ = tty.Close() }
		}
	}

	// Fall back to stdin/stdout if they're terminals
	if isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) {
		return os.Stdin, os.Stdout, cleanup
	}

	// Also check for Cygwin/MSYS terminals on Windows
	if isatty.IsCygwinTerminal(os.Stdin.Fd()) && isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return os.Stdin, os.Stdout, cleanup
	}

	return nil, nil, cleanup
}

// yapiFilePattern matches *.yapi, *.yapi.yaml or *.yapi.yml in subdirectories only
var yapiFilePattern = regexp.MustCompile(`^.+/.+\.yapi(\.ya?ml)?$`)

// FindConfigFiles returns all non-gitignored yapi config files relative to the current directory
func FindConfigFiles() ([]string, error) {
	return findFiles(false)
}

func findFiles(includeProjectConfig bool) ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	var configFiles []string
	err = findFilesInRepo(cwd, cwd, includeProjectConfig, &configFiles)
	if err != nil {
		return nil, err
	}

	if len(configFiles) == 0 {
		return nil, fmt.Errorf("no .yapi/.yapi.yaml/.yapi.yml files found in subdirectories")
	}

	sort.Strings(configFiles)
	return configFiles, nil
}

// findFilesInRepo walks a directory, respecting gitignore rules and handling submodules.
func findFilesInRepo(dir, searchRoot string, includeProjectConfig bool, results *[]string) error {
	// Open the git repository from this directory
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	repoRoot := wt.Filesystem.Root()

	// Read gitignore patterns for this repo
	patterns, err := gitignore.ReadPatterns(wt.Filesystem, nil)
	if err != nil {
		patterns = nil
	}
	matcher := gitignore.NewMatcher(patterns)

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible files, continue walking
		}

		// Get path relative to repo root for gitignore matching
		relToRepo, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return nil //nolint:nilerr // skip files with path errors
		}

		pathComponents := strings.Split(filepath.ToSlash(relToRepo), "/")

		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}

			// Check if this is a submodule (has its own .git)
			if path != dir {
				gitPath := filepath.Join(path, ".git")
				if _, err := os.Stat(gitPath); err == nil {
					// This is a submodule - recurse with new repo context
					_ = findFilesInRepo(path, searchRoot, includeProjectConfig, results)
					return filepath.SkipDir
				}
			}

			// Check if directory is gitignored
			if matcher.Match(pathComponents, true) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file is gitignored
		if matcher.Match(pathComponents, false) {
			return nil
		}

		// Get path relative to search root for display
		relPath, err := filepath.Rel(searchRoot, path)
		if err != nil {
			return nil //nolint:nilerr // skip files with path errors
		}

		// Match .yapi.yml files
		base := filepath.Base(relPath)
		if yapiFilePattern.MatchString(relPath) {
			*results = append(*results, relPath)
		} else if includeProjectConfig && (base == "yapi.config.yml" || base == "yapi.config.yaml") {
			*results = append(*results, relPath)
		}

		return nil
	})
}

// FindConfigFileSingle prompts the user to select a single config file.
// Only shows .yapi/.yapi.yml/.yapi.yaml files, not project config files.
func FindConfigFileSingle() (string, error) {
	return findConfigFileSingle(false)
}

// FindConfigFileSingleIncludingProject prompts the user to select a single config file.
// Shows both .yapi/.yapi.yml/.yapi.yaml files and project config files (yapi.config.yml).
func FindConfigFileSingleIncludingProject() (string, error) {
	return findConfigFileSingle(true)
}

func findConfigFileSingle(includeProjectConfig bool) (string, error) {
	files, err := findFiles(includeProjectConfig)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no .yapi/.yapi.yml/.yapi.yaml files found")
	}

	in, out, cleanup := getTTY()
	defer cleanup()

	if in == nil || out == nil {
		// No TTY at all (CI, cron, etc) -> non-interactive fallback
		return files[0], nil
	}

	// Render TUI to the chosen terminal, not to stdout.
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(out))

	p := tea.NewProgram(
		selector.New(files, false),
		tea.WithInput(in),
		tea.WithOutput(out),
		tea.WithAltScreen(),
	)

	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run selector: %w", err)
	}

	model := m.(selector.Model)
	selected := model.SelectedList()
	if len(selected) == 0 {
		return "", fmt.Errorf("no config file selected")
	}

	// The caller still prints the final path(s) to stdout,
	// which can safely be piped to jq, xargs, etc.
	return selected[0], nil
}
