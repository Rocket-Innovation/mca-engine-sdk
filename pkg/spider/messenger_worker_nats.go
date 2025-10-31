package spider

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sethvargo/go-envconfig"
	"github.com/targc/xnats-go"
)

type NATSWorkerMessengerAdapter struct {
	nc               *xnats.XNats
	p                *xnats.Producer
	c                *xnats.Consumer
	cctx             jetstream.ConsumeContext
	natsStreamPrefix string
	actionID         string
}

var _ WorkerMessengerAdapter = &NATSWorkerMessengerAdapter{}

type InitNATSWorkerMessengerAdapterOpt struct {
	BetaAutoSetupNATS bool
}

func InitNATSWorkerMessengerAdapter(ctx context.Context, actionID string, opt InitNATSWorkerMessengerAdapterOpt) (*NATSWorkerMessengerAdapter, error) {
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

	inputStream := buildInputSubject(env.NATSStreamPrefix)
	consumerID := buildWorkerConsumerID(env.NATSConsumerIDPrefix, actionID)

	slog.Info(
		"worker",
		slog.String("input_stream", inputStream),
		slog.String("consumer_id", consumerID),
		slog.String("action_id", actionID),
	)

	if opt.BetaAutoSetupNATS {
		err = betaCreateConsumer(ctx, nc.JS(), inputStream, consumerID)

		if err != nil {
			// return nil, err
		}
	}

	c, err := nc.Consumer(ctx, inputStream, consumerID)

	if err != nil {
		return nil, err
	}

	adapter := NATSWorkerMessengerAdapter{
		nc:               nc,
		p:                p,
		c:                c,
		natsStreamPrefix: env.NATSStreamPrefix,
		actionID:         actionID,
	}

	return &adapter, nil
}

func (m *NATSWorkerMessengerAdapter) ListenInputMessages(ctx context.Context, h func(c InputMessageContext, message InputMessage) error) error {

	if m.cctx != nil {
		return errors.New("cannot re-initialize")
	}

	ictx := context.Background()

	sem := make(chan struct{}, 10)

	cctx, err := m.c.Consume(func(msg jetstream.Msg) {
		sem <- struct{}{}

		err := msg.DoubleAck(ctx)

		if err != nil {
			slog.Error("failed to double ack message", slog.String("error", err.Error()))
			return
		}

		go func() error {
			defer func() {
				<-sem
			}()

			slog.Info(
				"received input",
				slog.String("b", string(msg.Data())),
			)

			metadata, err := msg.Metadata()

			if err != nil {
				slog.Error(err.Error())
				return err
			}

			var b NatsInputMessage

			err = json.Unmarshal(msg.Data(), &b)

			if err != nil {
				// TODO:
				slog.Error(err.Error())
				return err
			}

			if b.ActionID != m.actionID {
				return nil
			}

			err = h(
				InputMessageContext{
					Context:   ictx,
					Timestamp: metadata.Timestamp,
				},
				b.ToInputMessage(),
			)

			if err != nil {
				// TODO:
				return err
			}

			return nil
		}()
	})

	if err != nil {
		return err
	}

	m.cctx = cctx

	<-ctx.Done()

	return nil
}

func (m *NATSWorkerMessengerAdapter) SendTriggerMessage(ctx context.Context, message TriggerMessage) error {
	subject := buildTriggerSubject(m.natsStreamPrefix)

	b, err := json.Marshal(NatsTriggerMessage{}.FromTriggerMessage(message))

	if err != nil {
		return err
	}

	slog.Info(
		"sent trigger",
		slog.String("subject", subject),
		slog.String("b", string(b)),
	)

	err = m.p.Produce(ctx, subject, b)

	if err != nil {
		return err
	}

	return nil
}

func (m *NATSWorkerMessengerAdapter) SendOutputMessage(ctx context.Context, message OutputMessage) error {
	subject := buildOutputSubject(m.natsStreamPrefix)

	b, err := json.Marshal(NatsOutputMessage{}.FromOutputMessage(message))

	if err != nil {
		return err
	}

	slog.Info(
		"sent output",
		slog.String("subject", subject),
		slog.String("b", string(b)),
	)

	err = m.p.Produce(ctx, subject, b)

	if err != nil {
		return err
	}

	return nil
}

func (m *NATSWorkerMessengerAdapter) Close(ctx context.Context) error {
	m.cctx.Stop()
	m.nc.Close()
	return nil
}
