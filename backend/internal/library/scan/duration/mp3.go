package duration

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// MPEG version groups, used to index sample-rate and bitrate tables.
const (
	mpeg1   = 0
	mpeg2   = 1
	mpeg2_5 = 2
)

// Sample rates in Hz, by version group then index.
var mp3SampleRates = [3][3]int{
	mpeg1:   {44100, 48000, 32000},
	mpeg2:   {22050, 24000, 16000},
	mpeg2_5: {11025, 12000, 8000},
}

// Layer III bitrates in kbps: table 0 for MPEG1, table 1 for MPEG2/2.5.
// Index 0 (free) and 15 (bad) are unsupported.
var mp3BitrateKbps = [2][16]int{
	0: {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
	1: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
}

// MP3 estimates an MP3's duration. When the file carries a Xing/Info VBR
// header (the overwhelming majority of real-world VBR encoders write one),
// duration is computed exactly from its frame count. Otherwise, duration is
// estimated from the first frame's bitrate and the remaining file size,
// which is exact for CBR files and an approximation for the rare
// VBR-without-Xing-header file.
//
// Only MPEG 1/2/2.5 Layer III is supported (the layer essentially all
// real-world ".mp3" files use). f must be positioned anywhere — MP3 only
// ever reads at explicit offsets.
func MP3(f *os.File) (time.Duration, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	fileSize := info.Size()

	audioStart, err := skipID3v2(f)
	if err != nil {
		return 0, err
	}

	frameStart, h, err := findFrameSync(f, audioStart, fileSize)
	if err != nil {
		return 0, err
	}

	versionBits := (h >> 19) & 0x3
	layerBits := (h >> 17) & 0x3
	if layerBits != 0x1 {
		return 0, fmt.Errorf("unsupported MPEG layer (only Layer III is supported)")
	}

	var versionGroup, bitrateTable, samplesPerFrame int
	switch versionBits {
	case 0b11:
		versionGroup, bitrateTable, samplesPerFrame = mpeg1, 0, 1152
	case 0b10:
		versionGroup, bitrateTable, samplesPerFrame = mpeg2, 1, 576
	case 0b00:
		versionGroup, bitrateTable, samplesPerFrame = mpeg2_5, 1, 576
	default:
		return 0, fmt.Errorf("reserved MPEG version")
	}

	sampleRateIndex := (h >> 10) & 0x3
	if sampleRateIndex == 0x3 {
		return 0, fmt.Errorf("reserved sample rate index")
	}
	sampleRate := mp3SampleRates[versionGroup][sampleRateIndex]

	bitrateIndex := (h >> 12) & 0xF
	if bitrateIndex == 0 || bitrateIndex == 0xF {
		return 0, fmt.Errorf("unsupported bitrate index")
	}
	bitrateBps := mp3BitrateKbps[bitrateTable][bitrateIndex] * 1000

	channelMode := (h >> 6) & 0x3
	sideInfoSize := 32
	switch {
	case versionGroup == mpeg1 && channelMode == 0x3:
		sideInfoSize = 17
	case versionGroup == mpeg1:
		sideInfoSize = 32
	case channelMode == 0x3:
		sideInfoSize = 9
	default:
		sideInfoSize = 17
	}

	if frames, ok := readXingFrameCount(f, frameStart+4+int64(sideInfoSize)); ok {
		totalSamples := uint64(frames) * uint64(samplesPerFrame)
		return secondsToDuration(float64(totalSamples), float64(sampleRate)), nil
	}

	if bitrateBps <= 0 {
		return 0, fmt.Errorf("invalid bitrate")
	}
	audioBytes := fileSize - frameStart
	return secondsToDuration(float64(audioBytes)*8, float64(bitrateBps)), nil
}

// skipID3v2 returns the byte offset just past an ID3v2 tag at the start of
// the file, or 0 if there isn't one.
func skipID3v2(f *os.File) (int64, error) {
	var hdr [10]byte
	if _, err := f.ReadAt(hdr[:], 0); err != nil {
		return 0, fmt.Errorf("reading header: %w", err)
	}
	if string(hdr[0:3]) != "ID3" {
		return 0, nil
	}
	// The size is a "synchsafe" integer: only the low 7 bits of each byte
	// are significant. A conformant encoder always zeroes the high bit, but
	// a malformed/corrupt tag might not — mask it off explicitly rather
	// than trusting that, or a bad high bit silently inflates size and
	// throws off every offset computed from it.
	size := int64(hdr[6]&0x7F)<<21 | int64(hdr[7]&0x7F)<<14 | int64(hdr[8]&0x7F)<<7 | int64(hdr[9]&0x7F)
	return 10 + size, nil
}

// findFrameSyncScanWindow bounds how much of the file findFrameSync reads
// into memory per chunk while scanning for a sync pattern.
const findFrameSyncScanWindow = 4096

// findFrameSync scans forward from start for a valid-looking MPEG Layer III
// frame sync (11 set sync bits), returning its offset and 4-byte header
// value as a big-endian uint32. It reads in chunks rather than issuing one
// syscall per candidate byte offset.
func findFrameSync(f *os.File, start, end int64) (int64, uint32, error) {
	chunk := make([]byte, findFrameSyncScanWindow)
	for base := start; base+4 <= end; base += int64(len(chunk)) - 3 {
		n, err := f.ReadAt(chunk, base)
		if base+int64(n) > end {
			n = int(end - base)
		}
		for i := 0; i+4 <= n; i++ {
			h := binary.BigEndian.Uint32(chunk[i : i+4])
			if h>>21&0x7FF == 0x7FF && (h>>17)&0x3 == 0x1 {
				return base + int64(i), h, nil
			}
		}
		if err != nil {
			break
		}
	}
	return 0, 0, fmt.Errorf("no MPEG Layer III frame sync found")
}

// readXingFrameCount reads a Xing/Info VBR header at offset, if present,
// returning its frame count.
func readXingFrameCount(f *os.File, offset int64) (uint32, bool) {
	buf := make([]byte, 8)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return 0, false
	}
	tag := string(buf[0:4])
	if tag != "Xing" && tag != "Info" {
		return 0, false
	}
	flags := binary.BigEndian.Uint32(buf[4:8])
	if flags&0x1 == 0 {
		return 0, false
	}
	frameBuf := make([]byte, 4)
	if _, err := f.ReadAt(frameBuf, offset+8); err != nil {
		return 0, false
	}
	return binary.BigEndian.Uint32(frameBuf), true
}
