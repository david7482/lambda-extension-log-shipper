package logservice

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/david7482/lambda-extension-log-shipper/extension"
	"github.com/david7482/lambda-extension-log-shipper/logservice/automocks"
)

func logAPIClient(_ *testing.T, ctrl *gomock.Controller) LogAPIClient {
	client := automocks.NewMockLogAPIClient(ctrl)
	client.EXPECT().SubscribeLogs(gomock.Any(), gomock.Any(), gomock.Any()).Return(extension.SubscribeResponse{}, nil).Times(1)
	return client
}

func timeMustParse(value string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05.000Z07:00", value)
	if err != nil {
		panic(err)
	}
	return t
}

func TestLogService_Run(t *testing.T) {
	type args struct {
		Params       ServiceParams
		logAPIClient func(t *testing.T, ctrl *gomock.Controller) LogAPIClient
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "OK",
			args: func() args {
				params := ServiceParams{
					LogAPIClient: nil,
					LogTypes:     []extension.LogType{extension.Platform, extension.Function},
					LogsQueue:    make(chan []Log, 1), // buffered channel
					ListenPort:   8080,
					MaxItems:     128,
					MaxBytes:     128,
					TimeoutMS:    1000,
				}

				return args{
					Params:       params,
					logAPIClient: logAPIClient,
				}
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := New(ServiceParams{
				LogAPIClient: tt.args.logAPIClient(t, ctrl),
				LogTypes:     tt.args.Params.LogTypes,
				LogsQueue:    tt.args.Params.LogsQueue,
				ListenPort:   tt.args.Params.ListenPort,
				MaxItems:     tt.args.Params.MaxItems,
				MaxBytes:     tt.args.Params.MaxBytes,
				TimeoutMS:    tt.args.Params.TimeoutMS,
			})
			wg := sync.WaitGroup{}
			ctx, cancel := context.WithCancel(context.Background())

			wg.Add(1)
			s.Run(ctx, &wg)

			time.Sleep(100 * time.Millisecond)
			cancel()
			wg.Wait()
		})
	}
}

func TestLogService_logHandler(t *testing.T) {
	type args struct {
		Params       ServiceParams
		logAPIClient func(t *testing.T, ctrl *gomock.Controller) LogAPIClient
		logs         string
		wantLogs     []Log
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "OK",
			args: func() args {
				params := ServiceParams{
					LogAPIClient: nil,
					LogTypes:     []extension.LogType{extension.Platform, extension.Function},
					LogsQueue:    make(chan []Log, 1), // buffered channel
					ListenPort:   8080,
					MaxItems:     128,
					MaxBytes:     128,
					TimeoutMS:    1000,
				}

				logs := `
					[{
						"time": "2020-08-20T12:31:32.123Z",
						"type": "platform.start",
						"record": {
							"requestId": "6f7f0961f83442118a7af6fe80b88d56"
						 }
					},
					{
						"time": "2020-08-20T12:31:32.123Z",
						"type": "platform.logsSubscription",
						"record": {
							"name": "Foo.bar",
							"state": "Subscribed",
							"types": ["function", "platform"]
						}
					},
					{
						"time": "2020-08-20T12:31:32.123Z",
						"type": "function",
						"record": "ERROR something happened"
					},
					{
						"time": "2020-08-20T12:31:32.123Z",
						"type": "platform.logsDropped",
						"record": {
							"reason": "Consumer seems to have fallen behind as it has not acknowledged receipt of logs.",
							"droppedRecords": 123,
							"droppedBytes": 12345
						}
					},
					{
						"time": "2020-08-20T12:31:32.123Z",
						"type": "platform.end",
						"record": {
							"requestId": "6f7f0961f83442118a7af6fe80b88d56"
						 }
					},
					{
						"time": "2020-08-20T12:31:32.123Z",
						"type": "platform.report",
						"record": {
							"requestId": "6f7f0961f83442118a7af6fe80b88d56",
							"metrics": {
								"durationMs": 101.51,
								"billedDurationMs": 300,
								"memorySizeMB": 512,
								"maxMemoryUsedMB": 33,
								"initDurationMs": 116.67
							}
						}
					}]`

				wantLogs := []Log{
					{
						Time:      timeMustParse("2020-08-20T12:31:32.123Z"),
						Type:      Function,
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
						Content:   []byte(`"ERROR something happened"`),
					},
					{
						Time:      timeMustParse("2020-08-20T12:31:32.123Z"),
						Type:      PlatformLogsDropped,
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
						Content:   []byte(`{"reason": "Consumer seems to have fallen behind as it has not acknowledged receipt of logs.","droppedRecords": 123,"droppedBytes": 12345}`),
					},
					{
						Time:      timeMustParse("2020-08-20T12:31:32.123Z"),
						Type:      PlatformReport,
						RequestID: "6f7f0961f83442118a7af6fe80b88d56",
						Content:   []byte(`{"durationMs": 101.51,"billedDurationMs": 300,"memorySizeMB": 512,"maxMemoryUsedMB": 33,"initDurationMs": 116.67}`),
					},
				}

				return args{
					Params:       params,
					logAPIClient: logAPIClient,
					logs:         strings.ReplaceAll(strings.ReplaceAll(logs, "\n", ""), "\t", ""),
					wantLogs:     wantLogs,
				}
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := New(ServiceParams{
				LogAPIClient: tt.args.logAPIClient(t, ctrl),
				LogTypes:     tt.args.Params.LogTypes,
				LogsQueue:    tt.args.Params.LogsQueue,
				ListenPort:   tt.args.Params.ListenPort,
				MaxItems:     tt.args.Params.MaxItems,
				MaxBytes:     tt.args.Params.MaxBytes,
				TimeoutMS:    tt.args.Params.TimeoutMS,
			})
			wg := sync.WaitGroup{}
			ctx, cancel := context.WithCancel(context.Background())

			wg.Add(1)
			s.Run(ctx, &wg)

			time.Sleep(100 * time.Millisecond)

			resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d", tt.args.Params.ListenPort), "", strings.NewReader(tt.args.logs))
			require.NoError(t, err)
			require.EqualValues(t, 200, resp.StatusCode)

			logs := <-tt.args.Params.LogsQueue
			require.Len(t, logs, 3)
			require.EqualValues(t, tt.args.wantLogs, logs)

			cancel()
			wg.Wait()
		})
	}
}
