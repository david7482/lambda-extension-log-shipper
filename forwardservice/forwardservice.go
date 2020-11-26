package forwardservice

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/david7482/lambda-extension-log-shipper/forwardservice/forwarders/stdout"
	"github.com/david7482/lambda-extension-log-shipper/logservice"
)

type forwarder interface {
	SetupConfigs(app *kingpin.Application)
	IsEnable() bool
	SendLog(requestID string, time time.Time, content []byte)
	SendMetrics(requestID string, time time.Time, metrics logservice.Metrics)
}

var (
	Forwarders = []forwarder{
		stdout.New(),
	}
)

type ForwardServiceParams struct {
	LogQueue <-chan logservice.Log
}

type ForwardService struct {
	logQueue <-chan logservice.Log
}

func New(params ForwardServiceParams) *ForwardService {
	return &ForwardService{
		logQueue: params.LogQueue,
	}
}

func (s *ForwardService) Run(ctx context.Context, wg *sync.WaitGroup) {

	go func() {
		for log := range s.logQueue {
			// Send log to each forwarder
			for _, f := range Forwarders {
				if f.IsEnable() {
					switch log.Type {
					case "platform.report":
						f.SendMetrics(log.RequestID, log.Time, log.Metrics)
					case "function":
						f.SendLog(log.RequestID, log.Time, log.Content)
					}
				}
			}
		}

		zerolog.Ctx(ctx).Info().Msg("forward service is closed")
		wg.Done()
	}()

}
