package scale

import (
	"io/ioutil"

	scalecodec "github.com/itering/scale.go"
	"github.com/itering/scale.go/source"
	"github.com/itering/scale.go/types"
	"github.com/itering/scale.go/utiles"

	 "embed"
)


//go:embed build/networks
var networks embed.FS


type DecodeStorage struct {
	SpecName string
	SpecType string
}

func (ds *DecodeStorage) Init(specName string, io.Reader ) {
	ds.SpecName = specName

	types.RuntimeType{}.Reg()
}

func (ds *DecodeStorage) GetMetadata() types.MetadataStruct {
	/*
		m := scalecodec.MetadataDecoder{}
		m.Init(utiles.HexToBytes(res))
		if m.Process() != nil {
			log.Println("Test MetadataDecoder Process fail")
		}
		if m.Version != "MetadataV12Decoder" {
			log.Println("MetadataV12 version should equal 12")
		}
		return m.Metadata*/

	m := scalecodec.MetadataDecoder{}
	c, err := ioutil.ReadFile("./polkadot.json")
	if err != nil {
		panic(err)
	}
	types.RegCustomTypes(source.LoadTypeRegistry(c))
	m.Init(utiles.HexToBytes(metadata))
	if err = m.Process(); err != nil {
		return err
	}
	option := types.ScaleDecoderOption{Metadata: &m.Metadata, Spec: 29}
}

type PolkaBlock struct {
	Contents PolkaBlockContents `json:"block"`

	StateRoot      string `json:"stateRoot"`
	ExtrinsicsRoot string `json:"extrinsicsRoot"`
	ParentHash     string `json:"parentHash"`
}

type PolkaRuntimeVersion struct {
	Apis               [][2]interface{} `json:"apis"`
	AuthoringVersion   uint64           `json:"authoringVersion"`
	ImplName           string           `json:"implName"`
	ImplVersion        uint64           `json:"implVersion"`
	SpecName           string           `json:"specName"`
	SpecVersion        uint64           `json:"specVersion"`
	TransactionVersion uint64           `json:"transactionVersion"`
}

type PolkaBlockContents struct {
	Extrinsics []string    `json:"extrinsics"`
	Header     BlockHeader `json:"header"`
}

type BlockHeader struct {
	Number string `json:"number"`
}
