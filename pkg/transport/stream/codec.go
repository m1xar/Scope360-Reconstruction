package stream

import (
	"encoding/binary"
	"fmt"
	"io"
)

const DefaultMaxFrameSize = 16 << 20

type Codec interface {
	ReadFrame(r io.Reader) ([]byte, error)
	WriteFrame(w io.Writer, payload []byte) error
}

type LittleEndianFrameCodec struct {
	MaxFrameSize uint32
}

func (c LittleEndianFrameCodec) ReadFrame(r io.Reader) ([]byte, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return nil, err
	}
	limit := c.MaxFrameSize
	if limit == 0 {
		limit = DefaultMaxFrameSize
	}
	if size > limit {
		return nil, fmt.Errorf("stream frame too large: %d > %d", size, limit)
	}
	payload := make([]byte, size)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

func (c LittleEndianFrameCodec) WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > int(^uint32(0)) {
		return fmt.Errorf("stream frame too large: %d", len(payload))
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(payload))); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

type BigEndianFrameCodec struct {
	MaxFrameSize uint32
}

func (c BigEndianFrameCodec) ReadFrame(r io.Reader) ([]byte, error) {
	var size uint32
	if err := binary.Read(r, binary.BigEndian, &size); err != nil {
		return nil, err
	}
	limit := c.MaxFrameSize
	if limit == 0 {
		limit = DefaultMaxFrameSize
	}
	if size > limit {
		return nil, fmt.Errorf("stream frame too large: %d > %d", size, limit)
	}
	payload := make([]byte, size)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

func (c BigEndianFrameCodec) WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > int(^uint32(0)) {
		return fmt.Errorf("stream frame too large: %d", len(payload))
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(payload))); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}
