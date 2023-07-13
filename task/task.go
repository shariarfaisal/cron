package task

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/hibiken/asynq"
)

type ApiRequestPayload struct {
	Name   string `json:"name" binding:"required"`
	URL    string `json:"url" binding:"required"`
	Body   string `json:"body"`
	Method string `json:"method" binding:"required"`
	ExeAt  string `json:"exe_at"`
	Retry int    `json:"retry"`
}

func ApiRequest(ctx context.Context, t *asynq.Task) error {
	payload := t.Payload()

	var data ApiRequestPayload
	json.Unmarshal(payload, &data)

	client := &http.Client{}

	req, err := http.NewRequest(strings.ToUpper(data.Method), data.URL, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return err
	} 

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return err
	}

	// Print the response body
	fmt.Println("Response:", string(body))

	return nil
}
