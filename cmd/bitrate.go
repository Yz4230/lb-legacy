package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

type Bitrate struct {
	bitsPerSecond uint64
}

func newBitrate(bitsPerSecond uint64) *Bitrate {
	return &Bitrate{bitsPerSecond}
}

func (b *Bitrate) BytesPerSecond() uint64 {
	return b.bitsPerSecond / 8
}

func (b *Bitrate) BitsPerSecond() uint64 {
	return b.bitsPerSecond
}

func (b *Bitrate) String() string {
	// format to bps, kbps, Mbps, Gbps
	switch {
	case b.bitsPerSecond < 1<<10:
		return fmt.Sprintf("%dbps", b.bitsPerSecond)
	case b.bitsPerSecond < 1<<20:
		return fmt.Sprintf("%.2fkbps", float64(b.bitsPerSecond)/(1<<10))
	case b.bitsPerSecond < 1<<30:
		return fmt.Sprintf("%.2fMbps", float64(b.bitsPerSecond)/(1<<20))
	default:
		return fmt.Sprintf("%.2fGbps", float64(b.bitsPerSecond)/(1<<30))
	}
}

func parseBitrate(s string) (*Bitrate, error) {
	// 50bps, 100kbps, 1Mbps, 1Gbps
	if strings.HasSuffix(s, "bps") {
		s = strings.TrimSuffix(s, "bps")
		bitsPerSecond, err := parseUint(s)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse bps")
		}
		return newBitrate(bitsPerSecond), nil
	}
	// 50B/s, 100KB/s, 1MB/s, 1GB/s
	if strings.HasSuffix(s, "B/s") {
		s = strings.TrimSuffix(s, "B/s")
		bytesPerSecond, err := parseUint(s)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse b/s")
		}
		return newBitrate(bytesPerSecond * 8), nil
	}

	return nil, errors.New("Invalid bitrate format")
}

func parseUint(s string) (uint64, error) {
	unit := uint64(1)
	switch {
	case strings.HasSuffix(s, "k"):
		unit = 1_000
		s = strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "M"):
		unit = 1_000_000
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "G"):
		unit = 1_000_000_000
		s = strings.TrimSuffix(s, "G")
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "Failed to parse uint")
	}
	return v * unit, nil
}
