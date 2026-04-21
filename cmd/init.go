package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zixbaka/ishtrak/internal/config"
)

// nativeHostManifest is the template for the native messaging host JSON.
type nativeHostManifest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Path            string   `json:"path"`
	Type            string   `json:"type"`
	AllowedOrigins  []string `json:"allowed_origins"`
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up ishtrak for the first time",
	Long: `Runs an interactive wizard that:
  1. Prompts for your Chrome extension ID
  2. Writes the native messaging host manifest
  3. Creates ~/.config/ishtrak/config.toml`,
	RunE: runInit,
}

var updateExtensionID string

func init() {
	initCmd.Flags().StringVar(&updateExtensionID, "update-extension-id", "", "update extension ID in existing native host manifest without re-running wizard")
}

func runInit(cmd *cobra.Command, args []string) error {
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	if updateExtensionID != "" {
		return writeNativeHostManifest(binaryPath, updateExtensionID)
	}

	fmt.Println("=== Ishtrak Setup ===")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Step 1: Extension ID")
	fmt.Println("  Open chrome://extensions, enable Developer Mode, then copy")
	fmt.Println("  the ID shown under the Ishtrak extension.")
	fmt.Print("  Extension ID: ")
	extID, _ := reader.ReadString('\n')
	extID = strings.TrimSpace(extID)
	if extID == "" {
		fmt.Println("  (skipped — you can update later with: ishtrak init --update-extension-id YOUR_ID)")
	}

	fmt.Println()
	fmt.Println("Step 2: Writing native messaging host manifest...")
	if err := writeNativeHostManifest(binaryPath, extID); err != nil {
		return err
	}
	fmt.Println("  Done.")

	fmt.Println()
	fmt.Println("Step 3: Creating config file...")
	cfgFile := config.DefaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgFile), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.OpenFile(cfgFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			fmt.Printf("  Config already exists at %s — skipping.\n", cfgFile)
		} else {
			return fmt.Errorf("write config: %w", err)
		}
	} else {
		defer f.Close()
		if _, err := f.WriteString(config.Skeleton(extID)); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("  Config written to %s\n", cfgFile)
	}

	fmt.Println()
	fmt.Println("Setup complete! Next steps:")
	fmt.Println("  1. Edit ~/.config/ishtrak/config.toml to add your platform token")
	fmt.Println("  2. cd into a git repo and run: ishtrak hook install")
	fmt.Println("  3. Make a commit with a story ID in the branch name (e.g. feature/PROJ-123-my-task)")
	return nil
}

// writeNativeHostManifest writes the native messaging host manifest JSON to
// all browser-specific locations for the current OS.
func writeNativeHostManifest(binaryPath, extensionID string) error {
	origin := fmt.Sprintf("chrome-extension://%s/", extensionID)
	manifest := nativeHostManifest{
		Name:           "com.ishtrak.host",
		Description:    "Ishtrak CLI Native Messaging Host",
		Path:           binaryPath,
		Type:           "stdio",
		AllowedOrigins: []string{origin},
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	for _, dir := range nativeHostDirs() {
		dest := filepath.Join(dir, "com.ishtrak.host.json")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not create %s: %v\n", dir, err)
			continue
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not write %s: %v\n", dest, err)
			continue
		}
		fmt.Printf("  Manifest written: %s\n", dest)
	}
	return nil
}

// nativeHostDirs returns the OS-specific directories where native messaging
// host manifests must be placed.
func nativeHostDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts"),
			filepath.Join(home, "Library", "Application Support", "Chromium", "NativeMessagingHosts"),
			filepath.Join(home, "Library", "Application Support", "Mozilla", "NativeMessagingHosts"),
		}
	case "linux":
		return []string{
			filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts"),
			filepath.Join(home, ".config", "chromium", "NativeMessagingHosts"),
			filepath.Join(home, ".mozilla", "native-messaging-hosts"),
		}
	default: // windows paths handled via registry; best-effort directory
		appData := os.Getenv("APPDATA")
		return []string{
			filepath.Join(appData, "Google", "Chrome", "NativeMessagingHosts"),
		}
	}
}
