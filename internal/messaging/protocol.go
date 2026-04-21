package messaging

import "encoding/json"

// Message types (extension → host)
const (
	TypeCreateTask     = "CREATE_TASK"
	TypeGetProfile     = "GET_PROFILE"
	TypeListProfiles   = "LIST_PROFILES"
	TypeDeleteProfile  = "DELETE_PROFILE"
	TypePollRequests   = "POLL_REQUESTS"   // extension → host: read pending command requests
	TypeWriteResponses = "WRITE_RESPONSES" // extension → host: write command responses to disk
	TypeDrainQueue     = "DRAIN_QUEUE"     // extension → host: drain git-hook pending queue
	TypeListTasks      = "LIST_TASKS"
	TypeGetTask        = "GET_TASK"
	TypeUpdateTask     = "UPDATE_TASK"
)

// Response types (host → extension, or extension → CLI)
const (
	TypeTaskCreated     = "TASK_CREATED"
	TypeTaskError       = "TASK_ERROR"
	TypeProfileFound    = "PROFILE_FOUND"
	TypeProfileNotFound = "PROFILE_NOT_FOUND"
	TypeProfilesList    = "PROFILES_LIST"
	TypePendingRequests = "PENDING_REQUESTS" // host → extension: pending command requests
	TypePendingTasks    = "PENDING_TASKS"    // host → extension: pending git-hook tasks
	TypeOK              = "OK"               // host → extension: generic ack
	TypeTasksList       = "TASKS_LIST"
	TypeTaskFound       = "TASK_FOUND"
	TypeTaskUpdated     = "TASK_UPDATED"
)

// NativeMessage is a message sent to the extension (or received by the host).
type NativeMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// NativeResponse is a message received from the extension (or sent by the host).
type NativeResponse struct {
	Type      string            `json:"type"`
	TaskID    string            `json:"taskId,omitempty"`
	TaskURL   string            `json:"taskUrl,omitempty"`
	Error     string            `json:"error,omitempty"`
	Profile   interface{}       `json:"profile,omitempty"`
	Profiles  interface{}       `json:"profiles,omitempty"`
	Requests  []CommandRequest  `json:"requests,omitempty"`
	Task      *Task             `json:"task,omitempty"`
	Tasks     []Task            `json:"tasks,omitempty"`
}

// CreateTaskPayload is the payload for CREATE_TASK messages.
type CreateTaskPayload struct {
	Host        string `json:"host"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	StoryID     string `json:"storyId,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	Token       string `json:"token,omitempty"`
}

// ListTasksPayload is the payload for LIST_TASKS messages.
type ListTasksPayload struct {
	Host      string `json:"host"`
	ProjectID string `json:"projectId,omitempty"`
	Status    string `json:"status,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// GetTaskPayload is the payload for GET_TASK messages.
type GetTaskPayload struct {
	Host   string `json:"host"`
	TaskID string `json:"taskId"`
}

// UpdateTaskPayload is the payload for UPDATE_TASK messages.
type UpdateTaskPayload struct {
	Host        string `json:"host"`
	TaskID      string `json:"taskId"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

// Task is a task returned from the extension.
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	Assignee    string `json:"assignee,omitempty"`
	URL         string `json:"url,omitempty"`
}

// GetProfilePayload is the payload for GET_PROFILE messages.
type GetProfilePayload struct {
	Host string `json:"host"`
}

// DeleteProfilePayload is the payload for DELETE_PROFILE messages.
type DeleteProfilePayload struct {
	Host string `json:"host"`
}

// CommandRequest is a pending CLI command request read from disk by the host.
type CommandRequest struct {
	UUID    string          `json:"uuid"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// CommandResponse is a processed CLI command response written to disk by the host.
type CommandResponse struct {
	UUID  string          `json:"uuid"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// PendingRequestsResponse is sent by the native host in response to POLL_REQUESTS.
type PendingRequestsResponse struct {
	Type     string           `json:"type"`
	Requests []CommandRequest `json:"requests"`
}

// WriteResponsesMessage is sent by the extension to write command responses to disk.
type WriteResponsesMessage struct {
	Type      string            `json:"type"`
	Responses []CommandResponse `json:"responses"`
}

// PendingTasksResponse is sent by the native host in response to DRAIN_QUEUE.
type PendingTasksResponse struct {
	Type  string              `json:"type"`
	Tasks []CreateTaskPayload `json:"tasks"`
}
