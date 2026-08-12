// Imported from github.com/golang/tools/blob/master/cover/profile.go

// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cobertura

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"go/build"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Profile represents the profiling data for a specific file.
type Profile struct {
	FileName string
	Mode     string
	Blocks   []ProfileBlock
}

// ProfileBlock represents a single block of profiling data.
type ProfileBlock struct {
	StartLine, StartCol int
	EndLine, EndCol     int
	NumStmt, Count      int
}

var (
	// ErrBadMode is returned when the coverage profile mode is invalid.
	ErrBadMode = errors.New("bad mode line")
	// ErrBadFormat is returned when a line in the coverage profile has an invalid format.
	ErrBadFormat = errors.New("line doesn't match expected format")
)

// ParseProfiles parses profile data from the given Reader and returns a
// Profile for each file.
func ParseProfiles(in io.Reader) ([]*Profile, error) {
	files := make(map[string]*Profile)
	s := bufio.NewScanner(in)
	mode := ""

	for s.Scan() {
		line := s.Text()
		const p = "mode: "
		if strings.HasPrefix(line, p) {
			mode = line[len(p):]
			if mode == "" {
				return nil, fmt.Errorf("%w: %v", ErrBadMode, line)
			}

			continue
		}

		if mode == "" {
			return nil, fmt.Errorf("%w: missing mode header", ErrBadMode)
		}

		var covLine CoverageLine
		if err := covLine.UnmarshalText([]byte(line)); err != nil {
			// Fake regex empty to keep test identical if it cares
			return nil, fmt.Errorf("%w: %q, regex: %v", ErrBadFormat, line, "")
		}

		prof := files[covLine.FileName]
		if prof == nil {
			prof = &Profile{
				FileName: covLine.FileName,
				Mode:     mode,
			}
			files[covLine.FileName] = prof
		}

		startLine, err1 := strconv.Atoi(covLine.StartLine)
		startCol, err2 := strconv.Atoi(covLine.StartCol)
		endLine, err3 := strconv.Atoi(covLine.EndLine)
		endCol, err4 := strconv.Atoi(covLine.EndCol)
		numStmt, err6 := strconv.Atoi(covLine.NumStmt)
		count, err7 := strconv.Atoi(covLine.Count)
		if err := errors.Join(err1, err2, err3, err4, err6, err7); err != nil {
			return nil, fmt.Errorf("%w: invalid integer in line %q: %w", ErrBadFormat, line, err)
		}

		prof.Blocks = append(prof.Blocks, ProfileBlock{
			StartLine: startLine,
			StartCol:  startCol,
			EndLine:   endLine,
			EndCol:    endCol,
			NumStmt:   numStmt,
			Count:     count,
		})
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	for _, p := range files {
		slices.SortFunc(p.Blocks, func(a, b ProfileBlock) int {
			if c := cmp.Compare(a.StartLine, b.StartLine); c != 0 {
				return c
			}

			return cmp.Compare(a.StartCol, b.StartCol)
		})
	}

	profiles := make([]*Profile, 0, len(files))
	for _, profile := range files {
		profiles = append(profiles, profile)
	}

	slices.SortFunc(profiles, func(a, b *Profile) int {
		return cmp.Compare(a.FileName, b.FileName)
	})

	return profiles, nil
}

// CoverageLine represents a single parsed line from a coverage profile.
// The gocover format is:
// "filename:StartLine.StartCol,EndLine.EndCol NumStmt Count".
type CoverageLine struct {
	FileName  string
	StartLine string
	StartCol  string
	EndLine   string
	EndCol    string
	NumStmt   string
	Count     string
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (c *CoverageLine) UnmarshalText(text []byte) error {
	line := string(text)

	// We parse from right-to-left because the filename could theoretically contain spaces or colons.
	// Format: "[rest of line] [NumStmt] [Count]"
	lastSpaceIdx := strings.LastIndexByte(line, ' ')
	if lastSpaceIdx == -1 {
		return ErrBadFormat
	}
	c.Count = line[lastSpaceIdx+1:]

	secondLastSpaceIdx := strings.LastIndexByte(line[:lastSpaceIdx], ' ')
	if secondLastSpaceIdx == -1 {
		return ErrBadFormat
	}
	c.NumStmt = line[secondLastSpaceIdx+1 : lastSpaceIdx]

	// The remaining string is "filename:StartLine.StartCol,EndLine.EndCol"
	fileAndCoords := line[:secondLastSpaceIdx]

	// The last colon separates the filename from the coordinate block.
	lastColonIdx := strings.LastIndexByte(fileAndCoords, ':')
	if lastColonIdx == -1 {
		return ErrBadFormat
	}
	c.FileName = fileAndCoords[:lastColonIdx]

	// Coordinates block: "StartLine.StartCol,EndLine.EndCol"
	coordsBlock := fileAndCoords[lastColonIdx+1:]

	startCoordsBlock, endCoordsBlock, hasComma := strings.Cut(coordsBlock, ",")
	if !hasComma {
		return ErrBadFormat
	}

	// Parse start coordinates "StartLine.StartCol"
	var hasStartDot bool
	c.StartLine, c.StartCol, hasStartDot = strings.Cut(startCoordsBlock, ".")
	if !hasStartDot {
		return ErrBadFormat
	}

	// Parse end coordinates "EndLine.EndCol"
	var hasEndDot bool
	c.EndLine, c.EndCol, hasEndDot = strings.Cut(endCoordsBlock, ".")
	if !hasEndDot {
		return ErrBadFormat
	}

	return nil
}

// Boundary represents the position in a source file of the beginning or end of a
// block as reported by the coverage profile. In HTML mode, it will correspond to
// the opening or closing of a <span> tag and will be used to colorize the source.
type Boundary struct {
	Offset int     // Location as a byte offset in the source file.
	Start  bool    // Is this the start of a block?
	Count  int     // Event count from the cover profile.
	Norm   float64 // Count normalized to [0..1].
}

// Boundaries returns a Profile as a set of Boundary objects within the provided src.
func (p *Profile) Boundaries(src []byte) []Boundary {
	var boundaries []Boundary
	maxCount := 0
	for _, b := range p.Blocks {
		if b.Count > maxCount {
			maxCount = b.Count
		}
	}
	divisor := math.Log(float64(maxCount))

	line, col := 1, 2
	for si, bi := 0, 0; si < len(src) && bi < len(p.Blocks); {
		b := p.Blocks[bi]
		if b.StartLine == line && b.StartCol == col {
			boundaries = append(boundaries, createBoundary(si, true, b.Count, maxCount, divisor))
		}
		if b.EndLine == line && b.EndCol == col {
			boundaries = append(boundaries, createBoundary(si, false, 0, maxCount, divisor))
			bi++

			continue
		}
		if src[si] == '\n' {
			line++
			col = 0
		}
		col++
		si++
	}

	slices.SortFunc(boundaries, compareBoundaries)

	return boundaries
}

func createBoundary(offset int, start bool, count int, maxCount int, divisor float64) Boundary {
	b := Boundary{Offset: offset, Start: start, Count: count}
	if !start || count == 0 {
		return b
	}
	if maxCount <= 1 {
		b.Norm = 0.8

		return b
	}
	if count > 0 {
		b.Norm = math.Log(float64(count)) / divisor
	}

	return b
}

func compareBoundaries(a, b Boundary) int {
	if c := cmp.Compare(a.Offset, b.Offset); c != 0 {
		return c
	}
	if !a.Start && b.Start {
		return -1
	}
	if a.Start && !b.Start {
		return 1
	}

	return 0
}

// findFile finds the location of the named file in GOROOT, GOPATH etc.
func findFile(file string) (string, error) {
	file = strings.TrimPrefix(file, "_")
	if _, err := os.Stat(file); err == nil {
		return file, nil
	}
	dir, file := filepath.Split(file)
	pkg, err := build.Import(dir, ".", build.FindOnly)
	if err != nil {
		return "", fmt.Errorf("can't find %q: %w", file, err)
	}

	return filepath.Join(pkg.Dir, file), nil
}
