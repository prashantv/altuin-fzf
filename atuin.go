package main

import (
	"bufio"
	"bytes"
	"fmt"
	"iter"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const _atuinDelim = "\t:::\t"

type atuinParams struct {
	Query          string
	Limit          int
	FilterMode     string
	AdditionalArgs []string
}

type atuinResult struct {
	Time         string
	RelativeTime string
	Duration     string
	Exit         string
	Directory    string
	Command      string

	Error error
}

func runAtuin(p atuinParams) (iter.Seq[atuinResult], error) {
	format := strings.Join([]string{
		"{time}",
		"{relativetime}",
		"{duration}",
		"{exit}",
		"{directory}",
		"{command}", // intentionally last so command can contain the delimiter.
	}, _atuinDelim)

	args := []string{
		"search",
		"--limit", strconv.Itoa(p.Limit),
		"--format", format,
		"--print0",
	}
	if p.FilterMode != "" {
		args = append(args,
			"--filter-mode", p.FilterMode)
	}
	args = append(args, p.AdditionalArgs...)
	args = append(args, p.Query)

	cmd := exec.Command("atuin", args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return func(yield func(atuinResult) bool) {
		defer cmd.Wait()
		defer stdout.Close()

		scanner := bufio.NewScanner(stdout)
		scanner.Split(scanNull)
		for scanner.Scan() {
			parts := strings.SplitN(scanner.Text(), _atuinDelim, 6)
			if len(parts) < 6 {
				yield(atuinResult{
					Error: fmt.Errorf("text %q doesn't have expected 5 delimiters", scanner.Text()),
				})
				return
			}
			timestamp, relTimestamp, duration, exitCode, directory, command := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]

			if !yield(atuinResult{
				Time:         timestamp,
				RelativeTime: relTimestamp,
				Duration:     duration,
				Exit:         exitCode,
				Directory:    directory,
				Command:      command,
			}) {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			yield(atuinResult{Error: err})
		}
	}, nil
}

func scanNull(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexByte(data, byte(0)); i >= 0 {
		// terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
