package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"text/template"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/zixbaka/ishtrak/internal/config"
	"github.com/zixbaka/ishtrak/internal/daemon"
)

const daemonAddr = "127.0.0.1:7474"

var (
	daemonInstall   bool
	daemonUninstall bool
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the ishtrak daemon (WebSocket broker between CLI and extension)",
	Long: `Run the ishtrak daemon — a local WebSocket broker that lets CLI commands
communicate directly with the browser extension.

  ishtrak daemon             Start the daemon in the foreground
  ishtrak daemon --install   Install as a system service (auto-start on login)
  ishtrak daemon --uninstall Remove the system service`,
	RunE: runDaemon,
}

func init() {
	daemonCmd.Flags().BoolVar(&daemonInstall, "install", false, "install as a system service (auto-start on login)")
	daemonCmd.Flags().BoolVar(&daemonUninstall, "uninstall", false, "remove the system service")
}

func runDaemon(_ *cobra.Command, _ []string) error {
	if daemonInstall {
		return installService()
	}
	if daemonUninstall {
		return uninstallService()
	}
	return serveDaemon()
}

// ── Serve ─────────────────────────────────────────────────────────────────────

func serveDaemon() error {
	hub := daemon.NewHub()
	srv := daemon.NewServer(hub)

	ln, err := net.Listen("tcp", daemonAddr)
	if err != nil {
		return fmt.Errorf("daemon already running on %s (or port in use)", daemonAddr)
	}

	fmt.Fprintf(os.Stderr, "ishtrak daemon listening on %s\n", daemonAddr)
	log.Info().Str("addr", daemonAddr).Msg("daemon: started")

	httpSrv := &http.Server{Handler: srv.Handler()}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Info().Msg("daemon: shutting down")
		httpSrv.Close()
	}()

	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("daemon: %w", err)
	}
	return nil
}

// ── Service install/uninstall ─────────────────────────────────────────────────

func installService() error {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchd()
	case "linux":
		return installSystemd()
	default:
		return fmt.Errorf("service install not supported on %s — run 'ishtrak daemon' manually", runtime.GOOS)
	}
}

func uninstallService() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallLaunchd()
	case "linux":
		return uninstallSystemd()
	default:
		return fmt.Errorf("service uninstall not supported on %s", runtime.GOOS)
	}
}

// ── macOS launchd ─────────────────────────────────────────────────────────────

const launchdLabel = "com.ishtrak.daemon"

const launchdPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>{{.Label}}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{.Exe}}</string>
    <string>daemon</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>{{.LogDir}}/daemon.log</string>
  <key>StandardErrorPath</key>
  <string>{{.LogDir}}/daemon.log</string>
</dict>
</plist>
`

func launchdPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func installLaunchd() error {
	exe, err := selfExecutable()
	if err != nil {
		return err
	}

	logDir := config.ConfigDir()
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}

	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("plist").Parse(launchdPlist))
	if err := tmpl.Execute(f, map[string]string{
		"Label":  launchdLabel,
		"Exe":    exe,
		"LogDir": logDir,
	}); err != nil {
		return err
	}

	// Load the service immediately.
	if out, err := exec.Command("launchctl", "load", "-w", plistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s", out)
	}

	fmt.Printf("✓ ishtrak daemon installed as launchd service (%s)\n", launchdLabel)
	fmt.Printf("  Plist:   %s\n", plistPath)
	fmt.Printf("  Log:     %s/daemon.log\n", logDir)
	fmt.Println("  The daemon will start now and on every login.")
	return nil
}

func uninstallLaunchd() error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}
	// Unload (ignore errors if not loaded).
	exec.Command("launchctl", "unload", "-w", plistPath).Run() //nolint:errcheck
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	fmt.Println("✓ ishtrak daemon service removed")
	return nil
}

// ── Linux systemd ─────────────────────────────────────────────────────────────

const systemdUnit = `[Unit]
Description=Ishtrak daemon — WebSocket broker between CLI and browser extension
After=network.target

[Service]
ExecStart={{.Exe}} daemon
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
`

func systemdUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", "ishtrak-daemon.service"), nil
}

func installSystemd() error {
	exe, err := selfExecutable()
	if err != nil {
		return err
	}

	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(unitPath)
	if err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("unit").Parse(systemdUnit))
	if err := tmpl.Execute(f, map[string]string{"Exe": exe}); err != nil {
		return err
	}

	exec.Command("systemctl", "--user", "daemon-reload").Run()          //nolint:errcheck
	exec.Command("systemctl", "--user", "enable", "--now", "ishtrak-daemon").Run() //nolint:errcheck

	fmt.Println("✓ ishtrak daemon installed as systemd user service")
	fmt.Printf("  Unit:  %s\n", unitPath)
	fmt.Println("  The daemon will start now and on every login.")
	return nil
}

func uninstallSystemd() error {
	exec.Command("systemctl", "--user", "disable", "--now", "ishtrak-daemon").Run() //nolint:errcheck
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}
	exec.Command("systemctl", "--user", "daemon-reload").Run() //nolint:errcheck
	fmt.Println("✓ ishtrak daemon service removed")
	return nil
}
