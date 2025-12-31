// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package helpers provides utility functions for Epinio, including file ignore pattern
// matching similar to .gitignore. The ignore system supports:
//
//   - Pattern matching with wildcards (*, **)
//   - Directory-only patterns (ending with /)
//   - Negation patterns (starting with !)
//   - Root-relative patterns (starting with /)
//   - Comments (lines starting with #)
//
// Patterns can be specified in:
//   - .epinioignore file in the application directory
//   - epinio.yaml manifest file under the ignore field
//
// When both are present, patterns from both sources are merged, with manifest
// patterns processed first, followed by .epinioignore patterns (so .epinioignore
// patterns can override manifest patterns if needed).
//
// Example .epinioignore:
//
//	node_modules/
//	*.log
//	dist/
//	.env
//	# This is a comment
//	!important.log
//
// Example epinio.yaml:
//
//	name: myapp
//	configuration:
//	  ignore:
//	    - node_modules/
//	    - "*.log"
package helpers

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// epinioIgnoreFile is defined in tar.go

// IgnoreMatcher holds patterns to match against file paths.
// Patterns are processed in order, with later negation patterns able to override
// earlier ignore patterns.
type IgnoreMatcher struct {
	patterns []ignorePattern
}

type ignorePattern struct {
	pattern    string
	isNegation bool // patterns starting with ! negate previous matches
	isDir      bool // pattern ending with / matches only directories
}

// LoadIgnoreMatcher loads ignore patterns from .epinioignore file if it exists.
// If manifestPatterns is provided, those patterns are merged in first (before file patterns).
func LoadIgnoreMatcher(dir string, manifestPatterns []string) (*IgnoreMatcher, error) {
	matcher := &IgnoreMatcher{
		patterns: []ignorePattern{},
	}

	// First, add patterns from manifest (if any)
	for _, pattern := range manifestPatterns {
		parsed := parsePattern(pattern)
		if parsed != nil {
			matcher.patterns = append(matcher.patterns, *parsed)
		}
	}

	// Then, load patterns from .epinioignore file (if it exists)
	ignorePath := filepath.Join(dir, ".epinioignore")
	file, err := os.Open(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No .epinioignore file, return matcher with just manifest patterns
			return matcher, nil
		}
		return nil, errors.Wrap(err, "failed to open .epinioignore file")
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parsed := parsePattern(line)
		if parsed != nil {
			matcher.patterns = append(matcher.patterns, *parsed)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to read .epinioignore file")
	}

	return matcher, nil
}

// parsePattern parses a single ignore pattern line.
// It handles trimming, comments, negation, and directory markers.
// Returns nil if the line should be skipped (empty or comment).
func parsePattern(line string) *ignorePattern {
	// Trim whitespace first (before checking negation/directory)
	line = strings.TrimSpace(line)

	// Skip empty lines and comments
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}

	// Check for negation pattern (starts with !)
	isNegation := strings.HasPrefix(line, "!")
	if isNegation {
		line = strings.TrimPrefix(line, "!")
		line = strings.TrimSpace(line) // Trim again after removing !
		if line == "" {
			return nil
		}
	}

	// Check if pattern is directory-only (ends with /)
	isDir := strings.HasSuffix(line, "/")
	if isDir {
		line = strings.TrimSuffix(line, "/")
		line = strings.TrimSpace(line) // Trim again after removing /
		if line == "" {
			return nil
		}
	}

	return &ignorePattern{
		pattern:    line,
		isNegation: isNegation,
		isDir:      isDir,
	}
}

