package structs

type DecodeDataRequest struct {
	Block            []byte
	Chain            string
	BlockHash        string
	Events           []byte
	Timestamp        []byte
	MetadataParent   []byte
	RuntimeParent    []byte
	CurrentEra       []byte
	NextFeeMultipier []byte
}
