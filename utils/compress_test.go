package utils

import (
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
)

func TestCompressUncompress(t *testing.T) {
	type args struct {
		Data []byte
	}
	tests := []struct {
		name      string
		args      args
		wantError bool
	}{
		{
			name: "OK",
			args: func() args {
				var rets args
				_ = faker.FakeData(&rets)
				return rets
			}(),
		},
		{
			name: "Nil",
			args: func() args {
				return args{
					Data: nil,
				}
			}(),
			wantError: true,
		},
		{
			name: "Empty",
			args: func() args {
				return args{
					Data: make([]byte, 0),
				}
			}(),
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compress, err := Compress(tt.args.Data)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			decompress, err := Decompress(compress)
			assert.NoError(t, err)
			assert.EqualValues(t, tt.args.Data, decompress)
		})
	}
}
