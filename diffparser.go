// Copyright (c) 2015 Jesse Meek <https://github.com/waigani>
// This program is Free Software see LICENSE file for details.

package diffparser

import (
	"regexp"
	"strconv"
	"strings"

	"errors"
)

// FileMode represents the file status in a diff
type FileMode int

const (
	// DELETED if the file is deleted
	DELETED FileMode = iota
	// MODIFIED if the file is modified
	MODIFIED
	// NEW if the file is created and there is no diff
	NEW
	// RENAMED if the file is renamed
	RENAMED
)

func (fm FileMode) String() string {
	switch fm {
	case DELETED:
		return "DELETED"
	case MODIFIED:
		return "MODIFIED"
	case NEW:
		return "NEW"
	case RENAMED:
		return "RENAMED"
	default:
		return "UNKNOWN"
	}
}

// DiffRange contains the DiffLine's
type DiffRange struct {
	// starting line number
	Start int

	// the number of lines the change diffHunk applies to
	Length int

	// Each line of the hunk range.
	Lines []*DiffLine
}

// DiffLineMode tells the line if added, removed or unchanged
type DiffLineMode int

const (
	// ADDED if the line is added (shown green in diff)
	ADDED DiffLineMode = iota
	// REMOVED if the line is deleted (shown red in diff)
	REMOVED
	// UNCHANGED if the line is unchanged (not colored in diff)
	UNCHANGED
)

func (dlm DiffLineMode) String() string {
	switch dlm {
	case ADDED:
		return "ADDED"
	case REMOVED:
		return "REMOVED"
	case UNCHANGED:
		return "UNCHANGED"
	default:
		return "UNKNOWN"
	}
}

// DiffLine is the least part of an actual diff
type DiffLine struct {
	Mode     DiffLineMode
	Number   int
	Content  string
	Position int // the line in the diff
}

// DiffHunk is a group of difflines
type DiffHunk struct {
	HunkHeader string
	OrigRange  DiffRange
	NewRange   DiffRange
	WholeRange DiffRange
}

// DiffFile is the sum of diffhunks and holds the changes of the file features
type DiffFile struct {
	DiffHeader string
	Mode       FileMode
	OrigName   string
	NewName    string
	Hunks      []*DiffHunk
}

// Diff is the collection of DiffFiles
type Diff struct {
	Files []*DiffFile
	Raw   string `sql:"type:text"`

	PullID uint `sql:"index"`
}

// Changed returns a map of filename to lines changed in that file. Deleted
// files are ignored.
func (d *Diff) Changed() map[string][]int {
	dFiles := make(map[string][]int)

	for _, f := range d.Files {
		if f.Mode == DELETED {
			continue
		}

		for _, h := range f.Hunks {
			for _, dl := range h.NewRange.Lines {
				if dl.Mode == ADDED { // TODO(waigani) return removed
					dFiles[f.NewName] = append(dFiles[f.NewName], dl.Number)
				}
			}
		}
	}

	return dFiles
}

func lineMode(line string) (*DiffLineMode, error) {
	var m DiffLineMode
	switch line[:1] {
	case " ":
		m = UNCHANGED
	case "+":
		m = ADDED
	case "-":
		m = REMOVED
	default:
		return nil, errors.New("could not parse line mode for line: \"" + line + "\"")
	}
	return &m, nil
}

