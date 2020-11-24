package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/david7482/lambda-extension-log-shipper/eventservice"
	"github.com/david7482/lambda-extension-log-shipper/extension"
	"github.com/david7482/lambda-extension-log-shipper/logservice"
)

const (
	timeoutMs  = 1000
	maxBytes   = 262144
	maxItems   = 10000
	listenPort = 8443
)

var (
	extensionName = filepath.Base(os.Args[0]) // extension name has to match the filename
	logTypes      = []extension.LogType{extension.Platform, extension.Function}
)

func main() {
	// Setup zerolog
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339
	rootLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create root context
	rootCtx, rootCtxCancelFunc := context.WithCancel(context.Background())
	rootCtx = rootLogger.WithContext(rootCtx)

	rootLogger.Info().Msg("lambda-extension-log-shipper start...")

	// Register extension as soon as possible
	extensionClient := extension.NewClient(os.Getenv("AWS_LAMBDA_RUNTIME_API"))
	_, err := extensionClient.RegisterExtension(rootCtx, extensionName)
	if err != nil {
		rootLogger.Fatal().Err(err).Msg("fail to register extension")
	}

	wg := sync.WaitGroup{}

	// Start services
	wg.Add(1)
	logservice.RunLogService(rootCtx, &wg, logservice.LogServiceParams{
		LogAPIClient: extensionClient,
		LogTypes:     logTypes,
		ListenPort:   listenPort,
		MaxItems:     maxItems,
		MaxBytes:     maxBytes,
		TimeoutMS:    timeoutMs,
	})

	wg.Add(1)
	eventservice.RunEventService(rootCtx, &wg, extensionClient)

	// Listen to SIGTEM/SIGINT to close
	var gracefulStop = make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT)
	<-gracefulStop
	rootCtxCancelFunc()

	// Wait for all services to close with a specific timeout
	var waitUntilDone = make(chan struct{})
	go func() {
		wg.Wait()
		close(waitUntilDone)
	}()
	select {
	case <-waitUntilDone:
		rootLogger.Info().Msg("success to close all services")
	case <-time.After(1500 * time.Millisecond):
		rootLogger.Err(context.DeadlineExceeded).Msg("fail to close all services")
	}
}
