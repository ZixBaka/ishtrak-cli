package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"github.com/zixbaka/ishtrak/internal/messaging"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks on your project management platform",
}

// ── task create ───────────────────────────────────────────────────────────────

var taskCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new task",
	RunE:  runTaskCreate,
}

var (
	taskTitle     string
	taskDesc      string
	taskStoryID   string
	taskProjectID string
	taskHost      string
)

func init() {
	taskCreateCmd.Flags().StringVarP(&taskTitle, "title", "t", "", "task title (required)")
	taskCreateCmd.Flags().StringVarP(&taskDesc, "description", "d", "", "task description")
	taskCreateCmd.Flags().StringVarP(&taskStoryID, "story", "s", "", "story ID (e.g. PROJ-123)")
	taskCreateCmd.Flags().StringVar(&taskProjectID, "project", "", "project ID on the platform")
	taskCreateCmd.Flags().StringVar(&taskHost, "host", "", "platform host")
	_ = taskCreateCmd.MarkFlagRequired("title")
	taskCmd.AddCommand(taskCreateCmd)
}

func runTaskCreate(_ *cobra.Command, _ []string) error {
	host, projectID, _ := resolveHostAndProject(taskHost, taskProjectID)
	if host == "" {
		return fmt.Errorf("no platform host specified and none configured in config file")
	}
	payload := messaging.CreateTaskPayload{
		Host:        host,
		Title:       taskTitle,
		Description: taskDesc,
		StoryID:     taskStoryID,
		ProjectID:   projectID,
	}
	resp, err := sendRequest(messaging.TypeCreateTask, payload)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, "error:", resp.Error)
		os.Exit(1)
	}
	var task messaging.Task
	if err := json.Unmarshal(resp.Data, &task); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return printJSON(task)
}

// ── task list ─────────────────────────────────────────────────────────────────

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks in a project",
	RunE:  runTaskList,
}

var (
	listStatus    string
	listLimit     int
	listProjectID string
	listHost      string
)

func init() {
	taskListCmd.Flags().StringVar(&listProjectID, "project", "", "project ID")
	taskListCmd.Flags().StringVar(&listHost, "host", "", "platform host")
	taskListCmd.Flags().StringVar(&listStatus, "status", "", "filter by status (e.g. \"In Progress\")")
	taskListCmd.Flags().IntVar(&listLimit, "limit", 20, "max number of tasks to return")
	taskCmd.AddCommand(taskListCmd)
}

func runTaskList(_ *cobra.Command, _ []string) error {
	host, projectID, _ := resolveHostAndProject(listHost, listProjectID)
	if host == "" {
		return fmt.Errorf("no platform host specified and none configured in config file")
	}
	payload := messaging.ListTasksPayload{
		Host:      host,
		ProjectID: projectID,
		Status:    listStatus,
		Limit:     listLimit,
	}
	resp, err := sendRequest(messaging.TypeListTasks, payload)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, "error:", resp.Error)
		os.Exit(1)
	}
	var tasks []messaging.Task
	if err := json.Unmarshal(resp.Data, &tasks); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return printJSON(tasks)
}

// ── task get ──────────────────────────────────────────────────────────────────

var taskGetCmd = &cobra.Command{
	Use:   "get <issueKey>",
	Short: "Get a task by issue key (e.g. UHAVO-5)",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskGet,
}

var getHost string

func init() {
	taskGetCmd.Flags().StringVar(&getHost, "host", "", "platform host")
	taskCmd.AddCommand(taskGetCmd)
}

func runTaskGet(_ *cobra.Command, args []string) error {
	host, _, _ := resolveHostAndProject(getHost, "")
	if host == "" {
		return fmt.Errorf("no platform host specified and none configured in config file")
	}
	payload := messaging.GetTaskPayload{Host: host, TaskID: args[0]}
	resp, err := sendRequest(messaging.TypeGetTask, payload)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, "error:", resp.Error)
		os.Exit(1)
	}
	var task messaging.Task
	if err := json.Unmarshal(resp.Data, &task); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return printJSON(task)
}

