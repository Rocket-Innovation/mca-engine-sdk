package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/Rocket-Innovation/spider-go/pkg/spider"

	"github.com/go-co-op/gocron/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/r3labs/diff/v3"
)

const actionID = "cron-trigger-action"

func main() {

	ctx, cancel := context.WithCancel(context.Background())

	worker, err := spider.InitDefaultWorker(ctx, actionID)

	if err != nil {
		panic(err)
	}

	app := fiber.New()

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return nil
	})

	nctx, ncancel := signal.NotifyContext(ctx, os.Interrupt)
	defer ncancel()

	s, err := gocron.NewScheduler()

	if err != nil {
		panic(err)
	}

	defer func() {
		_ = s.Shutdown()
	}()

	s.Start()

	go WaitCronsChanged(ctx, worker, s, func(cc spider.WorkerConfig) {
		slog.Info(
			"triggered",
			slog.String("workflow_id", cc.WorkflowID),
			slog.String("key", cc.Key),
			slog.Any("config", cc.Config),
		)

		worker.SendTriggerMessage(ctx, spider.TriggerMessage{
			WorkflowID: cc.WorkflowID,
			MetaOutput: "triggered",
			Key:        cc.Key,
			Values:     "{}",
		})
	})

	go func() {
		err := app.Listen("0.0.0.0:8080")

		if err != nil {
			panic(err)
		}
	}()

	<-nctx.Done()

	cancel()
	_ = worker.Close(ctx)
}

func WaitCronsChanged(ctx context.Context, worker *spider.Worker, s gocron.Scheduler, h func(spider.WorkerConfig)) error {
	jobs := map[string]gocron.Job{}

	var curJConfs []string

	for {
		confs, err := worker.GetAllConfigs(ctx)

		if err != nil {
			return err
		}

		var jconfs []string

		for _, conf := range confs {
			b, err := json.Marshal(conf)

			if err != nil {
				return err
			}

			jconfs = append(jconfs, string(b))
		}

		changes, err := diff.Diff(curJConfs, jconfs)

		if err != nil {
			return err
		}

		for _, change := range changes {

			var (
				target     interface{}
				targetType string
			)

			switch change.Type {
			case "create":
				target = change.To
				targetType = "create"
			case "update":
				target = change.To
				targetType = "update"
			case "delete":
				target = change.From
				targetType = "delete"
			}

			c, ok := target.(string)

			if !ok {
				continue
			}

			var cc spider.WorkerConfig

			err := json.Unmarshal([]byte(c), &cc)

			if err != nil {
				continue
			}

			if cc.Config == nil {
				continue
			}

			cron, ok := cc.Config["cron"]

			if !ok {
				continue
			}

			switch targetType {
			case "create":
				job, err := s.NewJob(
					gocron.CronJob(
						cron,
						true,
					),
					gocron.NewTask(
						func() {
							h(cc)
						},
					),
				)

				if err != nil {
					slog.Error("failed to create job", slog.String("error", err.Error()))
					continue
				}

				log.Println("create", cc, job.ID())

				jobs[cc.WorkflowActionID] = job

			case "update":
				prevjob, ok := jobs[cc.WorkflowActionID]

				if !ok {
					continue
				}

				job, err := s.Update(
					prevjob.ID(),
					gocron.CronJob(
						cron,
						true,
					),
					gocron.NewTask(
						func() {
							h(cc)
						},
					),
				)

				if err != nil {
					continue
				}

				log.Println("update", cc, job.ID())

				jobs[cc.WorkflowActionID] = job
			case "delete":

				prevjob, ok := jobs[cc.WorkflowActionID]

				if !ok {
					continue
				}

				err := s.RemoveJob(prevjob.ID())

				if err != nil {
					continue
				}

				log.Println("delete", cc, prevjob.ID())

				delete(jobs, cc.WorkflowActionID)
			}
		}

		curJConfs = jconfs

		time.Sleep(time.Second * 10)
	}
}
