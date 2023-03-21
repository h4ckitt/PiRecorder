package app

import (
	"bytes"
	"fmt"
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
		//tmpBuf     []byte
		tmp []byte
	)
	// [1, 2, 3, 4, 5, 6, 7, 8, 9, 0]
	// [1, 2, 3, 4, 5]
	// [0, 1, 2, 3, 4][5, 6, 7, 8, 9]
	for {
		n, err := m.camStream.Read(buf)
		if err != nil {
			continue
		}

		if len(tmp) > 0 {
			endIndex = bytes.Index(buf[:n], []byte{0xFF, 0xD9})
			if endIndex == -1 {
				fmt.Println("Really Huge Frame")
				tmp = append(tmp, buf[:n]...)
				continue
			}

			//fmt.Println("Tmp: ", tmp[len(tmp)-2:])
			m.Lock()
			*m.frame = nil
			*m.frame = append(tmp, buf[:endIndex+2]...)
			m.Unlock()
			//fmt.Println("Frame: ", (*m.frame)[len(*m.frame)-2:])
			tmp = nil
		}

		//fmt.Println("Read: ", n)

		startIndex = bytes.Index(buf[:n], []byte{0xFF, 0xD8})
		if startIndex == -1 {
			continue
		}
		// [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
		// [1, 2, 3, 4, 5, 6, 7, 8, 9, 0]

		endIndex = bytes.Index(buf[startIndex+2:], []byte{0xFF, 0xD9})
		if endIndex == -1 {
			tmp = append(tmp, buf[startIndex:]...)
			continue
		}

		m.Lock()
		*m.frame = nil
		*m.frame = buf[startIndex : startIndex+endIndex+4]
		m.Unlock()
		tmp = nil
		endIndex = startIndex + endIndex + 4

		for endIndex < len(buf[:n]) {

			startIndex = bytes.Index(buf[endIndex:], []byte{0xFF, 0xD8})

			if startIndex == -1 {
				break
			}

			fmt.Println("Found another frame")

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
			fmt.Println("Frame: ", (*m.frame)[:2])
			fmt.Println("End Of Frame: ", (*m.frame)[len(*m.frame)-2:])
			endIndex = startIndex + endIndex + 2

		}
		/*fmt.Println("Read: ", n)
		b := buf[:n]
		//buf = buf[:n]
		//fmt.Println("Buf: ", len(buf))
		startIndex = bytes.Index(b, []byte{0xFF, 0xD8})
		//fmt.Println("Start Index: ", startIndex)
		//fmt.Println("Found Start Index")
		if startIndex == -1 {
			//	fmt.Println("Completely wrong")
			continue
		}
		//fmt.Println("Buf: ", buf[startIndex+1])
		if len(tmpBuf) > 0 {
			tmp = append(tmp, tmpBuf...)
			tmpBuf = nil
		}
		tmp = append(tmp, buf[startIndex:]...)
		//fmt.Println(tmp)
		//fmt.Println("Tmp: ", tmp[:2])

		endIndex = -1
		for endIndex == -1 {
			endIndex = bytes.Index(buf, []byte{0xFF, 0xD9})

			_, err = m.camStream.Read(buf)
			if err != nil {
				fmt.Println("ERROR!!!")
				break
			}
			tmp = append(tmp, buf...)
			//continue
		}
		//fmt.Println("Found End Index")
		//fmt.Println("Tmp: ", tmp[:2])
		tmp = append(tmp, buf[:endIndex+2]...)
		startIndex = bytes.Index(buf[endIndex+2:], []byte{0xFF, 0xD8})
		if startIndex != -1 {
			startIndex += endIndex + 2
			tmpBuf = append(tmpBuf, buf[startIndex:]...)
			//fmt.Println("Stashing Temporary BUffer")
		}
		m.Lock()
		*m.frame = tmp
		tmp = nil
		fmt.Println("Wrote ", len(*m.frame), " bytes to the frame")
		//	fmt.Println("Frame: ", m.frame[:2])
		m.Unlock()
		//fmt.Println("HEre")*/
	}
}

func (m *Mux) GetFrame() []byte {
	m.Lock()
	b := *m.frame
	m.Unlock()
	return b
}

func (m *Mux) Read(b []byte) (int, error) {

	if len(*m.frame) == 0 {
		fmt.Println("No Frame to read")
		return 0, nil
	}
	m.Lock()
	defer m.Unlock()

	fmt.Println("Read Frame: ", (*m.frame)[:2])
	fmt.Println(len(*m.frame), " bytes in the frame")
	if len(b) > len(*m.frame) {
		return copy(b, *m.frame), nil
	}
	n := copy(b, *m.frame)
	b = append(b, (*m.frame)[n:]...)
	fmt.Println("Read Buffer: ", b[:2])
	fmt.Println(len(b), " bytes in the buffer")
	return len(b), nil
}

func (m *Mux) Lock() {
	m.lock <- struct{}{}
}

func (m *Mux) Unlock() {
	<-m.lock
}
