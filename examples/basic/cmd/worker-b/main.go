package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Rocket-Innovation/mca-engine-sdk/pkg/spider"

	"github.com/sethvargo/go-envconfig"
	"github.com/slack-go/slack"
)

const actionID = "test-action-b"

func main() {
	ctx := context.Background()

	var conf struct {
		SlackWebhookURL string `env:"SLACK_WEBHOOK_URL, required"`
	}

	err := envconfig.Process(ctx, &conf)

	if err != nil {
		panic(err)
	}

	err = spider.LazyBootstrapWorker(actionID, func(c spider.InputMessageContext, m spider.InputMessage) error {

		slog.Info("[process] received input", slog.Any("message", m))

		var input struct {
			Value string `json:"value"`
		}

		err := json.Unmarshal([]byte(m.Values), &input)

		if err != nil {
			return err
		}

		attachment := slack.Attachment{
			Text: input.Value,
		}

		msg := slack.WebhookMessage{
			Attachments: []slack.Attachment{attachment},
		}

		err = slack.PostWebhook(conf.SlackWebhookURL, &msg)

		if err != nil {
			return err
		}

		output := map[string]interface{}{
			"value": input.Value,
		}

		outputb, err := json.Marshal(output)

		if err != nil {
			return err
		}

		err = c.SendOutput("success", string(outputb))

		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		panic(err)
	}
}
