package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileManager builds destination paths and writes media files for downloaded
// awemes. Ported from storage/file_manager.py.
//
// The Python original supports many author-directory styles and a streaming
// download with HTTP fallback; this Go port covers the path-building and local
// I/O surface (BuildPath, EnsureDir, WriteFile, FileExists) that the task
// requires. The file-naming convention is {author}/{aweme_id}_{type}.{ext}.
type FileManager struct {
	basePath string
}

// NewFileManager constructs a FileManager rooted at baseDir. The base
// directory is created (parents=true) if it does not exist.
func NewFileManager(baseDir string) (*FileManager, error) {
	if baseDir == "" {
		baseDir = "./Downloaded"
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create base dir %s: %w", baseDir, err)
	}
	return &FileManager{basePath: baseDir}, nil
}

// BasePath returns the configured base download directory.
func (fm *FileManager) BasePath() string {
	return fm.basePath
}

// BuildPathOptions controls how a destination path is composed.
type BuildPathOptions struct {
	AuthorName   string // sanitized into the author directory
	AwemeID      string
	AwemeType    string // e.g. "video", "gallery"
	Ext          string // file extension WITHOUT leading dot, e.g. "mp4"
	Filename     string // when set, overrides the generated leaf filename
}

// BuildPath returns the full destination file path following the convention
// {base}/{author}/{aweme_id}_{type}.{ext}. The author directory and any
// intermediate parents are NOT created here — call EnsureDir for that. When
// opts.Filename is provided it is used verbatim (after sanitization) instead of
// the {aweme_id}_{type} template.
func (fm *FileManager) BuildPath(opts BuildPathOptions) string {
	safeAuthor := SanitizeFilename(opts.AuthorName)
	if safeAuthor == "" {
		safeAuthor = "untitled"
	}

	leaf := opts.Filename
	if leaf == "" {
		leaf = fmt.Sprintf("%s_%s", opts.AwemeID, opts.AwemeType)
	}
	leaf = SanitizeFilename(leaf)
	ext := strings.TrimPrefix(opts.Ext, ".")
	var name string
	if ext != "" {
		name = leaf + "." + ext
	} else {
		name = leaf
	}
	return filepath.Join(fm.basePath, safeAuthor, name)
}

// EnsureDir creates the parent directory of path (or path itself if it has no
// extension) with 0o755 permissions, including any missing parents.
func (fm *FileManager) EnsureDir(path string) error {
	dir := path
	if filepath.Ext(path) != "" {
		dir = filepath.Dir(path)
	}
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// WriteFile writes data to path, creating parent directories as needed. It
// overwrites any existing file. Permissions are 0o644 for the file and 0o755
// for created directories.
func (fm *FileManager) WriteFile(path string, data []byte) error {
	if path == "" {
		return errors.New("WriteFile: empty path")
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return os.WriteFile(path, data, 0o644)
}

// FileExists reports whether path exists and is non-empty (>0 bytes). Mirrors
// the Python file_exists() guard that rejects zero-byte placeholder files.
func (fm *FileManager) FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Size() > 0
}

// ---------------- filename sanitization ----------------

var (
	// illegalCharsRe matches Windows-illegal characters plus control chars,
	// '#', and ',' — same set as utils/validators.sanitize_filename.
	illegalCharsRe = regexp.MustCompile(`[<>:"/\\|?*#\x00-\x1f]`)
	multiUnderscore = regexp.MustCompile(`_+`)
	multiSpace      = regexp.MustCompile(` +`)
)

// windowsReservedStems is the set of Windows reserved filename stems. A
// sanitized name whose first dot-segment matches one of these gets an
// underscore prefix to stay cross-platform safe.
var windowsReservedStems = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// SanitizeFilename mirrors utils/validators.sanitize_filename: collapses
// newlines to spaces, replaces illegal characters with "_", collapses runs of
// underscores and spaces, trims leading/trailing "._- ", truncates to 80
// runes, and prefixes reserved Windows stems with "_". Returns "untitled"
// when the result is empty.
func SanitizeFilename(name string, maxLen ...int) string {
	maxLength := 80
	if len(maxLen) > 0 && maxLen[0] > 0 {
		maxLength = maxLen[0]
	}

	// Newlines → space
	name = strings.ReplaceAll(name, "\n", " ")
	name = strings.ReplaceAll(name, "\r", " ")

	// Illegal chars → underscore
	name = illegalCharsRe.ReplaceAllString(name, "_")
	// Collapse consecutive underscores
	name = multiUnderscore.ReplaceAllString(name, "_")
	// Collapse consecutive spaces
	name = multiSpace.ReplaceAllString(name, " ")
	// Trim leading/trailing
	name = strings.Trim(name, "._- \t")

	if len([]rune(name)) > maxLength {
		runes := []rune(name)[:maxLength]
		name = strings.TrimRight(string(runes), "._- \t")
	}

	// Reserved stem check on first dot segment.
	if idx := strings.Index(name, "."); idx >= 0 {
		stem := name[:idx]
		if windowsReservedStems[strings.ToUpper(stem)] {
			name = "_" + name
			if len([]rune(name)) > maxLength {
				name = string([]rune(name)[:maxLength])
			}
		}
	}

	if name == "" {
		return "untitled"
	}
	return name
}
