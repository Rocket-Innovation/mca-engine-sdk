package main

import (
	"encoding/json"
	"log/slog"

	"github.com/Rocket-Innovation/spider-go/pkg/spider"
	"github.com/gofiber/fiber/v2"
)

func main() {
	go func() {
		const actionID = "condition-action"

		err := spider.LazyBootstrapWorker(actionID, func(c spider.InputMessageContext, m spider.InputMessage) error {

			slog.Info("[process] received input", slog.Any("message", m))

			var input struct {
				Condition bool `json:"condition"`
			}

			err := json.Unmarshal([]byte(m.Values), &input)

			if err != nil {
				slog.Error(err.Error())
				return err
			}

			metaOutput := "no"

			if input.Condition {
				metaOutput = "yes"
			}

			err = c.SendOutput(metaOutput, "{}")

			if err != nil {
				slog.Error(err.Error())
				return err
			}

			slog.Info("[process] sent output", slog.Any("meta_output", metaOutput))

			return nil
		})

		if err != nil {
			panic(err)
		}
	}()

	app := fiber.New()

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return nil
	})

	err := app.Listen("0.0.0.0:8080")

	if err != nil {
		panic(err)
	}
}
