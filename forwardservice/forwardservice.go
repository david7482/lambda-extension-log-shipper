package forwardservice

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
)

type ForwardServiceParams struct {
	LogQueue <-chan []byte
}

type ForwardService struct {
	logQueue <-chan []byte
}

func New(params ForwardServiceParams) *ForwardService {
	return &ForwardService{
		logQueue: params.LogQueue,
	}
}

func (s *ForwardService) Run(ctx context.Context, wg *sync.WaitGroup) {

	go func() {
		for data := range s.logQueue {
			zerolog.Ctx(ctx).Info().RawJSON("data", data).Msg("get logs from log queue")
		}

		zerolog.Ctx(ctx).Info().Msg("forward service is closed")
		wg.Done()
	}()

}
