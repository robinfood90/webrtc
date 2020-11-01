// parser code written by https://github.com/chertov

package h264reader

import (
	"errors"
	"io"
	"log"
)

// H264Reader reads data from stream and constructs h264 nal units
type H264Reader struct {
	stream io.Reader
}

var errNilReader = errors.New("stream is nil")

// NewReader creates new H264Reader
func NewReader(in io.Reader) (*H264Reader, error) {
	if in == nil {
		return nil, errNilReader
	}

	reader := &H264Reader{
		stream: in,
	}

	return reader, nil
}

// NAL H.264 Network Abstraction Layer
type NAL struct {
	PictureOrderCount uint32

	// NAL header
	ForbiddenZeroBit bool
	RefIdc           uint8
	UnitType         NalUnitType

	Data []byte // header byte + rbsp
}

// ReadFrames reads all data from stream and returns array of all parsed nal units
func (reader *H264Reader) ReadFrames() []NAL {
	nalsBytes := make([][]byte, 0)
	nalStream := newFindNalState()
	for {
		buf := make([]byte, 1024)
		n, err := reader.stream.Read(buf)
		if err != nil && err != io.EOF {
			log.Fatal("Error Reading: ", err)
			break
		}
		if n == 0 {
			break
		}
		nal := nalStream.NalScan(buf[0:n])
		nalsBytes = append(nalsBytes, nal...)
	}

	i := 0
	var nals []NAL
	for {
		if i >= len(nalsBytes) {
			break
		}
		nalData := nalsBytes[i]
		i++
		nal := newNal()
		nal.parseHeader(nalData[0])
		if nal.UnitType == NalUnitTypeSEI {
			continue
		}
		nal.Data = nalData
		nals = append(nals, nal)
	}
	return nals
}

// Implementation

type findNalState struct {
	PrefixCount   int
	LastNullCount int
	buf           []byte
}

func newFindNalState() findNalState {
	return findNalState{PrefixCount: 0, LastNullCount: 0, buf: make([]byte, 0)}
}

func (h *findNalState) NalScan(data []byte) [][]byte { //nolint:gocognit
	if len(h.buf) > 1024*1024 {
		// nolint:godox
		// TODO: remove panic
		panic("findNalState buf len panic")
	}
	nals := make([][]byte, 0)

	// offset after a NAL prefix (0x00_00_01 or 0x00_00_00_01) in the data buffer
	var lastPrefixOffset *int = nil
	i := 0
	for {
		if i >= len(data) {
			if lastPrefixOffset != nil {
				// prefix was founded
				// copy a part of data buffer from the end of the last prefix into the temporary buffer
				h.buf = make([]byte, 0)
				h.buf = append(h.buf, data[*lastPrefixOffset:]...)
			} else {
				// a prefix was not found, save all data into the temporary buffer
				h.buf = append(h.buf, data...)
			}
			break
		}
		b := data[i]
		i++
		switch b {
		case 0x00:
			{
				if h.LastNullCount < 3 {
					h.LastNullCount++
				}
				continue
			}
		case 0x01:
			{
				if h.LastNullCount >= 2 { // found a NAL prefix 0x00_00_01 or 0x00_00_00_01
					prefixOffset := i
					if lastPrefixOffset != nil {
						// NAL is a part of data from the end of the last prefix to the beginning of the current prefix. Save it
						size := (i - h.LastNullCount) - *lastPrefixOffset - 1
						if size > 0 && h.PrefixCount > 0 {
							nal := data[*lastPrefixOffset : *lastPrefixOffset+size]
							// save nal
							nals = append(nals, nal)
						}
					} else {
						// a previous (last) prefix isn't exist
						// NAL is the temporary buffer and a part of data from the beginning to the current prefix
						size := i - h.LastNullCount - 1
						nal := make([]byte, 0)
						if size < 0 {
							if len(h.buf) > 0 {
								nal = append(nal, h.buf[0:len(h.buf)+size]...)
							}
						} else {
							nal = append(nal, h.buf...)
							nal = append(nal, data[0:size]...)
						}

						// save non-empty NAL only after at least one prefix was detected
						if len(nal) > 0 && h.PrefixCount > 0 {
							nals = append(nals, nal)
						}
						h.buf = make([]byte, 0)
					}
					p := prefixOffset
					lastPrefixOffset = &p
					h.PrefixCount++
				}
			}
		default:
		}
		h.LastNullCount = 0
	}
	return nals
}

func newNal() NAL {
	return NAL{PictureOrderCount: 0, ForbiddenZeroBit: false, RefIdc: 0, UnitType: NalUnitTypeUnspecified, Data: make([]byte, 0)}
}

func (h *NAL) parseHeader(firstByte byte) {
	h.ForbiddenZeroBit = (((firstByte & 0x80) >> 7) == 1) // 0x80 = 0b10000000
	h.RefIdc = (firstByte & 0x60) >> 5                    // 0x60 = 0b01100000
	h.UnitType = NalUnitType((firstByte & 0x1F) >> 0)     // 0x1F = 0b00011111
}
