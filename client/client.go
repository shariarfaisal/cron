package client

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/shariarfaisal/cron/task"
)

type Client struct {
	redisClient asynq.Client
}

func NewClient(redisAddr string) *Client {
	redisClient := asynq.RedisClientOpt{Addr: redisAddr}
	return &Client{
		redisClient: *asynq.NewClient(redisClient),
	}
}

func (client *Client) ExeHandler(c *gin.Context) {

	var payload task.ApiRequestPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}

	// Convert payload to JSON
	taskPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}

	// Parse the ISO string into a time.Time value
	dateTime, err := time.Parse(time.RFC3339, payload.ExeAt)
	if err != nil {
		dateTime = time.Now()
	}

	if dateTime.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "exe_at must be in the future",
		})
		return
	}

	if payload.Retry == 0 {
		payload.Retry = 5
	}

	task := asynq.NewTask("instant", taskPayload, asynq.MaxRetry(payload.Retry))
	client.redisClient.Enqueue(task, asynq.ProcessAt(dateTime), asynq.Retention(time.Hour*24))

	c.JSON(200, gin.H{
		"message": "success",
	})
}
