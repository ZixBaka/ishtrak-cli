package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zixbaka/ishtrak/internal/messaging"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage learned platform profiles",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all platform profiles learned by the extension",
	RunE:  runProfileList,
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <host>",
	Short: "Delete a learned platform profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileDelete,
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileDeleteCmd)
}

func runProfileList(_ *cobra.Command, _ []string) error {
	if err := ensureDaemon(); err != nil {
		return err
	}
	resp, err := daemonCommand(messaging.TypeListProfiles, nil)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s", resp.Error)
	}
	out, _ := json.MarshalIndent(resp.Data, "", "  ")
	fmt.Fprintln(os.Stdout, string(out))
	return nil
}

func runProfileDelete(_ *cobra.Command, args []string) error {
	host := args[0]
	if err := ensureDaemon(); err != nil {
		return err
	}
	resp, err := daemonCommand(messaging.TypeDeleteProfile, messaging.DeleteProfilePayload{Host: host})
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s", resp.Error)
	}
	fmt.Printf("Profile deleted: %s\n", host)
	return nil
}
