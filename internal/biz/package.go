package biz

import (
	"strconv"
	"strings"
)

const MaxPackageImageBytes = 5 << 20

type Package struct {
	ID          int64
	Name        string
	Description string
	AmountFen   int64
	ReleaseDays int64
	Image       string
	Status      int32
	ProductID   int64
	SortOrder   int64
}

func ParseReleaseDaysQuery(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || !ValidReleaseDays(n) {
		return 0, ErrReleaseDays
	}
	return n, nil
}

func ClonePackage(p *Package) *Package {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// PackageImageExt 只认 jpg/png/webp；超过 5MB 或空内容直接拒绝。按文件头判断，不看文件名。
func PackageImageExt(data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrImageRequired
	}
	if len(data) > MaxPackageImageBytes {
		return "", ErrImageTooLarge
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return ".jpg", nil
	}
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png", nil
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp", nil
	}
	return "", ErrImageType
}
