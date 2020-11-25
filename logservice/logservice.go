package logservice

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/david7482/lambda-extension-log-shipper/extension"
)

type logAPIClient interface {
	SubscribeLogs(ctx context.Context, types []extension.LogType, params extension.SubscribeLogsParams) (res extension.SubscribeResponse, err error)
}

type LogServiceParams struct {
	LogAPIClient logAPIClient
	LogTypes     []extension.LogType
	LogQueue     chan<- []byte
	ListenPort   uint32
	MaxItems     uint32
	MaxBytes     uint32
	TimeoutMS    uint32
}

type LogService struct {
	logAPIClient logAPIClient
	logTypes     []extension.LogType
	logQueue     chan<- []byte
	listenPort   uint32
	maxItems     uint32
	maxBytes     uint32
	timeoutMS    uint32
}

func New(params LogServiceParams) *LogService {
	return &LogService{
		logAPIClient: params.LogAPIClient,
		logTypes:     params.LogTypes,
		logQueue:     params.LogQueue,
		listenPort:   params.ListenPort,
		maxItems:     params.MaxItems,
		maxBytes:     params.MaxBytes,
		timeoutMS:    params.TimeoutMS,
	}
}

func (s *LogService) Run(ctx context.Context, wg *sync.WaitGroup) {
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("fail to read all response body from log API")
			return
		}

		// Sends to a buffered channel block only when the buffer is full
		s.logQueue <- body
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.listenPort),
		Handler: router,
	}

	go func() {
		// Wait for ctx done
		<-ctx.Done()

		// Give 3 second to shutdown server
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		server.SetKeepAlivesEnabled(false)
		_ = server.Shutdown(ctx)

		// Close log queue channel to notify forwarder
		close(s.logQueue)

		// Notify when server is closed
		zerolog.Ctx(ctx).Info().Msg("log service is closed")
		wg.Done()
	}()

	go func() {
		zerolog.Ctx(ctx).Info().Msgf("log service is on http://%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zerolog.Ctx(ctx).Fatal().Err(err).Str("Addr", server.Addr).Msg("Fail to start log service")
		}
	}()

	// Subscribe to logs API after log service is running
	// Logs start being delivered only after the subscription happens.
	_, err := s.logAPIClient.SubscribeLogs(ctx, s.logTypes, extension.SubscribeLogsParams{
		ListenPort: s.listenPort,
		MaxItems:   s.maxItems,
		MaxBytes:   s.maxBytes,
		TimeoutMS:  s.timeoutMS,
	})
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("fail to subscribe log API")
	}
}
