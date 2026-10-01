package random

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const (
	gachaRemoteFPS    = 24
	gachaRemoteWidth  = 640
	gachaRemoteHeight = 360
)

// gachaStyle is one wish animation the renderer serves: its path next to
// /api/gif and the clip length in seconds.
type gachaStyle struct {
	Path     string
	Duration int
}

var (
	gachaStyleWish = gachaStyle{Path: "gacha", Duration: 7}
	gachaStyleBeta = gachaStyle{Path: "gachabeta", Duration: 11}
)

type gachaAPIRequest struct {
	Label  string `json:"label"`
	Rarity int    `json:"rarity"`
	FPS    int    `json:"fps"`
	Width  int    `json:"width"`
}

// gachaAPIEndpoint derives a wish renderer from the configured wheel
// endpoint: the same service serves /api/gacha and /api/gachabeta next to
// /api/gif, so the last path segment is swapped and no second URL needs
// configuring.
func gachaAPIEndpoint(rawURL, path string) (*url.URL, error) {
	endpoint, err := wheelAPIEndpoint(rawURL)
	if err != nil {
		return nil, err
	}
	base := *endpoint
	base.Path = strings.TrimSuffix(base.Path, "/")
	base.RawPath = ""
	return base.ResolveReference(&url.URL{Path: path}), nil
}

// RenderGacha returns the MP4 wish animation at path revealing label at
// rarity stars.
func (c wheelAPIClient) RenderGacha(ctx context.Context, path, label string, rarity int) ([]byte, error) {
	endpoint, err := gachaAPIEndpoint(c.URL, path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(label) == "" {
		return nil, fmt.Errorf("gacha api label empty")
	}
	if rarity < gachaMinRarity || rarity > gachaMaxRarity {
		return nil, fmt.Errorf("gacha api rarity %d out of range", rarity)
	}
	body, err := json.Marshal(gachaAPIRequest{
		Label:  label,
		Rarity: rarity,
		FPS:    gachaRemoteFPS,
		Width:  gachaRemoteWidth,
	})
	if err != nil {
		return nil, fmt.Errorf("gacha api request encode failed: %w", err)
	}
	return c.post(ctx, endpoint, body, "video/mp4", isMP4)
}

// isMP4 checks for the ISO base media "ftyp" box that opens every MP4.
func isMP4(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[4:8], []byte("ftyp"))
}

func renderGachaAnimation(ctx context.Context, style gachaStyle, label string, rarity int) (wheelAnimation, error) {
	data, err := newWheelAPIClientFromEnv().RenderGacha(ctx, style.Path, label, rarity)
	if err != nil {
		return wheelAnimation{}, err
	}
	return wheelAnimation{
		Data:     data,
		Duration: style.Duration,
		Width:    gachaRemoteWidth,
		Height:   gachaRemoteHeight,
	}, nil
}
