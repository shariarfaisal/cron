package worker

import (
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/shariarfaisal/cron/task"
)

type Worker struct {
	redisConn asynq.RedisConnOpt
}

func NewWorker(redisAddr string) *Worker {
	redisConnOpt := asynq.RedisClientOpt{Addr: redisAddr}

	return &Worker{
		redisConn: redisConnOpt,
	}
}

func (worker *Worker) Start() {

	srv := asynq.NewServer(
		worker.redisConn,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 3,
				"default":  2,
				"low":      1,
			},
			StrictPriority: false,
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// This specifies that all failed task will wait two seconds before being processed again.
				return 5 * time.Second
			},
		},
	)

	mux := asynq.NewServeMux()

	mux.HandleFunc("instant", task.ApiRequest)

	go func() {
		if err := srv.Run(mux); err != nil {
			log.Fatal(err)
		}
	}()
}
