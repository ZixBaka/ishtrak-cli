package cmd

import (
	"encoding/json"
	"io"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/zixbaka/ishtrak/internal/messaging"
	"github.com/zixbaka/ishtrak/internal/queue"
	"github.com/zixbaka/ishtrak/internal/requests"
)

var hostCmd = &cobra.Command{
	Use:    "host",
	Short:  "Run as a native messaging host (launched by the browser extension)",
	Hidden: true,
	RunE:   runHost,
}

func runHost(_ *cobra.Command, _ []string) error {
	h := messaging.NewStdio(cfg.MessagingTimeout())

	for {
		var req messaging.NativeMessage
		if err := h.ReceiveRequest(&req); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			log.Debug().Err(err).Msg("host: read request failed")
			return nil
		}

		switch req.Type {
		case messaging.TypePollRequests:
			if err := handlePollRequests(h); err != nil {
				log.Debug().Err(err).Msg("host: poll requests failed")
			}

		case messaging.TypeWriteResponses:
			if err := handleWriteResponses(h, req); err != nil {
				log.Debug().Err(err).Msg("host: write responses failed")
			}

		case messaging.TypeDrainQueue:
			if err := handleDrainQueue(h); err != nil {
				log.Debug().Err(err).Msg("host: drain queue failed")
			}

		default:
			log.Debug().Str("type", req.Type).Msg("host: unknown message type")
		}
	}
}

func handlePollRequests(h *messaging.Host) error {
	reqs, err := requests.ReadAllRequests()
	if err != nil {
		log.Debug().Err(err).Msg("host: read requests failed")
		reqs = nil
	}
	return h.SendRaw(messaging.PendingRequestsResponse{
		Type:     messaging.TypePendingRequests,
		Requests: reqs,
	})
}

func handleDrainQueue(h *messaging.Host) error {
	items, err := queue.DrainAll()
	if err != nil {
		log.Debug().Err(err).Msg("host: drain queue failed")
		items = nil
	}
	tasks := make([]messaging.CreateTaskPayload, 0, len(items))
	for _, raw := range items {
		var p messaging.CreateTaskPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			log.Debug().Err(err).Msg("host: skip malformed queue item")
			continue
		}
		tasks = append(tasks, p)
	}
	return h.SendRaw(messaging.PendingTasksResponse{
		Type:  messaging.TypePendingTasks,
		Tasks: tasks,
	})
}

func handleWriteResponses(h *messaging.Host, msg messaging.NativeMessage) error {
	raw, _ := json.Marshal(msg.Payload)
	var wr messaging.WriteResponsesMessage
	if err := json.Unmarshal(raw, &wr); err != nil {
		return err
	}
	for _, resp := range wr.Responses {
		if err := requests.WriteResponse(resp.UUID, resp.Data, resp.Error); err != nil {
			log.Debug().Err(err).Str("uuid", resp.UUID).Msg("host: write response failed")
		}
	}
	return h.SendRaw(struct {
		Type string `json:"type"`
	}{Type: messaging.TypeOK})
}
