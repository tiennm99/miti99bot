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
	gachaRemoteFPS = 24
	// gachaRemoteWidth is the requested long edge; the wish is portrait, so
	// the video is gachaVideoWidth x gachaRemoteWidth.
	gachaRemoteWidth = 640
	gachaVideoWidth  = 360
	gachaDuration    = 6
)

type gachaAPIRequest struct {
	Label  string `json:"label"`
	Rarity int    `json:"rarity"`
	FPS    int    `json:"fps"`
	Width  int    `json:"width"`
}

// gachaAPIEndpoint derives the wish renderer from the configured wheel
// endpoint: the same service serves /api/gacha next to /api/gif, so the last
// path segment is swapped and no second URL needs configuring.
func gachaAPIEndpoint(rawURL string) (*url.URL, error) {
	endpoint, err := wheelAPIEndpoint(rawURL)
	if err != nil {
		return nil, err
	}
	base := *endpoint
	base.Path = strings.TrimSuffix(base.Path, "/")
	base.RawPath = ""
	return base.ResolveReference(&url.URL{Path: "gacha"}), nil
}

// RenderGacha returns the MP4 wish animation revealing label at rarity stars.
func (c wheelAPIClient) RenderGacha(ctx context.Context, label string, rarity int) ([]byte, error) {
	endpoint, err := gachaAPIEndpoint(c.URL)
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

func renderGachaAnimation(ctx context.Context, label string, rarity int) (wheelAnimation, error) {
	data, err := newWheelAPIClientFromEnv().RenderGacha(ctx, label, rarity)
	if err != nil {
		return wheelAnimation{}, err
	}
	return wheelAnimation{
		Data:     data,
		Duration: gachaDuration,
		Width:    gachaVideoWidth,
		Height:   gachaRemoteWidth,
	}, nil
}
