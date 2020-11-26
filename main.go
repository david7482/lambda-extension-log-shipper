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
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/david7482/lambda-extension-log-shipper/extension"
	"github.com/david7482/lambda-extension-log-shipper/forwardservice"
	"github.com/david7482/lambda-extension-log-shipper/logservice"
)

var (
	extensionName = filepath.Base(os.Args[0]) // extension name has to match the filename
	logTypes      = []extension.LogType{extension.Platform, extension.Function}
)

type generalConfig struct {
	AWSLambdaName *string
	AWSRegion     *string
	AWSRuntimeAPI *string
	TimeoutMs     *int
	MaxBytes      *int
	MaxItems      *int
	ListenPort    *int
}

func setupGeneralConfigs(app *kingpin.Application) generalConfig {
	var config generalConfig

	// the followings would read from lambda runtime environment variables
	config.AWSLambdaName = app.
		Flag("lambda-name", "The name of the lambda function").
		Envar("AWS_LAMBDA_FUNCTION_NAME").
		Required().String()
	config.AWSRegion = app.
		Flag("region", "The AWS Region where the Lambda function is executed").
		Envar("AWS_REGION").
		Required().String()
	config.AWSRuntimeAPI = app.
		Flag("runtime-api", "The endpoint URL of lambda extension runtime API").
		Envar("AWS_LAMBDA_RUNTIME_API").
		Required().String()

	// the followings are general settings for Lambda Log API
	config.TimeoutMs = app.
		Flag("timeout", "The timeout setting for Lambda Log API (in milliseconds)").
		Default("1000").Int()
	config.MaxBytes = app.
		Flag("maxbytes", "The maxbytes setting for Lambda Log API").
		Default("262144").Int()
	config.MaxItems = app.
		Flag("maxitems", "The maxitems setting for Lambda Log API").
		Default("10000").Int()
	config.ListenPort = app.
		Flag("port", "The port number that our log server listens on").
		Default("8443").Int()

	return config
}

func setupForwarderConfigs(app *kingpin.Application) {
	// let each forwarder setup its own configurations
	for _, f := range forwardservice.Forwarders {
		f.SetupConfigs(app)
	}
}

func main() {
	// Setup zerolog
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"
	rootLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Setup configurations
	app := kingpin.New("LS", "Lambda Extension Log Shipper").DefaultEnvars()
	cfg := setupGeneralConfigs(app)
	setupForwarderConfigs(app)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Create root context
	rootCtx, rootCtxCancelFunc := context.WithCancel(context.Background())
	rootCtx = rootLogger.WithContext(rootCtx)

	rootLogger.Info().Interface("config", cfg).Msg("lambda-extension-log-shipper start...")

	// Register extension as soon as possible
	extensionClient := extension.NewClient(*cfg.AWSRuntimeAPI)
	_, err := extensionClient.RegisterExtension(rootCtx, extensionName)
	if err != nil {
		rootLogger.Fatal().Err(err).Msg("fail to register extension")
	}

	wg := sync.WaitGroup{}
	logQueue := make(chan logservice.Log, *cfg.MaxItems)

	// Start services
	wg.Add(1)
	logSrv := logservice.New(logservice.LogServiceParams{
		LogAPIClient: extensionClient,
		LogTypes:     logTypes,
		LogQueue:     logQueue,
		ListenPort:   *cfg.ListenPort,
		MaxItems:     *cfg.MaxItems,
		MaxBytes:     *cfg.MaxBytes,
		TimeoutMS:    *cfg.TimeoutMs,
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