// ShouldIgnore checks if a file or directory path should be ignored.
// baseDir is the root directory of the application.
// filePath is the full path to the file or directory.
// isDir indicates if the path is a directory.
//
// The method processes patterns in order, tracking which patterns matched.
// Negation patterns only un-ignore if the path was previously ignored by a non-negation pattern.
func (m *IgnoreMatcher) ShouldIgnore(baseDir, filePath string, isDir bool) bool {
	// Get relative path from baseDir
	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		// If we can't get relative path, don't ignore
		return false
	}

	// Normalize path separators to forward slashes (like gitignore)
	relPath = filepath.ToSlash(relPath)

	// If it's the root directory, don't ignore
	if relPath == "." || relPath == "" {
		return false
	}

	// Track if this path should be ignored and which patterns matched
	ignored := false
	matchedByNonNegation := false // Track if any non-negation pattern matched

	// Check each pattern in order
	for _, pattern := range m.patterns {
		matched := m.matchesPattern(pattern.pattern, relPath, isDir, pattern.isDir)

		if matched {
			if pattern.isNegation {
				// Negation pattern - only un-ignore if path was previously ignored
				if matchedByNonNegation {
					ignored = false
					// Reset the flag since we've un-ignored
					matchedByNonNegation = false
				}
			} else {
				// Regular pattern - ignore this path
				ignored = true
				matchedByNonNegation = true
			}
		}
	}

	return ignored
}

