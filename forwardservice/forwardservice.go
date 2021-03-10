package forwardservice

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/david7482/lambda-extension-log-shipper/logservice"
)

type ForwarderParams struct {
	LambdaName string
	AWSRegion  string
}

type Forwarder interface {
	SetupConfigs(app *kingpin.Application)
	Init(params ForwarderParams)
	IsEnable() bool
	SendLog([]logservice.Log)
	Shutdown()
}

type ServiceParams struct {
	Forwarders []Forwarder
	LogsQueue  <-chan []logservice.Log
	LambdaName string
	AWSRegion  string
}

type ForwardService struct {
	forwarders []Forwarder
	logsQueue  <-chan []logservice.Log
}

func New(params ServiceParams) *ForwardService {
	s := &ForwardService{
		forwarders: params.Forwarders,
		logsQueue:  params.LogsQueue,
	}
	for _, f := range s.forwarders {
		f.Init(ForwarderParams{
			LambdaName: params.LambdaName,
			AWSRegion:  params.AWSRegion,
		})
	}
	return s
}

func (s *ForwardService) Run(ctx context.Context, wg *sync.WaitGroup) {

	go func() {
		zerolog.Ctx(ctx).Info().Msg("forward service is running")
		for logs := range s.logsQueue {
			// Send log to each forwarder
			for _, f := range s.forwarders {
				if f.IsEnable() {
					f.SendLog(logs)
				}
			}
		}

		zerolog.Ctx(ctx).Info().Msg("forward service is closing")
		for _, f := range s.forwarders {
			if f.IsEnable() {
				f.Shutdown()
			}
		}

		zerolog.Ctx(ctx).Info().Msg("forward service is closed")
		wg.Done()
	}()

}
