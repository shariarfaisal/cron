package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/shariarfaisal/cron/client"
	"github.com/shariarfaisal/cron/worker"
)

func main() {
	// Load the environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	redisUri := os.Getenv("REDIS_URI")
	r := gin.Default()

	client := client.NewClient(redisUri)
	worker := worker.NewWorker(redisUri)
	worker.Start()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
			"name":    "faisal",
		})
	})

	r.POST("/add", func(c *gin.Context) {

		type Payload struct {
			A int
			B int
		}

		var payload Payload

		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(400, gin.H{
				"message": err.Error(),
			})
			return
		}

		add := payload.A + payload.B

		c.JSON(200, gin.H{
			"result": add,
		})
	})

	r.POST("/api-caller", client.ApiCallerHandler)

	r.Run(port)
}
