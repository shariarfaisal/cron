package schedular

import (
	"log"
	"time"

	"github.com/hibiken/asynq"
)

type Scheduler struct {
	redisConn asynq.RedisConnOpt
}

func NewScheduler(redisAddr string) *Scheduler {
	redisConnOpt := asynq.RedisClientOpt{Addr: redisAddr}
	return &Scheduler{
		redisConn: redisConnOpt,
	}
}

func (s *Scheduler) Run() {

	// load dhaka timezone
	loc, err := time.LoadLocation("Asia/Dhaka")
	if err != nil {
		panic(err)
	}

	scheduler := asynq.NewScheduler(
		s.redisConn,
		&asynq.SchedulerOpts{
			Location: loc,
			EnqueueErrorHandler: func(task *asynq.Task, opts []asynq.Option, err error) {
				// your error handling logic
			},
		})

	// scheduler.Register("@every 24h", asynq.NewTask("", nil), asynq.Queue("myqueue"))

	go func() {
		if err := scheduler.Run(); err != nil {
			log.Fatal(err)
		}
	}()
}
