package xxhash

import (
	"hash"

	"github.com/OneOfOne/xxhash"
)

func New64x2() hash.Hash {
	d := new(digest)
	h := xxhash.NewS64(0)
	d.xxhash = h

	h2 := xxhash.NewS64(1)
	d.xxhash2 = h2

	return d
}

type digest struct {
	xxhash  *xxhash.XXHash64
	xxhash2 *xxhash.XXHash64
}

func (d *digest) Reset() {
	d.xxhash.Reset()
	d.xxhash2.Reset()
}

func (d *digest) Size() int      { return 16 }
func (d *digest) BlockSize() int { return 32 }

func (d *digest) Write(p []byte) (nn int, err error) {
	d.xxhash.Write(p)
	d.xxhash2.Write(p)
	return

}

func (d *digest) Sum(in []byte) []byte {
	return append(reverse(d.xxhash.Sum(in)), reverse(d.xxhash2.Sum(in))...)
}

func reverse(s []byte) []byte {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
