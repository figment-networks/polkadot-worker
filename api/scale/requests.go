package scale

import (
	"github.com/figment-networks/polkadot-worker/api/scale/address"
	"github.com/figment-networks/polkadot-worker/api/scale/blake128"
	"github.com/figment-networks/polkadot-worker/api/scale/xxhash"
	"github.com/itering/scale.go/utiles"
)

func AccountRequest(accountAddress string) (string, error) {
	d := xxhash.New64x2()
	d.Write([]byte("System"))
	system := d.Sum(nil)

	d2 := xxhash.New64x2()
	d2.Write([]byte("Account"))
	account := d2.Sum(nil)

	decoded := address.Decode(accountAddress)
	endPos, ss58Length, _ := address.CheckAddress(decoded)
	addressInBytes := decoded[ss58Length:endPos]

	sum, err := blake128.ToBlakeSum(addressInBytes)
	return "0x" + utiles.BytesToHex(system) + utiles.BytesToHex(account) + utiles.BytesToHex(sum) + utiles.BytesToHex(addressInBytes), err
}