// ── task update ───────────────────────────────────────────────────────────────

var taskUpdateCmd = &cobra.Command{
	Use:   "update <issueKey>",
	Short: "Update a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskUpdate,
}

var (
	updateTitle  string
	updateDesc   string
	updateStatus string
	updateHost   string
)

func init() {
	taskUpdateCmd.Flags().StringVar(&updateHost, "host", "", "platform host")
	taskUpdateCmd.Flags().StringVarP(&updateTitle, "title", "t", "", "new title")
	taskUpdateCmd.Flags().StringVarP(&updateDesc, "description", "d", "", "new description")
	taskUpdateCmd.Flags().StringVar(&updateStatus, "status", "", "new status (e.g. \"In Progress\")")
	taskCmd.AddCommand(taskUpdateCmd)
}

func runTaskUpdate(_ *cobra.Command, args []string) error {
	host, _, _ := resolveHostAndProject(updateHost, "")
	if host == "" {
		return fmt.Errorf("no platform host specified and none configured in config file")
	}
	payload := messaging.UpdateTaskPayload{
		Host:        host,
		TaskID:      args[0],
		Title:       updateTitle,
		Description: updateDesc,
		Status:      updateStatus,
	}
	resp, err := sendRequest(messaging.TypeUpdateTask, payload)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, "error:", resp.Error)
		os.Exit(1)
	}
	var task messaging.Task
	if err := json.Unmarshal(resp.Data, &task); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return printJSON(task)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func resolveHostAndProject(hostFlag, projectFlag string) (host, projectID, token string) {
	host = hostFlag
	projectID = projectFlag
	for h, p := range cfg.Platforms {
		if host == "" {
			host = h
		}
		if host == h {
			if projectID == "" {
				projectID = p.DefaultProjectID
			}
			token = p.Token
			break
		}
	}
	return
}

// sendRequest ensures the daemon is running and forwards the command to it.
func sendRequest(msgType string, payload interface{}) (*messaging.CommandResponse, error) {
	if err := ensureDaemon(); err != nil {
		return nil, err
	}
	return daemonCommand(msgType, payload)
}

// ensureDaemon starts the daemon if it is not already running.
// When the daemon is freshly started, it waits up to 5s for the extension to connect.
func ensureDaemon() error {
	client := &http.Client{Timeout: 500 * time.Millisecond}

	if resp, err := client.Get("http://127.0.0.1:7474/health"); err == nil {
		resp.Body.Close()
		return nil // daemon already running; let the command itself handle "not connected"
	}

	// Auto-start daemon in background.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find ishtrak executable: %w", err)
	}
	cmd := exec.Command(exe, "daemon")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	// Wait for daemon ready, then for extension to connect (up to 5s total).
	fmt.Fprintln(os.Stderr, "starting daemon, waiting for extension to connect...")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		resp, err := client.Get("http://127.0.0.1:7474/health")
		if err != nil {
			continue
		}
		var h struct {
			Connected bool `json:"connected"`
		}
		json.NewDecoder(resp.Body).Decode(&h) //nolint:errcheck
		resp.Body.Close()
		if h.Connected {
			return nil
		}
	}
	// Daemon is up but extension didn't connect yet — proceed and let the
	// daemon return a clear "extension not connected" error if needed.
	return nil
}

// daemonCommand sends a command to the daemon and returns the response.
func daemonCommand(msgType string, payload interface{}) (*messaging.CommandResponse, error) {
	body, err := json.Marshal(map[string]interface{}{
		"type":    msgType,
		"payload": payload,
	})
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 35 * time.Second}
	resp, err := client.Post(
		"http://127.0.0.1:7474/command",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &messaging.CommandResponse{Data: result.Data, Error: result.Error}, nil
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
