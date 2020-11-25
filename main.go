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

	"github.com/david7482/lambda-extension-log-shipper/extension"
	"github.com/david7482/lambda-extension-log-shipper/forwardservice"
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
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"
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
	logQueue := make(chan []byte, maxItems)

	// Start services
	wg.Add(1)
	logSrv := logservice.New(logservice.LogServiceParams{
		LogAPIClient: extensionClient,
		LogTypes:     logTypes,
		LogQueue:     logQueue,
		ListenPort:   listenPort,
		MaxItems:     maxItems,
		MaxBytes:     maxBytes,
		TimeoutMS:    timeoutMs,
	})
	logSrv.Run(rootCtx, &wg)

	wg.Add(1)
	forwardSrv := forwardservice.New(forwardservice.ForwardServiceParams{
		LogQueue: logQueue,
	})
	forwardSrv.Run(rootCtx, &wg)

	// Listen to SIGTEM/SIGINT to close
	var gracefulStop = make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT)

	// Will block until invoke or shutdown event is received or cancelled via the context.
LOOP:
	for {
		select {
		case s := <-gracefulStop:
			rootLogger.Info().Msgf("received signal to terminate: %s", s.String())
			break LOOP
		default:
			// This is a blocking call
			res, err := extensionClient.NextEvent(rootCtx)
			if err != nil {
				rootLogger.Error().Err(err).Msg("fail to invoke NextEvent")
				return
			}

			// Exit if we receive a SHUTDOWN event
			if res.EventType == extension.Shutdown {
				rootLogger.Info().Msg("received SHUTDOWN event")
				break LOOP
			}
		}
	}

	// Close root context to terminate everything
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
	case <-time.After(1950 * time.Millisecond):
		rootLogger.Err(context.DeadlineExceeded).Msg("fail to close all services")
	}
}
