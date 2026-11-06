package gitlog

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// BlameLine is one line of the current code (at HEAD) and who wrote it.
type BlameLine struct {
	Author string
	When   time.Time // when the commit that wrote this line was authored
}

// FileBlame is the blame of one file at HEAD.
type FileBlame struct {
	Path  string
	Lines []BlameLine
}

// Blame runs `git blame` on every text file at HEAD, in parallel.
//
// progress (optional) is called after each file with how many are done.
// It is called from several goroutines at once, so it must be safe for that.
func Blame(repoDir string, progress func(done, total int)) ([]FileBlame, error) {
	files, err := textFiles(repoDir)
	if err != nil {
		return nil, err
	}

	// Each worker writes only to results[i] for the i it received, so no two
	// goroutines touch the same element and we need no lock for the slice.
	results := make([]FileBlame, len(files))
	jobs := make(chan int)

	var (
		wg       sync.WaitGroup
		done     atomic.Int64 // safe counter shared by all workers
		errOnce  sync.Once
		firstErr error
	)

	for range runtime.NumCPU() {
		wg.Go(func() {
			for i := range jobs { // receives until the channel is closed
				fb, err := blameFile(repoDir, files[i])
				if err != nil {
					errOnce.Do(func() { firstErr = err })
					continue
				}
				results[i] = fb
				n := done.Add(1)
				if progress != nil {
					progress(int(n), len(files))
				}
			}
		})
	}

	for i := range files {
		jobs <- i // blocks until a worker is free to take it
	}
	close(jobs) // tells the workers there is no more work
	wg.Wait()   // waits for every worker to finish

	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// textFiles lists the non-binary files at HEAD.
//
// `git grep -I -e ""` matches every line of every text file (-I skips
// binaries), and --name-only -z prints each file once as "HEAD:<path>\0".
func textFiles(repoDir string) ([]string, error) {
	cmd := exec.Command("git", "grep", "-I", "--name-only", "-z", "-e", "", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		// git grep exits with 1 when nothing matched (e.g. only binary files).
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("listing files: %w", err)
	}

	var files []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		files = append(files, strings.TrimPrefix(name, "HEAD:"))
	}
	return files, nil
}

// blameFile runs git blame on one file and keeps the author of each line.
//
// --line-porcelain repeats the commit info before every line:
//
//	<sha> <orig-line> <final-line>
//	author André
//	author-mail <a@b.c>
//	author-time 1730900000
//	...
//	\t<the line's content>
//
// The content line always starts with a tab; header lines never do.
// -w ignores whitespace-only changes and -M follows lines moved inside the file.
func blameFile(repoDir, path string) (FileBlame, error) {
	out, err := git(repoDir, "blame", "--line-porcelain", "-w", "-M", "HEAD", "--", path)
	if err != nil {
		return FileBlame{}, fmt.Errorf("blame %s: %w", path, err)
	}

	fb := FileBlame{Path: path}
	var author string
	var when time.Time

	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024) // allow very long lines (minified files)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "\t"):
			fb.Lines = append(fb.Lines, BlameLine{Author: author, When: when})
		case strings.HasPrefix(line, "author "):
			author = strings.TrimPrefix(line, "author ")
		case strings.HasPrefix(line, "author-time "):
			secs, err := strconv.ParseInt(strings.TrimPrefix(line, "author-time "), 10, 64)
			if err != nil {
				return FileBlame{}, fmt.Errorf("blame %s: bad author-time: %w", path, err)
			}
			when = time.Unix(secs, 0)
		}
	}
	if err := sc.Err(); err != nil {
		return FileBlame{}, fmt.Errorf("blame %s: %w", path, err)
	}
	return fb, nil
}
