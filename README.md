# DiffParser

DiffParser is a Golang package which parse's a git diff.

> [!NOTE]
>
> This package is a hard-fork of <https://github.com/waigani/diffparser> which
> hasn't been updated in a while.
>
> Expect API changes, bug fixes, etc.

## Install

    go get github.com/jedevc/diffparser

## Usage Example

```go
package main

import (
    "os"

    "github.com/jedevc/diffparser"
)

// error handling left out for brevity
func main() {
    byt, _ := os.ReadFile("example.diff")
    diff, _ := diffparser.Parse(string(byt))

    // You now have a slice of files from the diff,
    file := diff.Files[0]

    // diff hunks in the file,
    hunk := file.Hunks[0]

    // new and old ranges in the hunk
    newRange := hunk.NewRange

    // and lines in the ranges.
    line := newRange.Lines[0]
}
```

## More Examples

See `diffparser_test.go` for further examples.
