package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Rocket-Innovation/spider-go/pkg/spider"

	"github.com/sethvargo/go-envconfig"
	"github.com/slack-go/slack"
)

const actionID = "slack-action"

func main() {
	ctx := context.Background()

	var conf struct {
		SlackWebhookURL string `env:"SLACK_WEBHOOK_URL, required"`
	}

	err := envconfig.Process(ctx, &conf)

	if err != nil {
		panic(err)
	}

	slog.Info("config", slog.Any("config", conf))

	err = spider.LazyBootstrapWorker(actionID, func(c spider.InputMessageContext, m spider.InputMessage) error {

		slog.Info("[process] received input", slog.Any("message", m))

		var input struct {
			Value string `json:"value"`
		}

		err := json.Unmarshal([]byte(m.Values), &input)

		if err != nil {
			slog.Error(err.Error())
			return err
		}

		text := input.Value

		attachment := slack.Attachment{
			Text: text,
		}

		msg := slack.WebhookMessage{
			Attachments: []slack.Attachment{attachment},
		}

		err = slack.PostWebhookContext(c.Context, conf.SlackWebhookURL, &msg)

		if err != nil {
			slog.Error(err.Error())
			return err
		}

		output := map[string]interface{}{
			"value": text,
		}

		outputb, err := json.Marshal(output)

		if err != nil {
			slog.Error(err.Error())
			return err
		}

		err = c.SendOutput("success", string(outputb))

		if err != nil {
			slog.Error(err.Error())
			return err
		}

		slog.Info("[process] sent output", slog.Any("message", string(outputb)))

		return nil
	})

	if err != nil {
		panic(err)
	}
}
