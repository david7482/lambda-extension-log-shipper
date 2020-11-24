package extension

import (
	"fmt"
	"net/http"
)

// Client is a simple client for the Lambda Extensions API
type Client struct {
	baseURL     string
	httpClient  *http.Client
	ExtensionID string
}

// NewClient returns a Lambda Extensions API client
func NewClient(awsLambdaRuntimeAPI string) *Client {
	return &Client{
		baseURL:    fmt.Sprintf("http://%s", awsLambdaRuntimeAPI),
		httpClient: &http.Client{},
	}
}