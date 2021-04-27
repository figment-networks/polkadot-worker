package scale

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	scalecodec "github.com/itering/scale.go"
	"github.com/itering/scale.go/source"
	"github.com/itering/scale.go/types"
	"github.com/itering/scale.go/utiles"

	"embed"
)

//go:embed networks/*
var networks embed.FS

type MDecoder struct {
	Decoder *scalecodec.MetadataDecoder
	Spec    uint
	Bytes   []byte
}

type DecodeStorage struct {
	SpecName    string
	decoders    map[uint]*MDecoder
	storageLock sync.RWMutex
}

func NewDecodeStorage() *DecodeStorage {
	return &DecodeStorage{decoders: make(map[uint]*MDecoder)}
}

func (ds *DecodeStorage) Init(specName string) (err error) {
	ds.SpecName = specName
	file, err := networks.ReadFile("networks/" + specName + ".json")
	if err != nil {
		return err
	}
	ds.storageLock.Lock()
	types.RuntimeType{}.Reg()
	types.RegCustomTypes(source.LoadTypeRegistry(file))
	ds.storageLock.Unlock()
	return err
}

func (ds *DecodeStorage) GetMetadataDecoder(specName string, specVersion uint) (*scalecodec.MetadataDecoder, bool, error) {
	m, b, er := ds.GetMDecoder(specName, specVersion)
	return m.Decoder, b, er
}

func (ds *DecodeStorage) GetMetadataBytes(specName string, specVersion uint) (io.Reader, bool, error) {
	m, b, er := ds.GetMDecoder(specName, specVersion)
	return bytes.NewReader(m.Bytes), b, er
}

func (ds *DecodeStorage) GetMDecoder(specName string, specVersion uint) (*MDecoder, bool, error) {
	if ds.SpecName != specName {
		return nil, false, fmt.Errorf("network name (specName) does not match - %s:%s", ds.SpecName, specName)
	}
	ds.storageLock.RLock()
	md, ok := ds.decoders[specVersion]
	ds.storageLock.RUnlock()
	return md, ok, nil
}

func (ds *DecodeStorage) SetMetadataDecoder(specVersion uint, metadata []byte) (md *MDecoder, err error) {
	// todo(lukanus): on panic
	/*
	   if r := recover(); r != nil {
	   	fmt.Println("Recovered in f", r)
	   }
	*/

	mDec := &MDecoder{
		Bytes: make([]byte, len(metadata)),
		Spec:  specVersion,
	}
	copy(mDec.Bytes, metadata)

	mDec.Decoder = &scalecodec.MetadataDecoder{}
	mDec.Decoder.Init(utiles.HexToBytes(string(metadata)))
	if err := mDec.Decoder.Process(); err != nil {
		return nil, err
	}

	ds.storageLock.Lock()
	ds.decoders[specVersion] = mDec
	ds.storageLock.Unlock()

	return mDec, nil

}

func (ds *DecodeStorage) GetExtrinsic(extrinsicRaw string, metadata *types.MetadataStruct, specVer int) (scalecodec.ExtrinsicDecoder, error) {

	// todo(lukanus): on panic
	e := scalecodec.ExtrinsicDecoder{}
	e.Init(types.ScaleBytes{Data: utiles.HexToBytes(extrinsicRaw)}, &types.ScaleDecoderOption{
		Metadata: metadata,
		Spec:     specVer,
	})
	e.Process()

	/*	b, _ := json.Marshal(e.Value)
		log.Println("extrinsicRaw", block.Contents.Header.Number, i, extrinsicRaw)
		log.Println("block", block.Contents.Header.Number, i, e.ContainsTransaction, e.ExtrinsicHash, string(b))*/
	//	}
	return e, nil
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
