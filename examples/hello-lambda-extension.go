package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
)

func HandleRequest(ctx context.Context) error {
	fmt.Printf("Hello lambda extension!!! 1\n")
	fmt.Printf("Hello lambda extension!!! 2\n")
	fmt.Printf("Hello lambda extension!!! 3\n")
	return nil
}

func main() {
	lambda.Start(HandleRequest)
}
