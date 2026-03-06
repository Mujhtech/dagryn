package worker

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

type Scheduler struct {
	scheduler *asynq.Scheduler
}

func NewScheduler(opts asynq.RedisConnOpt) *Scheduler {
	scheduler := asynq.NewScheduler(opts, &asynq.SchedulerOpts{})

	return &Scheduler{
		scheduler: scheduler,
	}
}

func (s *Scheduler) Start() error {
	return s.scheduler.Start()
}

func (s *Scheduler) Stop() {
	s.scheduler.Shutdown()
}

func (s *Scheduler) RegisterTask(cronSpec string, queue QueueName, taskName TaskName) {
	task := asynq.NewTask(string(taskName), nil)
	id, err := s.scheduler.Register(cronSpec, task, asynq.Queue(string(queue)))
	if err != nil {
		log.Error().Err(err).Msgf("Failed to register %s scheduler task", taskName)
	}
	log.Info().Msgf("Registered task %v with id %v", taskName, id)
}
