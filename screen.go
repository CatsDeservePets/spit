package main

import (
	"fmt"
)

func moveCursor(row, col int) {
	fmt.Printf("\033[%d;%dH", row, col)
}

func moveCursorToCol(n int) {
	fmt.Printf("\033[%dG", n)
}

func moveCursorUp(n int) {
	fmt.Printf("\033[%dA", n)
}

func moveCursorDown(n int) {
	fmt.Printf("\033[%dB", n)
}

func moveCursorForward(n int) {
	fmt.Printf("\033[%dC", n)
}

func moveCursorBack(n int) {
	fmt.Printf("\033[%dD", n)
}

func showCursor() {
	fmt.Print("\033[?25h")
}

func hideCursor() {
	fmt.Print("\033[?25l")
}

func cursorPosition() {
	fmt.Print("\033[6n")
}

func saveCursorPosition() {
	fmt.Print("\0337")
}

func restoreCursorPosition() {
	fmt.Print("\0338")
}

func printAt(row, col int, a ...any) {
	text := fmt.Sprint(a...)
	fmt.Printf("\033[%d;%dH%s", row, col, text)
}

func clear() {
	fmt.Print("\033[2J\033[H\033[3J")
}

func clearLine() {
	fmt.Print("\033[2K")
}

func clearAtCursor(n int) {
	fmt.Printf("\033[%dK", n)
}

func showAlternateScreen() {
	fmt.Print("\033[?1049h")
}

func hideAlternateScreen() {
	fmt.Print("\033[?1049l")
}

func setTitle(s string) {
	fmt.Printf("\033]0;%s\007", s)
}
