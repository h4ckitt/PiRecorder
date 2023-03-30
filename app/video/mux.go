package video

import (
	"bytes"
	"io"
)

type Mux struct {
	camStream io.Reader
	frame     *[]byte
	lock      chan struct{}
}

func NewMux(stream io.Reader) *Mux {
	m := &Mux{
		camStream: stream,
		lock:      make(chan struct{}, 1),
		frame:     &[]byte{},
	}

	go m.Start()

	return m
}

func (m *Mux) Start() {
	buf := make([]byte, 16348)
	var (
		startIndex int
		endIndex   int
		tmp        []byte
	)

	for {
		n, err := m.camStream.Read(buf)
		if err != nil {
			continue
		}

		if len(tmp) > 0 {
			endIndex = bytes.Index(buf[:n], []byte{0xFF, 0xD9})
			if endIndex == -1 {
				tmp = append(tmp, buf[:n]...)
				continue
			}

			m.Lock()
			*m.frame = nil
			*m.frame = append(tmp, buf[:endIndex+2]...)
			m.Unlock()
			tmp = nil
		}

		startIndex = bytes.Index(buf[:n], []byte{0xFF, 0xD8})
		if startIndex == -1 {
			continue
		}

		endIndex = bytes.Index(buf[startIndex:], []byte{0xFF, 0xD9})
		if endIndex == -1 {
			tmp = append(tmp, buf[startIndex:]...)
			continue
		}

		m.Lock()
		*m.frame = nil
		*m.frame = buf[startIndex : startIndex+endIndex+2]
		m.Unlock()
		tmp = nil
		endIndex = startIndex + endIndex + 2

		for endIndex < len(buf[:n]) {

			startIndex = bytes.Index(buf[endIndex:], []byte{0xFF, 0xD8})

			if startIndex == -1 {
				break
			}

			startIndex += endIndex

			endIndex = bytes.Index(buf[startIndex:], []byte{0xFF, 0xD9})
			if endIndex == -1 {
				tmp = append(tmp, buf[startIndex+endIndex:]...)
				break
			}

			m.Lock()
			*m.frame = nil
			*m.frame = buf[startIndex : startIndex+endIndex+2]
			m.Unlock()
			endIndex = startIndex + endIndex + 2
		}
	}
}

func (m *Mux) GetFrame() []byte {
	m.Lock()
	b := *m.frame
	m.Unlock()
	return b
}

func (m *Mux) Lock() {
	m.lock <- struct{}{}
}

func (m *Mux) Unlock() {
	<-m.lock
}