// matchesPattern checks if a path matches a gitignore-style pattern.
func (m *IgnoreMatcher) matchesPattern(pattern, relPath string, pathIsDir, patternIsDir bool) bool {
	// If pattern is directory-only but path is not a directory, don't match
	if patternIsDir && !pathIsDir {
		return false
	}

	// Normalize pattern separators
	pattern = filepath.ToSlash(pattern)

	// Handle patterns starting with / (root-relative)
	if strings.HasPrefix(pattern, "/") {
		pattern = strings.TrimPrefix(pattern, "/")
		// Match only at the beginning of the path
		return m.matchAtStart(pattern, relPath)
	}

	// Handle patterns with ** (match any number of directories)
	if strings.Contains(pattern, "**") {
		return m.matchGlobPattern(pattern, relPath)
	}

	// For patterns without leading slash, match anywhere in the path
	pathParts := strings.Split(relPath, "/")
	patternParts := strings.Split(pattern, "/")

	// Simple case: single pattern part (e.g., "*.log", "node_modules")
	if len(patternParts) == 1 {
		patternPart := patternParts[0]
		for _, pathPart := range pathParts {
			if m.matchPart(patternPart, pathPart) {
				return true
			}
		}
		return false
	}

	// Multi-part pattern: try to match from each position in the path
	for i := 0; i <= len(pathParts)-len(patternParts); i++ {
		matched := true
		for j, patternPart := range patternParts {
			if i+j >= len(pathParts) {
				matched = false
				break
			}
			if !m.matchPart(patternPart, pathParts[i+j]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	return false
}

// matchAtStart matches a pattern against the start of a path.
func (m *IgnoreMatcher) matchAtStart(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	// Check if path starts with pattern followed by /
	if strings.HasPrefix(path, pattern+"/") {
		return true
	}

	// Use filepath.Match for glob patterns at the start
	matched, err := filepath.Match(pattern, path)
	if err == nil && matched {
		return true
	}

	// Also check if first path segment matches
	pathParts := strings.Split(path, "/")
	if len(pathParts) > 0 {
		matched, err := filepath.Match(pattern, pathParts[0])
		if err == nil && matched {
			return true
		}
	}

	return false
}

// matchGlobPattern handles patterns with ** (recursive matching).
// It properly handles complex patterns like:
//   - a/**/b/**/c
//   - **/test/**
//   - src/**/*.js
func (m *IgnoreMatcher) matchGlobPattern(pattern, path string) bool {
	// Normalize separators
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Convert pattern to a regex-like matching by handling ** segments
	// Split by ** to get pattern segments
	segments := strings.Split(pattern, "**")
	
	// If no ** found, fall back to regular matching
	if len(segments) == 1 {
		return m.matchPatternAnywhere(pattern, path)
	}

	// Handle patterns with **
	// We need to match each segment in order, with ** allowing any number of path segments between
	
	// Special case: pattern starts with **/
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		// Match suffix anywhere in path
		return m.matchPatternAnywhere(suffix, path)
	}

	// Special case: pattern ends with /**
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		// Match prefix at start of path
		return strings.HasPrefix(path, prefix+"/") || path == prefix
	}

	// General case: pattern has ** in the middle or multiple **
	// Build a matching function that handles ** segments
	return m.matchWithDoubleStar(segments, path)
}

// matchPatternAnywhere checks if a pattern matches anywhere in a path.
func (m *IgnoreMatcher) matchPatternAnywhere(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	// Check if pattern matches any segment
	pathParts := strings.Split(path, "/")
	patternParts := strings.Split(pattern, "/")

	if len(patternParts) == 1 {
		// Single pattern part - check all path parts
		for _, part := range pathParts {
			matched, err := filepath.Match(pattern, part)
			if err == nil && matched {
				return true
			}
		}
		return false
	}

	// Multi-part pattern - try matching at each position
	for i := 0; i <= len(pathParts)-len(patternParts); i++ {
		matched := true
		for j, patternPart := range patternParts {
			if i+j >= len(pathParts) {
				matched = false
				break
			}
			partMatched, err := filepath.Match(patternPart, pathParts[i+j])
			if err != nil || !partMatched {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	return false
}

// matchWithDoubleStar matches a pattern split by ** segments against a path.
// Each segment must match in order, with ** allowing any number of path segments between.
func (m *IgnoreMatcher) matchWithDoubleStar(segments []string, path string) bool {
	pathParts := strings.Split(path, "/")
	
	// Remove empty segments (from patterns like "a/**/**/b")
	nonEmptySegments := make([]string, 0, len(segments))
	for _, seg := range segments {
		seg = strings.Trim(seg, "/")
		if seg != "" {
			nonEmptySegments = append(nonEmptySegments, seg)
		}
	}

	if len(nonEmptySegments) == 0 {
		return true // Pattern is just **, matches everything
	}

	// Match segments in order using recursive backtracking
	return m.matchSegmentsRecursive(nonEmptySegments, 0, pathParts, 0)
}

// matchSegmentsRecursive recursively matches pattern segments against path parts.
// segIndex: current segment index
// pathIndex: current path part index
func (m *IgnoreMatcher) matchSegmentsRecursive(segments []string, segIndex int, pathParts []string, pathIndex int) bool {
	// If we've matched all segments, success
	if segIndex >= len(segments) {
		return true
	}

	// If we've run out of path parts but still have segments, failure
	if pathIndex >= len(pathParts) {
		return false
	}

	segment := segments[segIndex]
	segmentParts := strings.Split(segment, "/")
	isLastSegment := segIndex == len(segments)-1

	// Try to match this segment starting at each possible pathIndex position
	// (allowing ** to skip any number of path parts)
	for startPathIndex := pathIndex; startPathIndex <= len(pathParts); startPathIndex++ {
		// For the last segment, we can match it against any remaining path part
		if isLastSegment && len(segmentParts) == 1 {
			// Single pattern part at the end - try matching against any remaining path part
			for i := startPathIndex; i < len(pathParts); i++ {
				matched, err := filepath.Match(segmentParts[0], pathParts[i])
				if err == nil && matched {
					return true // Matched the last segment, all done
				}
			}
			return false // Couldn't match last segment
		}

		// Try matching the segment starting at startPathIndex
		if startPathIndex+len(segmentParts) <= len(pathParts) {
			segMatched := true
			for j, segPart := range segmentParts {
				partMatched, err := filepath.Match(segPart, pathParts[startPathIndex+j])
				if err != nil || !partMatched {
					segMatched = false
					break
				}
			}
			
			if segMatched {
				// This segment matched, try matching the next segment
				nextPathIndex := startPathIndex + len(segmentParts)
				if m.matchSegmentsRecursive(segments, segIndex+1, pathParts, nextPathIndex) {
					return true
				}
				// If next segment didn't match, continue trying other positions for this segment
			}
		}
	}

	return false
}

// matchPart matches a single pattern part against a path part.
func (m *IgnoreMatcher) matchPart(pattern, part string) bool {
	// Exact match
	if pattern == part {
		return true
	}

	// Use filepath.Match for glob patterns
	matched, err := filepath.Match(pattern, part)
	return err == nil && matched
}
