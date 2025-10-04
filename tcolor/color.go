package tcolor

import "fmt"

type Color int

const (
	Red   Color = 1
	Green Color = 2
	Gray  Color = 8
)

func (c Color) Foreground(s string) string {
	return fmt.Sprintf("\033[38;5;%dm%s\033[0m", c, s)
}

func Bold(s string) string {
	return fmt.Sprintf("\033[1m%s\033[0m", s)
}
