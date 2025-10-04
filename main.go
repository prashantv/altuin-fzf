package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/prashantv/atuin-fzf/tcolor"
)

const _delim = ":::"

// TODOs:
// Consider replacing the emoji X with a red indicator of exit status.
// Add fzf bind to go to a dir AND exec
// Bind to Ctrl-R

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--preview" {
		if len(os.Args) > 2 {
			if err := fzfPreview(os.Args[2]); err != nil {
				log.Fatal(err)
			}
			return
		}
		return
	}

	var initialQuery string
	if len(os.Args) > 1 {
		initialQuery = os.Args[1]
	}

	if err := run(initialQuery); err != nil {
		log.Fatal(err)
	}
}

func run(query string) error {
	atuin, err := altuinSearch()
	if err != nil {
		return err
	}
	defer atuin.stdout.Close()

	fzfInput, err := atuinAdapt(atuin.stdout)
	if err != nil {
		return err
	}

	if err := fzf(fzfInput, query); err != nil {
		return err
	}

	if err := atuin.cmd.Wait(); err != nil {
		return err
	}

	return nil
}

type cmdOutput struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

func altuinSearch() (*cmdOutput, error) {
	atuinFmt := strings.Join([]string{"{command}", "{exit}", "{directory}", "{duration}", "{time}"}, _delim)
	cmd := exec.Command("atuin", "search", "--limit", "1000", "--format", atuinFmt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("atuin stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start atuin: %w", err)
	}

	return &cmdOutput{
		cmd:    cmd,
		stdout: stdout,
	}, nil
}

func atuinAdapt(input io.Reader) (io.Reader, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	curDir, _ := os.Getwd() // best effort
	go func() {
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), _delim)
			command, exitCode, directory, duration, timestamp := parts[0], parts[1], parts[2], parts[3], parts[4]

			exitStatus := " "
			if exitCode != "0" {
				exitStatus = tcolor.Red.Foreground("exit " + exitCode)
			}

			dirCtx := ""
			if directory == curDir {
				dirCtx = " \033[38;5;242m(current dir)\033[0m"
			}

			_, err := fmt.Fprintln(w, strings.Join([]string{
				command,
				exitCode,
				directory,
				duration,
				timestamp,
				exitStatus,
				dirCtx,
			}, _delim))
			if err != nil {
				panic(err)
			}
		}
		if err := scanner.Err(); err != nil {
			// FIXME
			panic(err)
		}

		if err := w.Close(); err != nil {
			panic(err)
		}
	}()

	return r, nil
}

func fzf(input io.Reader, query string) error {
	selfExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("self executable: %w", err)
	}

	previewFmt := strings.Join([]string{"{1}", "{2}", "{3}", "{4}", "{5}", "{6}"}, _delim)
	previewCmd := fmt.Sprintf("%s --preview %s ", selfExe, previewFmt)

	fzfCmd := exec.Command(
		"fzf",
		"--tac",
		"--ansi",
		"--scheme", "history",
		"--prompt", "> ",
		"--header", "[Enter] to select, [Ctrl-Y] to yank.",
		"--preview", previewCmd,
		"--preview-window", "right:40%:wrap",
		"--delimiter", _delim,
		"--with-nth", "{1}  {6} {7}",
		"--accept-nth", "{1}",
		"--bind", "ctrl-y:execute-silent(echo -n {1} | pbcopy)+abort",
		"--query", query,
		"--height", "80%",
	)

	fzfCmd.Stdin = input
	fzfCmd.Stderr = os.Stderr
	fzfCmd.Stdout = os.Stdout

	if err := fzfCmd.Run(); err != nil {
		return fmt.Errorf("run fzf: %w", err)
	}

	return nil
}

func fzfPreview(data string) error {
	parts := strings.Split(data, _delim)
	if len(parts) < 5 {
		return fmt.Errorf("data format incorrect, expected 5 parts, got %d", len(parts))
	}
	command, exitCode, directory, duration, timestamp := parts[0], parts[1], parts[2], parts[3], parts[4]

	exitCol := tcolor.Green
	if exitCode != "0" {
		exitCol = tcolor.Red
	}

	fmt.Println(tcolor.Bold("Full Command"))
	fmt.Println("───────────────────────────────────────────────────")
	fmt.Println(command)
	fmt.Println()
	fmt.Println(tcolor.Bold("Execution Details"))
	fmt.Println("───────────────────────────────────────────────────")
	fmt.Printf("%-10s %s\n", "Status:", exitCol.Foreground(exitCode))
	fmt.Printf("%-10s %s\n", "Ran In:", directory)
	fmt.Printf("%-10s %s\n", "Duration:", duration)
	fmt.Printf("%-10s %s\n", "When:", timestamp)
	fmt.Println()
	fmt.Println(tcolor.Bold("Recent Similar Commands"))
	fmt.Println("───────────────────────────────────────────────────")

	// Run two atuin searches and combine/deduplicate the results
	globalSearch := exec.Command("atuin", "search", "--limit", "5", "--search-mode", "prefix", "--format", "{command}\t{directory}", command)
	dirSearch := exec.Command("atuin", "search", "--limit", "5", "--search-mode", "prefix", "--cwd", directory, "--format", "{command}\t{directory}", command)

	seen := make(map[string]bool)
	printResults := func(cmd *exec.Cmd) error {
		output, err := cmd.Output()
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(bytes.NewReader(output))
		for scanner.Scan() {
			line := scanner.Text()
			if !seen[line] {
				seen[line] = true
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) == 2 {
					fmt.Printf("%-40.40s (%s)\n", parts[0], parts[1])
				}
			}
		}
		return nil
	}

	err := errors.Join(
		printResults(globalSearch),
		printResults(dirSearch),
	)
	return err
}
