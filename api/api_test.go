package api

import (
	"testing"

	///"github.com/cespare/xxhash"

	"github.com/figment-networks/polkadot-worker/api/scale"
	"github.com/stretchr/testify/require"
)

func TestA(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name string
	}{
		{name: "asdsa"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outp, err := scale.AccountRequest("19W6iAE2aBQyVHC1E7hD89juYQQAyxoSoaiTYTSFWgWiLoH")
			require.NoError(t, err)
			require.Equal(t, outp, "0x26aa394eea5630e07c48ae0c9558cef7b99d880ec681799c0cf30e8886371da9f46244291085a6f49696f1d97e3124c8067bead5a674b1409931cb07cd94a97a3405991387afabdeebb61962617b6241")
		})
	}
}
