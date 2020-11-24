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
	ListenPort   uint32
	MaxItems     uint32
	MaxBytes     uint32
	TimeoutMS    uint32
}

func RunLogService(ctx context.Context, wg *sync.WaitGroup, params LogServiceParams) {
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msg("fail to read all response body from log API")
			return
		}

		zerolog.Ctx(ctx).Info().Bytes("body", body).Msg("get logs from log API")
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", params.ListenPort),
		Handler: router,
	}

	go func() {
		// Wait for ctx done
		<-ctx.Done()

		// Give 3 second to shutdown server
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		server.SetKeepAlivesEnabled(false)
		_ = server.Shutdown(ctx)

		// Notify when server is closed
		zerolog.Ctx(ctx).Info().Msgf("log service is closed")
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
	_, err := params.LogAPIClient.SubscribeLogs(ctx, params.LogTypes, extension.SubscribeLogsParams{
		ListenPort: params.ListenPort,
		MaxItems:   params.MaxItems,
		MaxBytes:   params.MaxBytes,
		TimeoutMS:  params.TimeoutMS,
	})
	if err != nil {
		zerolog.Ctx(ctx).Fatal().Err(err).Msg("fail to subscribe log API")
	}
}
