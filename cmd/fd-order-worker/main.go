package main

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Rocket-Innovation/mca-engine-sdk/pkg/spider"
)

const actionID = "fd-order-action"

func main() {
	err := spider.LazyBootstrapWorker(actionID, func(c spider.InputMessageContext, m spider.InputMessage) error {

		slog.Info("[process] received input", slog.Any("message", m))

		var input struct {
			Value string `json:"value"`
		}

		err := json.Unmarshal([]byte(m.Values), &input)

		if err != nil {
			slog.Error(err.Error())
			return err
		}

		for i := range 10 {

			orderID := "order@" + fmt.Sprint(i+1)

			output := map[string]interface{}{
				"order_id": orderID,
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
		}

		return nil
	})

	if err != nil {
		panic(err)
	}
}