// Parse takes a diff, such as produced by "git diff", and parses it into a
// Diff struct.
func Parse(diffString string) (*Diff, error) {
	var diff Diff
	diff.Raw = diffString
	lines := strings.Split(diffString, "\n")

	var file *DiffFile
	var hunk *DiffHunk
	var ADDEDCount int
	var REMOVEDCount int
	var inHunk bool

	var diffPosCount int
	var firstHunkInFile bool
	// Parse each line of diff.
	for idx, l := range lines {
		diffPosCount++
		switch {
		case strings.HasPrefix(l, "diff "):
			inHunk = false
			firstHunkInFile = true

			// Start a new file.
			file = &DiffFile{
				Mode: MODIFIED, // default is modified
			}
			diff.Files = append(diff.Files, file)

			// Parse the filenames from the diff line.
			if fields := strings.Fields(l); len(fields) >= 3 {
				from, to := fields[len(fields)-2], fields[len(fields)-1]
				if original, ok := strings.CutPrefix(from, "a/"); ok {
					file.OrigName = original
				}
				if updated, ok := strings.CutPrefix(to, "b/"); ok {
					file.NewName = updated
				}
			}

			header := l
			if len(lines) > idx+3 {
				// FIXME(jedevc): this logic is pretty much entirely broken
				rein := regexp.MustCompile(`^index .+$`)
				remp := regexp.MustCompile(`^(-|\+){3} .+$`)
				index := lines[idx+1]
				if rein.MatchString(index) {
					header = header + "\n" + index
				}
				mp1 := lines[idx+2]
				mp2 := lines[idx+3]
				if remp.MatchString(mp1) && remp.MatchString(mp2) {
					header = header + "\n" + mp1 + "\n" + mp2
				}
			}
			file.DiffHeader = header
		case strings.HasPrefix(l, "deleted file "):
			file.Mode = DELETED
		case strings.HasPrefix(l, "new file "):
			file.Mode = NEW
		case strings.HasPrefix(l, "rename "):
			file.Mode = RENAMED
		case strings.HasPrefix(l, "@@ "):
			if firstHunkInFile {
				diffPosCount = 0
				firstHunkInFile = false
			}

			inHunk = true
			// Start new hunk.
			hunk = &DiffHunk{}
			file.Hunks = append(file.Hunks, hunk)

			// Parse hunk heading for ranges
			re := regexp.MustCompile(`@@ \-(\d+),?(\d+)? \+(\d+),?(\d+)? @@ ?(.+)?`)
			m := re.FindStringSubmatch(l)
			if len(m) < 5 {
				return nil, errors.New("Error parsing line: " + l)
			}
			a, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, err
			}
			b := a
			if len(m[2]) > 0 {
				b, err = strconv.Atoi(m[2])
				if err != nil {
					return nil, err
				}
			}
			c, err := strconv.Atoi(m[3])
			if err != nil {
				return nil, err
			}
			d := c
			if len(m[4]) > 0 {
				d, err = strconv.Atoi(m[4])
				if err != nil {
					return nil, err
				}
			}
			if len(m[5]) > 0 {
				hunk.HunkHeader = m[5]
			}

			// hunk orig range.
			hunk.OrigRange = DiffRange{
				Start:  a,
				Length: b,
			}

			// hunk new range.
			hunk.NewRange = DiffRange{
				Start:  c,
				Length: d,
			}

			// (re)set line counts
			ADDEDCount = hunk.NewRange.Start
			REMOVEDCount = hunk.OrigRange.Start
		case inHunk && isSourceLine(l):
			m, err := lineMode(l)
			if err != nil {
				return nil, err
			}
			line := DiffLine{
				Mode:     *m,
				Content:  l[1:],
				Position: diffPosCount,
			}
			newLine := line
			origLine := line

			// add lines to ranges
			switch *m {
			case ADDED:
				newLine.Number = ADDEDCount
				hunk.NewRange.Lines = append(hunk.NewRange.Lines, &newLine)
				hunk.WholeRange.Lines = append(hunk.WholeRange.Lines, &newLine)
				ADDEDCount++

			case REMOVED:
				origLine.Number = REMOVEDCount
				hunk.OrigRange.Lines = append(hunk.OrigRange.Lines, &origLine)
				hunk.WholeRange.Lines = append(hunk.WholeRange.Lines, &origLine)
				REMOVEDCount++

			case UNCHANGED:
				newLine.Number = ADDEDCount
				hunk.NewRange.Lines = append(hunk.NewRange.Lines, &newLine)
				hunk.WholeRange.Lines = append(hunk.WholeRange.Lines, &newLine)
				origLine.Number = REMOVEDCount
				hunk.OrigRange.Lines = append(hunk.OrigRange.Lines, &origLine)
				ADDEDCount++
				REMOVEDCount++
			}
		}
	}

	return &diff, nil
}

func isSourceLine(line string) bool {
	if line == `\ No newline at end of file` {
		return false
	}
	if l := len(line); l == 0 || (l >= 3 && (line[:3] == "---" || line[:3] == "+++")) {
		return false
	}
	return true
}

// Length returns the hunks line length
func (hunk *DiffHunk) Length() int {
	return len(hunk.WholeRange.Lines) + 1
}
