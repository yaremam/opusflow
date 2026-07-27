package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// wvBlockHeaderSize is the fixed size of a WavPack block header (the
// on-disk WavpackHeader struct: ckID, ckSize, version, track_no, index_no,
// total_samples, block_index, block_samples, flags, crc).
const wvBlockHeaderSize = 32

// wvUnknownTotalSamples is the sentinel a WavPack encoder writes into the
// first block's total_samples field when the input's length wasn't known
// upfront (e.g. encoded from a live/streaming source).
const wvUnknownTotalSamples = 0xFFFFFFFF

// wvSampleRates is WavPack's fixed table of standard sample rates, indexed
// by the 4-bit rate index packed into a block's flags (bits 23-26). Index
// 15 means "not one of these" — the real rate then lives in a decoder-config
// extension sub-block this parser doesn't chase, so it's treated as an
// error rather than guessed (same "don't have what's needed, don't guess"
// stance mp3.go already takes without a VBR header).
var wvSampleRates = [15]uint32{
	6000, 8000, 9600, 11025, 12000, 16000, 22050, 24000, 32000,
	44100, 48000, 64000, 88200, 96000, 192000,
}

type wvBlockHeader struct {
	ckSize       uint32
	totalSamples uint32
	blockSamples uint32
	flags        uint32
}

func readWVBlockHeader(f *os.File) (wvBlockHeader, error) {
	var raw [wvBlockHeaderSize]byte
	if _, err := io.ReadFull(f, raw[:]); err != nil {
		return wvBlockHeader{}, err
	}
	if string(raw[0:4]) != "wvpk" {
		return wvBlockHeader{}, fmt.Errorf("not a WavPack block (bad ckID)")
	}
	return wvBlockHeader{
		ckSize:       binary.LittleEndian.Uint32(raw[4:8]),
		totalSamples: binary.LittleEndian.Uint32(raw[12:16]),
		blockSamples: binary.LittleEndian.Uint32(raw[20:24]),
		flags:        binary.LittleEndian.Uint32(raw[24:28]),
	}, nil
}

func wvSampleRate(flags uint32) (uint32, error) {
	idx := (flags >> 23) & 0xF
	if int(idx) >= len(wvSampleRates) {
		return 0, fmt.Errorf("non-standard WavPack sample rate (index %d)", idx)
	}
	return wvSampleRates[idx], nil
}

// WavPack parses a .wv file's duration from its first block header's
// total_samples field — present precisely so a reader doesn't have to scan
// the whole file. Falls back to hopping block-to-block (reading only each
// header, never decoding audio) summing block_samples, for the rare file
// encoded without a known total length upfront. f must be positioned
// anywhere — WavPack seeks to the start itself before reading.
func WavPack(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	first, err := readWVBlockHeader(f)
	if err != nil {
		return 0, fmt.Errorf("reading WavPack block header: %w", err)
	}
	rate, err := wvSampleRate(first.flags)
	if err != nil {
		return 0, err
	}
	if rate == 0 {
		return 0, fmt.Errorf("invalid WavPack sample rate")
	}

	if first.totalSamples != wvUnknownTotalSamples {
		return secondsToDuration(float64(first.totalSamples), float64(rate)), nil
	}

	// Unknown upfront length: hop through every block, summing samples.
	total := uint64(first.blockSamples)
	if err := skipRestOfBlock(f, first.ckSize); err != nil {
		return 0, fmt.Errorf("skipping WavPack block: %w", err)
	}
	for {
		// Any failure to read a further block — true EOF, or running into
		// a trailing APEv2 tag appended after the last audio block (a
		// tagged .wv's normal shape) — means there are no more audio
		// blocks to sum, not that something's wrong; stop and return what
		// was accumulated, the same "break rather than error" stance
		// wav.go's own chunk loop already takes.
		hdr, err := readWVBlockHeader(f)
		if err != nil {
			break
		}
		total += uint64(hdr.blockSamples)
		if err := skipRestOfBlock(f, hdr.ckSize); err != nil {
			break
		}
	}

	return secondsToDuration(float64(total), float64(rate)), nil
}

// skipRestOfBlock advances past whatever of the current block wasn't part
// of the 32-byte header already read — ckSize is the block's total size
// minus the 8 bytes (ckID+ckSize) that precede it.
func skipRestOfBlock(f *os.File, ckSize uint32) error {
	remaining := int64(ckSize) - (wvBlockHeaderSize - 8)
	if remaining < 0 {
		return fmt.Errorf("invalid WavPack block size")
	}
	_, err := f.Seek(remaining, io.SeekCurrent)
	return err
}
