package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func prompt(message string) (string, error) {
	fmt.Print(message)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) || line == "" {
			return "", err
		}
	}
	return strings.TrimSpace(line), nil
}
