package main

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/prashantv/atuin-fzf/tcolor"
)

const _delim = "\t:::\t"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--preview":
			if err := fzfPreview(os.Args[2]); err != nil {
				log.Fatal(err)
			}
			return
		case "--zsh":
			exe, err := os.Executable()
			if err != nil {
				exe = os.Args[0]
			}
			fmt.Printf(_zshFn, exe)
			return
		}
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
	results, err := runAtuin(atuinParams{
		Limit: 1000,
	})
	if err != nil {
		return err
	}

	fzfInput, err := atuinToFzf(results)
	if err != nil {
		return err
	}

	if err := fzf(fzfInput, query); err != nil {
		return err
	}

	return nil
}

func atuinToFzf(results iter.Seq[atuinResult]) (io.Reader, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	curDir, _ := os.Getwd() // best effort
	go func() {
		for r := range results {
			if r.Error != nil {
				// FIXME
				panic(err)
			}

			dirCtx := ""
			if r.Directory == curDir {
				dirCtx = tcolor.Gray.Foreground("(same cwd)")
			}

			_, err := fmt.Fprintln(w, strings.Join([]string{
				r.Command,
				r.Exit,
				r.Directory,
				r.Duration,
				r.Time,
				r.RelativeTime,
				exitColor(r.Exit),
				dirCtx,
			}, _delim))
			if err != nil {
				// FIXME
				panic(err)
			}
		}

		if err := w.Close(); err != nil {
			// FIXME
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

	previewCmd := fmt.Sprintf("%s --preview {}", selfExe)
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
		"--with-nth", "{1}  {7} {8}",
		"--accept-nth", "{1}",
		"--bind", "ctrl-y:execute-silent(echo -n {1} | pbcopy)+abort",
		"--bind", "ctrl-o:become(printf \"CHDIR:\\t%s\\t%s\" {3} {1})",
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
	if len(parts) < 6 {
		return fmt.Errorf("data format incorrect, expected 5 parts, got %d in %s", len(parts), data)
	}
	command, exitCode, directory, duration, timestamp, relTimestamp := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]

	exitCol := tcolor.Green
	if exitCode != "0" {
		exitCol = tcolor.Red
	}

	fmt.Println(tcolor.Bold("Command"))
	fmt.Println("───────────────────────────────────────────────────")
	fmt.Println(command)
	fmt.Println()
	fmt.Println(tcolor.Bold("Execution Details"))
	fmt.Println("───────────────────────────────────────────────────")
	fmt.Printf("%-10s %s %s\n", "When:", timestamp, tcolor.Cyan.Foreground(relTimestamp+" ago"))
	fmt.Printf("%-10s %s\n", "Directory:", shortenHome(directory))
	fmt.Printf("%-10s %s\n", "Exit Code:", exitCol.Foreground(exitCode))
	fmt.Printf("%-10s %s\n", "Duration:", duration)
	fmt.Println()
	fmt.Println(tcolor.Bold("Recent Similar Commands"))
	fmt.Println("───────────────────────────────────────────────────")

	seen := make(map[atuinResult]bool)
	printResults := func(addArgs ...string) error {
		results, err := runAtuin(atuinParams{
			Query:          command,
			Limit:          5,
			AdditionalArgs: addArgs,
		})
		if err != nil {
			return err
		}

		for r := range results {
			if !seen[r] {
				seen[r] = true
				fmt.Printf("%s %s %s\n%s\n",
					tcolor.Cyan.Foreground(r.RelativeTime),
					tcolor.Gray.Foreground(shortenHome(r.Directory)),
					exitColor(r.Exit),
					tcolor.Bold("$ ")+r.Command,
				)
			}
		}
		return nil
	}

	err := errors.Join(
		printResults(),
		printResults("--cwd", directory),
	)
	return err
}

func exitColor(exitCode string) string {
	if exitCode != "0" {
		return tcolor.Red.Foreground("exit " + exitCode)
	}
	return ""
}

func shortenHome(s string) string {
	homeDir, err := os.UserHomeDir()
	if err == nil {
		if suffix, ok := strings.CutPrefix(s, homeDir); ok {
			return filepath.Join("~", suffix)
		}
	}

	return s
}
