package metrics

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/posthog/posthog-go"
)

type client struct {
	posthog.Client
}

var c *client

func Init() {
	if !isEnabled() {
		return
	}

	ph, err := posthog.NewWithConfig(
		"phc_whf6xThJIHQw6Og5F9dsP3eb7dkrh3N4oZT7nzDhMR0",
		posthog.Config{
			Endpoint: "https://app.posthog.com",
		},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrap(err, "fail to init telemetry"))
	}

	c = &client{ph}
}

func isEnabled() bool {
	env := strings.ToLower(os.Getenv("TELOPHASE_METRICS_DISABLED"))
	if env == "true" || env == "t" {
		return false
	}

	return true
}

func Close() {
	if c == nil || !isEnabled() {
		return
	}

	err := c.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrap(err, "fail to close telemetry"))
	}
}

type Event string

const EventRunCommand Event = "telophasecli"

func Push(event Event, properties posthog.Properties) {
	if c == nil || !isEnabled() {
		return
	}

	if err := c.Enqueue(posthog.Capture{
		DistinctId: getDistinctId(),
		Event:      string(event),
		Properties: properties,
	}); err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrap(err, "fail to enqueue telemetry"))
	}
}

func RegisterCommand() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	Push(
		EventRunCommand,
		map[string]interface{}{
			"$set": map[string]interface{}{
				"command": command,
			},
		},
	)
}

func getDistinctId() string {
	id := uuid.NewString()

	homedir, err := os.UserHomeDir()
	if err != nil {
		return id
	}

	telpohaseDir := path.Join(homedir, ".telophase")
	err = os.MkdirAll(telpohaseDir, os.ModePerm)
	if err != nil {
		return id
	}

	fpath := path.Join(telpohaseDir, "userid")

	bs, err := os.ReadFile(fpath)
	if err != nil {
		os.WriteFile(fpath, []byte(id), 0644)
		return id
	}

	prevId, err := uuid.ParseBytes(bs)
	if err != nil {
		os.WriteFile(fpath, []byte(id), 0644)
		return id
	}

	return prevId.String()
}
