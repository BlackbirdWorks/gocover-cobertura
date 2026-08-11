// Package cobertura defines data structures and XML serialization models for Cobertura coverage reports.
package cobertura

import (
	"encoding/xml"
)

// Coverage represents the top-level Cobertura XML report structure.
type Coverage struct {
	XMLName         xml.Name   `xml:"coverage"`
	Version         string     `xml:"version,attr"`
	Sources         []*Source  `xml:"sources>source"`
	Packages        []*Package `xml:"packages>package"`
	Timestamp       int64      `xml:"timestamp,attr"`
	LinesCovered    int64      `xml:"lines-covered,attr"`
	LinesValid      int64      `xml:"lines-valid,attr"`
	BranchesCovered int64      `xml:"branches-covered,attr"`
	BranchesValid   int64      `xml:"branches-valid,attr"`
	LineRate        float32    `xml:"line-rate,attr"`
	BranchRate      float32    `xml:"branch-rate,attr"`
	Complexity      float32    `xml:"complexity,attr"`
}

// Source represents a source code root directory in a Cobertura report.
type Source struct {
	Path string `xml:",chardata"`
}

// Package represents a package containing source classes in a Cobertura report.
type Package struct {
	Name       string   `xml:"name,attr"`
	Classes    []*Class `xml:"classes>class"`
	LineRate   float32  `xml:"line-rate,attr"`
	BranchRate float32  `xml:"branch-rate,attr"`
	Complexity float32  `xml:"complexity,attr"`
}

// Class represents a single source file/type definition in a Cobertura report.
type Class struct {
	Name       string    `xml:"name,attr"`
	Filename   string    `xml:"filename,attr"`
	Methods    []*Method `xml:"methods>method"`
	Lines      Lines     `xml:"lines>line"`
	LineRate   float32   `xml:"line-rate,attr"`
	BranchRate float32   `xml:"branch-rate,attr"`
	Complexity float32   `xml:"complexity,attr"`
}

// Method represents a function or method definition in a Cobertura report.
type Method struct {
	Name       string  `xml:"name,attr"`
	Signature  string  `xml:"signature,attr"`
	Lines      Lines   `xml:"lines>line"`
	LineRate   float32 `xml:"line-rate,attr"`
	BranchRate float32 `xml:"branch-rate,attr"`
	Complexity float32 `xml:"complexity,attr"`
}

// Line represents code execution metrics for a single line number in a Cobertura report.
type Line struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}

// Lines is a slice of Line pointers, with convenience calculation methods.
type Lines []*Line

// HitRate returns a float32 from 0.0 to 1.0 representing what fraction of lines have hits.
func (lines *Lines) HitRate() float32 {
	return float32(lines.NumLinesWithHits()) / float32(len(*lines))
}

// NumLines returns the number of lines.
func (lines *Lines) NumLines() int64 {
	return int64(len(*lines))
}

// NumLinesWithHits returns the number of lines with a hit count > 0.
func (lines *Lines) NumLinesWithHits() int64 {
	var numLinesWithHits int64
	for _, line := range *lines {
		if line.Hits > 0 {
			numLinesWithHits++
		}
	}

	return numLinesWithHits
}

// AddOrUpdateLine adds a line if it is a different line than the last line recorded.
// If it's the same line as the last line recorded then we update the hits down
// if the new hits is less; otherwise just leave it as-is.
func (lines *Lines) AddOrUpdateLine(lineNumber int, hits int64) {
	if len(*lines) > 0 {
		lastLine := (*lines)[len(*lines)-1]
		if lineNumber == lastLine.Number {
			if hits < lastLine.Hits {
				lastLine.Hits = hits
			}

			return
		}
	}
	*lines = append(*lines, &Line{Number: lineNumber, Hits: hits})
}

// HitRate returns a float32 from 0.0 to 1.0 representing what fraction of lines have hits.
func (method Method) HitRate() float32 {
	return method.Lines.HitRate()
}

// NumLines returns the number of lines.
func (method Method) NumLines() int64 {
	return method.Lines.NumLines()
}

// NumLinesWithHits returns the number of lines with a hit count > 0.
func (method Method) NumLinesWithHits() int64 {
	return method.Lines.NumLinesWithHits()
}

// HitRate returns a float32 from 0.0 to 1.0 representing what fraction of lines have hits.
func (class Class) HitRate() float32 {
	return float32(class.NumLinesWithHits()) / float32(class.NumLines())
}

// NumLines returns the number of lines.
func (class Class) NumLines() int64 {
	var numLines int64
	for _, method := range class.Methods {
		numLines += method.NumLines()
	}

	return numLines
}

// NumLinesWithHits returns the number of lines with a hit count > 0.
func (class Class) NumLinesWithHits() int64 {
	var numLinesWithHits int64
	for _, method := range class.Methods {
		numLinesWithHits += method.NumLinesWithHits()
	}

	return numLinesWithHits
}

// HitRate returns a float32 from 0.0 to 1.0 representing what fraction of lines have hits.
func (pkg Package) HitRate() float32 {
	return float32(pkg.NumLinesWithHits()) / float32(pkg.NumLines())
}

// NumLines returns the number of lines.
func (pkg Package) NumLines() int64 {
	var numLines int64
	for _, class := range pkg.Classes {
		numLines += class.NumLines()
	}

	return numLines
}

// NumLinesWithHits returns the number of lines with a hit count > 0.
func (pkg Package) NumLinesWithHits() int64 {
	var numLinesWithHits int64
	for _, class := range pkg.Classes {
		numLinesWithHits += class.NumLinesWithHits()
	}

	return numLinesWithHits
}

// HitRate returns a float32 from 0.0 to 1.0 representing what fraction of lines have hits.
func (cov *Coverage) HitRate() float32 {
	return float32(cov.NumLinesWithHits()) / float32(cov.NumLines())
}

// NumLines returns the number of lines.
func (cov *Coverage) NumLines() int64 {
	var numLines int64
	for _, pkg := range cov.Packages {
		numLines += pkg.NumLines()
	}

	return numLines
}

// NumLinesWithHits returns the number of lines with a hit count > 0.
func (cov *Coverage) NumLinesWithHits() int64 {
	var numLinesWithHits int64
	for _, pkg := range cov.Packages {
		numLinesWithHits += pkg.NumLinesWithHits()
	}

	return numLinesWithHits
}
