package cmd

import (
	"errors"
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

func checkResp(resp *messaging.CommandResponse) error {
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}
