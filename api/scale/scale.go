package scale

import (
	"log"

	scalecodec "github.com/itering/scale.go"
	"github.com/itering/scale.go/types"
	"github.com/itering/scale.go/utiles"
)

func ParseMetadata(res string) types.MetadataStruct {

	m := scalecodec.MetadataDecoder{}
	m.Init(utiles.HexToBytes(res))
	if m.Process() != nil {
		log.Println("Test MetadataDecoder Process fail")
	}
	if m.Version != "MetadataV12Decoder" {
		log.Println("MetadataV12 version should equal 12")
	}
	return m.Metadata
}
