package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func confirmAction(prompt string) (bool, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return false, fmt.Errorf("confirmation required but stdin is not interactive; rerun with --yes")
	}

	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
