package structs

type DecodeDataRequest struct {
	Block          []byte
	BlockHash      string
	Events         []byte
	Timestamp      []byte
	MetadataParent []byte
	RuntimeParent  []byte
}
