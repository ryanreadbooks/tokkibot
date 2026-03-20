package audio

import (
	"bytes"
	"encoding/binary"
	"strings"

	codec "github.com/yapingcat/gomedia/go-codec"
	gomp4 "github.com/yapingcat/gomedia/go-mp4"
	"github.com/yapingcat/gomedia/go-ogg"
)

// DetectDurationMs detects audio duration in milliseconds from raw bytes and MIME type.
// Returns 0 if the format is unsupported or parsing fails.
func DetectDurationMs(data []byte, mimeType string) int64 {
	switch {
	case strings.Contains(mimeType, "mpeg") || strings.Contains(mimeType, "mp3"):
		return detectMp3(data)
	case strings.Contains(mimeType, "aac"):
		return detectAac(data)
	case strings.Contains(mimeType, "mp4") || strings.Contains(mimeType, "m4a"):
		return detectMp4(data)
	case strings.Contains(mimeType, "ogg"):
		return detectOgg(data)
	case strings.Contains(mimeType, "wav") || strings.Contains(mimeType, "wave"):
		return detectWav(data)
	case strings.Contains(mimeType, "flac"):
		return detectFlac(data)
	default:
		return 0
	}
}

func detectMp3(data []byte) int64 {
	var totalSamples int64
	var sampleRate int
	codec.SplitMp3Frames(data, func(head *codec.MP3FrameHead, frame []byte) {
		totalSamples += int64(head.SampleSize)
		if sampleRate == 0 {
			sampleRate = head.GetSampleRate()
		}
	})
	if sampleRate == 0 {
		return 0
	}
	return totalSamples * 1000 / int64(sampleRate)
}

func detectAac(data []byte) int64 {
	var frameCount int
	var sampleRate int
	hdr := codec.NewAdtsFrameHeader()
	codec.SplitAACFrame(data, func(aac []byte) {
		frameCount++
		if sampleRate == 0 && len(aac) >= 7 {
			hdr.Decode(aac)
			idx := int(hdr.Fix_Header.Sampling_frequency_index)
			if idx < len(codec.AAC_Sampling_Idx) {
				sampleRate = codec.AAC_Sampling_Idx[idx]
			}
		}
	})
	if sampleRate == 0 {
		return 0
	}
	return int64(frameCount) * 1024 * 1000 / int64(sampleRate)
}

func detectMp4(data []byte) int64 {
	demuxer := gomp4.CreateMp4Demuxer(bytes.NewReader(data))
	if _, err := demuxer.ReadHead(); err != nil {
		return 0
	}
	info := demuxer.GetMp4Info()
	if info.Timescale == 0 {
		return 0
	}
	return int64(info.Duration) * 1000 / int64(info.Timescale)
}

func detectOgg(data []byte) int64 {
	demuxer := ogg.NewDemuxer()
	var lastGranule uint64
	demuxer.OnPacket = func(_ uint32, granule uint64, _ []byte, _ int) {
		if granule > lastGranule {
			lastGranule = granule
		}
	}
	_ = demuxer.Input(data)
	params := demuxer.GetAudioParam()
	if params == nil || params.SampleRate == 0 {
		return 0
	}
	return int64(lastGranule) * 1000 / int64(params.SampleRate)
}

func detectWav(data []byte) int64 {
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return 0
	}

	var byteRate uint32
	var dataSize uint32
	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		switch chunkID {
		case "fmt ":
			if offset+20 <= len(data) {
				byteRate = binary.LittleEndian.Uint32(data[offset+16 : offset+20])
			}
		case "data":
			dataSize = chunkSize
		}
		offset += 8 + int(chunkSize)
		if chunkSize%2 != 0 {
			offset++
		}
	}
	if byteRate == 0 {
		return 0
	}
	return int64(dataSize) * 1000 / int64(byteRate)
}

func detectFlac(data []byte) int64 {
	// fLaC(4) + block header(4) + STREAMINFO(34) = 42 bytes minimum
	if len(data) < 42 || string(data[0:4]) != "fLaC" {
		return 0
	}

	// STREAMINFO data starts at byte 8
	si := data[8:]
	sampleRate := uint32(si[10])<<12 | uint32(si[11])<<4 | uint32(si[12]>>4)
	if sampleRate == 0 {
		return 0
	}
	totalHigh := uint64(si[13] & 0x0F)
	totalLow := uint64(binary.BigEndian.Uint32(si[14:18]))
	totalSamples := totalHigh<<32 | totalLow
	return int64(totalSamples) * 1000 / int64(sampleRate)
}
