package cmd

import (
	"fmt"
	"os"

	"github.com/zixbaka/ishtrak/internal/messaging"
)

func selfExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find ishtrak executable: %w", err)
	}
	return exe, nil
}

func checkResp(resp *messaging.CommandResponse) {
	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, "error:", resp.Error)
		os.Exit(1)
	}
}
