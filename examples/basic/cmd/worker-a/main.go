package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"

	"github.com/Rocket-Innovation/spider-go/pkg/spider"

	"github.com/gofiber/fiber/v2"
)

const actionID = "test-action-a"

func main() {

	ctx, cancel := context.WithCancel(context.Background())

	workerA, err := spider.InitDefaultWorker(ctx, actionID)

	if err != nil {
		panic(err)
	}

	app := fiber.New()

	app.Post("/trigger", func(c *fiber.Ctx) error {

		var payload struct {
			WorkflowID       string `json:"workflow_id"`
			WorkflowActionID string `json:"workflow_action_id"`
			Key              string `json:"key"`
			Value            string `json:"value"`
		}

		err = c.BodyParser(&payload)

		if err != nil {
			return err
		}

		output := map[string]interface{}{
			"value": payload.Value,
		}

		outputb, err := json.Marshal(output)

		if err != nil {
			return err
		}

		err = workerA.SendTriggerMessage(ctx, spider.TriggerMessage{
			WorkflowID: payload.WorkflowID,
			MetaOutput: "triggered",
			Key:        payload.Key,
			Values:     string(outputb),
		})

		if err != nil {
			return err
		}

		slog.Info("[process] sent")

		return nil
	})

	go func() {
		err := app.Listen("0.0.0.0:8080")

		if err != nil {
			panic(err)
		}
	}()

	nctx, ncancel := signal.NotifyContext(ctx, os.Interrupt)
	defer ncancel()

	<-nctx.Done()

	cancel()
	_ = workerA.Close(ctx)
}
