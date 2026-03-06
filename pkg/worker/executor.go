package worker

import (
	"context"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Executor struct {
	mux *asynq.ServeMux
	srv *asynq.Server
}

func NewExecutor(appCtx context.Context, opts asynq.RedisConnOpt, concurrency int) *Executor {
	if concurrency <= 0 {
		concurrency = 10
	}

	srv := asynq.NewServer(
		opts,
		asynq.Config{
			Queues: map[string]int{
				string(QueueNameDefault):  1,
				string(ScheduleQueueName): 1,
			},
			Concurrency: concurrency,
			BaseContext: func() context.Context {
				return appCtx
			},
		},
	)

	mux := asynq.NewServeMux()

	return &Executor{
		mux: mux,
		srv: srv,
	}
}

func (e *Executor) Start() error {
	return e.srv.Start(e.mux)
}

func (e *Executor) Stop() {
	e.srv.Stop()
	e.srv.Shutdown()
}

func (e *Executor) RegisterJobHandler(name TaskName, handler asynq.Handler) {
	e.mux.HandleFunc(string(name), e.loggingMiddleware(handler).ProcessTask)
}

func (e *Executor) loggingMiddleware(h asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {

		newCtx, span := tracer.Start(ctx, t.Type())
		span.SetStatus(codes.Ok, "OK")
		defer span.End()

		err := h.ProcessTask(newCtx, t)
		if err != nil {
			span.RecordError(err)
			return err
		}

		span.AddEvent("job completed", trace.WithAttributes(attribute.String("job", t.Type())))

		return nil
	})
}
