package audio

import (
	"encoding/binary"
	"io"
	"os"
)

type File struct {
	file *os.File
}

func NewFile(path string, sampleRate int, bitsPerSample int, numChannels int) (*File, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	soundFile := &File{
		file: file,
	}

	if err = soundFile.WriteHeaders(sampleRate, bitsPerSample, numChannels); err != nil {
		return nil, err
	}

	return soundFile, nil
}

func (f *File) WriteHeaders(sampleRate int, bitsPerSample int, numChannels int) error {
	var err error
	err = binary.Write(f.file, binary.LittleEndian, []byte("RIFF"))
	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint32(0)) // File size To be filled in later
	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, []byte("WAVE"))
	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, []byte("fmt "))
	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint32(16)) // Chunk size

	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint16(3)) // PCM

	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint16(numChannels)) // Mono

	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint32(sampleRate)) // Sample rate

	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint32(sampleRate*bitsPerSample*numChannels/8)) // Byte rate

	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint16(bitsPerSample*numChannels/8)) // Block align

	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint16(bitsPerSample)) // Bits per sample

	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, []byte("data"))

	if err != nil {
		return err
	}

	err = binary.Write(f.file, binary.LittleEndian, uint32(0)) // Data size To be filled in later

	if err != nil {
		return err
	}

	return nil
}

func (f *File) WriteSamples(samples []float32) (int, error) {
	return len(samples), binary.Write(f.file, binary.LittleEndian, samples)
}

// [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
// [R, I, F, F, 0, 0, 0, 0, W, A, V, E, f, m, t, 0x20, 0x10, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x02, 0x00, 0x44, 0xAC, 0x00, 0x00, 0x10, 0xB1, 0x02, 0x00, 0x04, 0x00, 0x10, 0x00, d, a, t, a, 0, 0, 0, 0]
func (f *File) Close() error {

	pos, err := f.file.Seek(0, io.SeekCurrent)

	if err != nil {
		return err
	}

	if _, err = f.file.Seek(4, io.SeekStart); err != nil {
		return err
	}

	if err = binary.Write(f.file, binary.LittleEndian, uint32(pos-8)); err != nil {
		return err
	}

	if _, err = f.file.Seek(40, io.SeekStart); err != nil {
		return err
	}

	if err = binary.Write(f.file, binary.LittleEndian, uint32(pos-44)); err != nil {
		return err
	}

	return nil
}
