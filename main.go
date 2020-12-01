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
	"github.com/david7482/lambda-extension-log-shipper/forwardservice/forwarders/newrelic"
	"github.com/david7482/lambda-extension-log-shipper/forwardservice/forwarders/stdout"
	"github.com/david7482/lambda-extension-log-shipper/logservice"
)

const (
	// ListenPort is the port that our log server listens on.
	listenPort = 8443
	// MaxItems is the maximum number of events to be buffered in memory. (default: 10000, minimum: 1000, maximum: 10000)
	maxItems = 10000
	// MaxBytes is the maximum size in bytes of the logs to be buffered in memory. (default: 262144, minimum: 262144, maximum: 1048576)
	maxBytes = 262144
	// TimeoutMS is the maximum time (in milliseconds) for a batch to be buffered. (default: 1000, minimum: 100, maximum: 30000)
	timeoutMS = 1000
)

var (
	extensionName = filepath.Base(os.Args[0]) // extension name has to match the filename
	logTypes      = []extension.LogType{extension.Platform, extension.Function}
	forwarders    = []forwardservice.Forwarder{stdout.New(), newrelic.New()}
)

type generalConfig struct {
	AWSLambdaName *string
	AWSRegion     *string
	AWSRuntimeAPI *string
	LogLevel      *string
	LogTimeFormat *string
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

	// the followings are general settings
	config.LogLevel = app.
		Flag("log-level", "The level of the logger").
		Envar("LS_LOG_LEVEL").
		Default("info").Enum("error", "warn", "info", "debug")
	config.LogTimeFormat = app.
		Flag("log-timeformat", "The timeformat of the logger").
		Envar("LS_LOG_TIMEFORMAT").
		Default("2006-01-02T15:04:05.000Z07:00").String()

	return config
}

func setupForwarderConfigs(app *kingpin.Application) {
	// let each forwarder setup its own configurations
	for _, f := range forwarders {
		f.SetupConfigs(app)
	}
}

func main() {
	// Setup configurations
	app := kingpin.New("lambda-extension-log-shipper", "Lambda Extension Log Shipper")
	cfg := setupGeneralConfigs(app)
	setupForwarderConfigs(app)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Setup zerolog
	lvl, _ := zerolog.ParseLevel(*cfg.LogLevel)
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = *cfg.LogTimeFormat
	rootLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()

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

	// Create the logs queue
	logsQueue := make(chan []logservice.Log, 8)

	// Start services
	wg := sync.WaitGroup{}
	wg.Add(1)
	logSrv := logservice.New(logservice.ServiceParams{
		LogAPIClient: extensionClient,
		LogTypes:     logTypes,
		LogsQueue:    logsQueue,
		ListenPort:   listenPort,
		MaxItems:     maxItems,
		MaxBytes:     maxBytes,
		TimeoutMS:    timeoutMS,
	})
	logSrv.Run(rootCtx, &wg)

	wg.Add(1)
	forwardSrv := forwardservice.New(forwardservice.ServiceParams{
		Forwarders: forwarders,
		LogsQueue:  logsQueue,
		LambdaName: *cfg.AWSLambdaName,
		AWSRegion:  *cfg.AWSRegion,
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
