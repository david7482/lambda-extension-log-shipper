package eventservice

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	"github.com/david7482/lambda-extension-log-shipper/extension"
)

type extentionAPIClient interface {
	NextEvent(ctx context.Context) (res extension.NextEventResponse, err error)
	InitError(ctx context.Context, errorType string) (res extension.StatusResponse, err error)
	ExitError(ctx context.Context, errorType string) (res extension.StatusResponse, err error)
}

func RunEventService(ctx context.Context, wg *sync.WaitGroup, client extentionAPIClient) {
	go func() {
		defer wg.Done()

		// Will block until invoke or shutdown event is received or cancelled via the context.
		for {
			select {
			case <-ctx.Done():
				return
			default:
				zerolog.Ctx(ctx).Info().Msg("NextEvent ->")
				// This is a blocking call
				res, err := client.NextEvent(ctx)
				if err != nil {
					zerolog.Ctx(ctx).Error().Err(err).Msg("NextEvent <-")
					return
				}
				zerolog.Ctx(ctx).Info().Msg("NextEvent <-")

				// Flush log queue in here after waking up
				//flushLogQueue()

				// Exit if we receive a SHUTDOWN event
				if res.EventType == extension.Shutdown {
					zerolog.Ctx(ctx).Info().Msg("received SHUTDOWN event")
					//flushLogQueue()
					//logsApiAgent.Shutdown()
					return
				}
			}
		}
	}()
}
