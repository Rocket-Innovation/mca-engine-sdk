package spider

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sethvargo/go-envconfig"
	"github.com/targc/xnats-go"
	"golang.org/x/sync/errgroup"
)

type NATSWorkflowMessengerAdapter struct {
	nc                     *xnats.XNats
	p                      *xnats.Producer
	triggerMessageConsumer *xnats.Consumer
	outputMessageConsumer  *xnats.Consumer
	outputMessageCCtx      jetstream.ConsumeContext
	triggerMessageCCtx     jetstream.ConsumeContext
	natsStreamPrefix       string
}

var _ WorkflowMessengerAdapter = &NATSWorkflowMessengerAdapter{}

type InitNATSWorkflowMessengerAdapterOpt struct {
	BetaAutoSetupNATS bool
}

func InitNATSWorkflowMessengerAdapter(ctx context.Context, opt InitNATSWorkflowMessengerAdapterOpt) (*NATSWorkflowMessengerAdapter, error) {
	type Env struct {
		NATSHost             string `env:"NATS_HOST,required"`
		NATSPort             int    `env:"NATS_PORT,required"`
		NATSUser             string `env:"NATS_USER,required"`
		NATSPassword         string `env:"NATS_PASSWORD,required"`
		NATSStreamPrefix     string `env:"NATS_STREAM_PREFIX,required"`
		NATSConsumerIDPrefix string `env:"NATS_CONSUMER_ID_PREFIX,required"`
	}

	var env Env

	err := envconfig.Process(ctx, &env)

	if err != nil {
		return nil, err
	}

	nc, err := xnats.Connect(xnats.ConnectOpt{
		Host:     env.NATSHost,
		Port:     env.NATSPort,
		User:     env.NATSUser,
		Password: env.NATSPassword,
	})

	if err != nil {
		return nil, err
	}

	p := nc.Producer()

	triggerStream := buildTriggerSubject(env.NATSStreamPrefix)
	inputStream := buildInputSubject(env.NATSStreamPrefix)
	outputStream := buildOutputSubject(env.NATSStreamPrefix)
	workflowActionTriggerConsumerID := buildWorkflowActionTriggerConsumerID(env.NATSConsumerIDPrefix)
	workflowActionOutputConsumerID := buildWorkflowActionOutputConsumerID(env.NATSConsumerIDPrefix)

	slog.Info(
		"workflow",
		slog.String("output_stream", outputStream),
		slog.String("consumer_id", workflowActionOutputConsumerID),
	)

	if opt.BetaAutoSetupNATS {

		err = betaCreateJetstream(ctx, nc.JS(), triggerStream)

		if err != nil {
			// return nil, err
		}

		err = betaCreateJetstream(ctx, nc.JS(), inputStream)

		if err != nil {
			// return nil, err
		}

		err = betaCreateJetstream(ctx, nc.JS(), outputStream)

		if err != nil {
			// return nil, err
		}

		err = betaCreateConsumer(ctx, nc.JS(), triggerStream, workflowActionTriggerConsumerID)

		if err != nil {
			// return nil, err
		}

		err = betaCreateConsumer(ctx, nc.JS(), outputStream, workflowActionOutputConsumerID)

		if err != nil {
			// return nil, err
		}
	}

	triggerMessageConsumer, err := nc.Consumer(ctx, triggerStream, workflowActionTriggerConsumerID)

	if err != nil {
		return nil, err
	}

	outputMessageConsumer, err := nc.Consumer(ctx, outputStream, workflowActionOutputConsumerID)

	if err != nil {
		return nil, err
	}

	adapter := NATSWorkflowMessengerAdapter{
		nc:                     nc,
		p:                      p,
		triggerMessageConsumer: triggerMessageConsumer,
		outputMessageConsumer:  outputMessageConsumer,
		natsStreamPrefix:       env.NATSStreamPrefix,
	}

	return &adapter, nil
}

func (m *NATSWorkflowMessengerAdapter) ListenTriggerMessages(ctx context.Context, h func(c TriggerMessageContext, message TriggerMessage) error) error {

	if m.triggerMessageCCtx != nil {
		return errors.New("cannot re-initialize")
	}

	ictx := context.Background()

	eg := errgroup.Group{}

	eg.SetLimit(50)

	cctx, err := m.triggerMessageConsumer.Consume(func(msg jetstream.Msg) {
		eg.Go(func() error {
			msg.Ack()

			slog.Info(
				"received output",
				slog.String("b", string(msg.Data())),
			)

			metadata, err := msg.Metadata()

			if err != nil {
				// TODO:
				return err
			}

			var b NatsTriggerMessage

			err = json.Unmarshal(msg.Data(), &b)

			if err != nil {
				// TODO:
				return err
			}

			err = h(
				TriggerMessageContext{
					Context:   ictx,
					Timestamp: metadata.Timestamp,
				},
				b.ToTriggerMessage(),
			)

			if err != nil {
				// TODO:
				return err
			}

			return nil
		})
	})

	if err != nil {
		return err
	}

	m.triggerMessageCCtx = cctx

	<-ctx.Done()

	return nil
}

func (m *NATSWorkflowMessengerAdapter) ListenOutputMessages(ctx context.Context, h func(c OutputMessageContext, message OutputMessage) error) error {

	if m.outputMessageCCtx != nil {
		return errors.New("cannot re-initialize")
	}

	ictx := context.Background()

	eg := errgroup.Group{}

	eg.SetLimit(50)

	cctx, err := m.outputMessageConsumer.Consume(func(msg jetstream.Msg) {
		eg.Go(func() error {
			msg.Ack()

			slog.Info(
				"received output",
				slog.String("b", string(msg.Data())),
			)

			metadata, err := msg.Metadata()

			if err != nil {
				// TODO:
				return err
			}

			var b NatsOutputMessage

			err = json.Unmarshal(msg.Data(), &b)

			if err != nil {
				// TODO:
				return err
			}

			err = h(
				OutputMessageContext{
					Context:   ictx,
					Timestamp: metadata.Timestamp,
				},
				b.ToOutputMessage(),
			)

			if err != nil {
				// TODO:
				return err
			}

			return nil
		})
	})

	if err != nil {
		return err
	}

	m.outputMessageCCtx = cctx

	<-ctx.Done()

	return nil
}

func (m *NATSWorkflowMessengerAdapter) SendInputMessage(ctx context.Context, message InputMessage) error {
	subject := buildInputSubject(m.natsStreamPrefix)

	b, err := json.Marshal(NatsInputMessage{
		SessionID:  message.SessionID,
		TaskID:     message.TaskID,
		WorkflowID: message.WorkflowID,
		TenantID:   message.TenantID,
		// TODO
		// WorkflowActionID: message.WorkflowActionID,
		Key:      message.Key,
		ActionID: message.ActionID,
		Values:   message.Values,
	})

	if err != nil {
		return err
	}

	slog.Info("produce", slog.Any("b", string(b)))

	err = m.p.Produce(ctx, subject, b)

	if err != nil {
		return err
	}

	slog.Info(
		"sent input",
		slog.String("subject", subject),
		slog.String("b", string(b)),
	)

	return nil
}

func (m *NATSWorkflowMessengerAdapter) Close(ctx context.Context) error {
	m.outputMessageCCtx.Stop()
	m.nc.Close()
	return nil
}
