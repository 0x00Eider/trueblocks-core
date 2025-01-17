package scrapePkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"syscall"

	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/config"
	"github.com/TrueBlocks/trueblocks-core/src/apps/chifra/pkg/notify"
)

var ErrConfiguredButNotRunning = fmt.Errorf("listener is configured but not running")

// Notify may be used to tell other processes about progress.
func Notify[T notify.NotificationPayload](notification notify.Notification[T]) error {
	endpoint := config.GetSettings().NotifyUrl
	if endpoint == "" {
		return nil
	}
	return notifyEndpoint(endpoint, notification)
}

func notifyEndpoint(endpoint string, notification any) error {
	encoded, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshalling message: %w", err)
	}

	resp, err := http.Post(
		endpoint,
		"application/json",
		bytes.NewReader(encoded),
	)

	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			return ErrConfiguredButNotRunning
		}
		return fmt.Errorf("sending notification: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("listener responded with %d: %s", resp.StatusCode, respBody)
	}
	return nil
}
