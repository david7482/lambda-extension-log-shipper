package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// RegisterResponse is the body of the response for /register
type RegisterResponse struct {
	FunctionName    string `json:"functionName"`
	FunctionVersion string `json:"functionVersion"`
	Handler         string `json:"handler"`
}

// NextEventResponse is the response for /event/next
type NextEventResponse struct {
	EventType          EventType `json:"eventType"`
	DeadlineMs         int64     `json:"deadlineMs"`
	RequestID          string    `json:"requestId"`
	InvokedFunctionArn string    `json:"invokedFunctionArn"`
	Tracing            Tracing   `json:"tracing"`
}

// Tracing is part of the response for /event/next
type Tracing struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// StatusResponse is the body of the response for /init/error and /exit/error
type StatusResponse struct {
	Status string `json:"status"`
}

// EventType represents the type of events received from /event/next
type EventType string

const (
	// Invoke is a lambda invoke
	Invoke EventType = "INVOKE"

	// Shutdown is a shutdown event for the environment
	Shutdown EventType = "SHUTDOWN"
)

const (
	extensionNameHeader       = "Lambda-Extension-Name"
	extensionIdentifierHeader = "Lambda-Extension-Identifier"
	extensionErrorType        = "Lambda-Extension-Function-Error-Type"

	extensionURL = "/2020-01-01/extension"
)

// RegisterExtension will register the extension with the Extensions API
func (e *Client) RegisterExtension(ctx context.Context, filename string) (res RegisterResponse, err error) {
	const action = "/register"
	url := e.baseURL + extensionURL + action

	reqBody, err := json.Marshal(map[string]interface{}{
		"events": []EventType{Invoke, Shutdown},
	})
	if err != nil {
		return res, err
	}

	// Create a HTTP Request with Context.
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return res, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(extensionNameHeader, filename)

	// Make the request
	httpRes, err := e.httpClient.Do(httpReq)
	if err != nil {
		return res, err
	}
	defer httpRes.Body.Close()
	body, err := ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return res, err
	}
	if httpRes.StatusCode != http.StatusOK {
		return res, fmt.Errorf("extension: RegisterExtension failed, status: %s, response: %s", httpRes.Status, string(body))
	}

	// Parse the response
	err = json.Unmarshal(body, &res)
	if err != nil {
		return res, err
	}
	e.ExtensionID = httpRes.Header.Get(extensionIdentifierHeader)

	return res, nil
}

// NextEvent blocks while long polling for the next lambda invoke or shutdown
func (e *Client) NextEvent(ctx context.Context) (res NextEventResponse, err error) {
	const action = "/event/next"
	url := e.baseURL + extensionURL + action

	// Create a HTTP Request with Context.
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return res, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(extensionIdentifierHeader, e.ExtensionID)

	// Make the request
	httpRes, err := e.httpClient.Do(httpReq)
	if err != nil {
		return res, err
	}
	defer httpRes.Body.Close()
	body, err := ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return res, err
	}
	if httpRes.StatusCode != http.StatusOK {
		return res, fmt.Errorf("extension: NextEvent failed, status: %s, response: %s", httpRes.Status, string(body))
	}

	// Parse the response
	err = json.Unmarshal(body, &res)
	if err != nil {
		return res, err
	}
	return res, nil
}

// InitError reports an initialization error to the platform. Call it when you registered but failed to initialize
func (e *Client) InitError(ctx context.Context, errorType string) (res StatusResponse, err error) {
	const action = "/init/error"
	url := e.baseURL + extensionURL + action

	// Create a HTTP Request with Context.
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return res, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(extensionIdentifierHeader, e.ExtensionID)
	httpReq.Header.Set(extensionErrorType, errorType)

	// Make the request
	httpRes, err := e.httpClient.Do(httpReq)
	if err != nil {
		return res, err
	}
	defer httpRes.Body.Close()
	body, err := ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return res, err
	}
	if httpRes.StatusCode != http.StatusOK {
		return res, fmt.Errorf("extension: InitError failed, status: %s, response: %s", httpRes.Status, string(body))
	}

	// Parse the response
	err = json.Unmarshal(body, &res)
	if err != nil {
		return res, err
	}
	return res, nil
}

// ExitError reports an error to the platform before exiting. Call it when you encounter an unexpected failure
func (e *Client) ExitError(ctx context.Context, errorType string) (res StatusResponse, err error) {
	const action = "/exit/error"
	url := e.baseURL + extensionURL + action

	// Create a HTTP Request with Context.
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return res, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(extensionIdentifierHeader, e.ExtensionID)
	httpReq.Header.Set(extensionErrorType, errorType)

	// Make the request
	httpRes, err := e.httpClient.Do(httpReq)
	if err != nil {
		return res, err
	}
	defer httpRes.Body.Close()
	body, err := ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return res, err
	}
	if httpRes.StatusCode != http.StatusOK {
		return res, fmt.Errorf("extension: ExitError failed, status: %s, response: %s", httpRes.Status, string(body))
	}

	// Parse the response
	err = json.Unmarshal(body, &res)
	if err != nil {
		return res, err
	}
	return res, nil
}
