package h264reader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestH264Reader(t *testing.T) {
	assert := assert.New(t)

	file, ivfErr := os.Open("test.h264")
	if ivfErr != nil {
		panic(ivfErr)
	}

	reader, err := NewReader(file)
	if err != nil {
		panic(ivfErr)
	}

	assert.Nil(err)
	assert.NotNil(reader)

	nals := reader.ReadFrames()

	expectedCountOfNals := 26
	assert.Equal(len(nals), expectedCountOfNals)

	for _, nal := range nals {
		assert.True(len(nal.Data) > 0)
	}
}
