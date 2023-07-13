package task

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

type ApiRequestPayload struct {
	Name         string            `json:"name" binding:"required"`
	URL          string            `json:"url" binding:"required"`
	Body         string            `json:"body"`
	Method       string            `json:"method" binding:"required"`
	Headers      map[string]string `json:"headers"`
	ExeAt        string            `json:"exe_at"`
	Retry        int               `json:"retry"`
	ResponseCode int               `json:"response_code"`
}

func ApiRequest(ctx context.Context, t *asynq.Task) error {
	c := make(chan error, 1)

	go func() {
		payload := t.Payload()

		var data ApiRequestPayload
		json.Unmarshal(payload, &data)

		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		req, err := http.NewRequest(strings.ToUpper(data.Method), data.URL, strings.NewReader(data.Body))
		if err != nil {
			c <- fmt.Errorf("failed to process task: %v", err)
			return
		}

		// Set headers
		if data.Headers != nil {
			for k, v := range data.Headers {
				req.Header.Set(k, v)
			}
		}

		// DO request
		resp, err := client.Do(req)
		if err != nil {
			c <- fmt.Errorf("failed to process task: %v", err)
			return
		}
		defer resp.Body.Close()

		// Check the response status code
		if data.ResponseCode != 0 && resp.StatusCode != data.ResponseCode {
			c <- fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		log.Println(string(body), resp)
		if err != nil {
			t.ResultWriter().Write(
				[]byte(fmt.Sprintf("failed to process task body: %v", err)),
			)
		} else {
			t.ResultWriter().Write(body)
		}

		c <- nil
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-c:
		return res
	}
}
