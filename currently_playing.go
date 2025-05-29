package spotify

import "time"

type Playing struct {
	Track
	//InfoStructure InfoStructure `json:"info_structure"`
	//IsPlaying     bool          `json:"is_playing"`
	TimeStamp string `json:"timestamp"`
}

// FromSpotify
func (c *Client) GetCurrentlyPlayingTrack(dbc dbClient) (*Playing, error) {
	cp, err := c.C.PlayerCurrentlyPlaying(c.Ctx)
	if err != nil {
		return nil, err
	}

	if cp.Item == nil || !cp.Playing {
		return nil, nil
	}

	track, err := c.convertTrack(dbc, cp.Item)
	if err != nil {
		return nil, err
	}

	p := &Playing{*track, time.UnixMilli(int64(cp.Item.Duration)).UTC().Format(time.TimeOnly)}

	return p, nil
}
