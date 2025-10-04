// Package main Spider Workflow API
//
// @title Spider Workflow API
// @version 1.0
// @description Multi-tenant workflow management system
// @termsOfService http://swagger.io/terms/
//
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io
//
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
//
// @host localhost:8080
// @BasePath /
package main

import (
	"context"
	"os"
	"os/signal"

	_ "github.com/targc/spider-go/cmd/workflow/docs"
	"github.com/targc/spider-go/pkg/spider"
	"github.com/targc/spider-go/pkg/spider/apis"
	"github.com/targc/spider-go/pkg/spider/usecase"

	"github.com/gofiber/fiber/v2"
	fiberSwagger "github.com/swaggo/fiber-swagger"
)

func main() {

	ctx, cancel := context.WithCancel(context.Background())

	worflow, err := spider.InitDefaultWorkflow(ctx)

	if err != nil {
		panic(err)
	}

	storage := worflow.Storage()

	uc := usecase.NewUsecase(storage)
	handler := apis.NewHandler(uc)

	app := fiber.New()

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return nil
	})

	// swagger
	app.All("/swagger/*", fiberSwagger.WrapHandler)

	// flows
	app.Get("/tenants/:tenant_id/flows", handler.ListFlows)
	app.Get("/tenants/:tenant_id/flows/:id", handler.GetFlow)
	app.Post("/tenants/:tenant_id/flows", handler.CreateFlow)
	app.Put("/tenants/:tenant_id/flows/:flow_id", handler.UpdateFlow)
	app.Delete("/tenants/:tenant_id/flows/:flow_id", handler.DeleteFlow)

	// actions
	app.Post("/tenants/:tenant_id/workflows/:workflow_id/actions/:key/disable", handler.DisableAction)

	go worflow.Run(ctx)

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
	_ = worflow.Close(ctx)
}
