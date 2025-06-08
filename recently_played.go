package spotify

import (
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/zmb3/spotify/v2"
)

// PlayedHistory 是数据库列表 played-history中 的存储格式
type PlayedHistory struct {
	ID       string `json:"id"`
	PlayedAt string `json:"played_at"`
	// EmbedURL string `json:"embed_url"`
}

func (c *Client) getRecentlyPlayedTracksFromSpotify() ([]PlayedHistory, error) {
	recentlyPlayedTracks, err := c.C.PlayerRecentlyPlayedOpt(c.Ctx, &spotify.RecentlyPlayedOptions{Limit: 50})
	if err != nil {
		return nil, err
	}

	var res []PlayedHistory

	for _, item := range recentlyPlayedTracks {
		res = append(res, PlayedHistory{item.Track.ID.String(), item.PlayedAt.Local().Format(time.DateTime)})
	}

	return res, nil
}

func (c *Client) truncate(dbc dbClient, playedHistory []PlayedHistory) ([]PlayedHistory, error) {
	latestPlayed, err := dbc.GetSliceByIndex("played-history", -1)
	if err != nil {
		return nil, err
	}

	if latestPlayed == "" {
		return playedHistory, nil
	}

	ph := PlayedHistory{}

	err = json.Unmarshal([]byte(latestPlayed), &ph)
	if err != nil {
		return nil, err
	}

	latestPlayedIndex := 0

	for ; latestPlayedIndex < len(playedHistory); latestPlayedIndex++ {
		if playedHistory[latestPlayedIndex].PlayedAt == ph.PlayedAt {
			break
		}
	}

	return playedHistory[:latestPlayedIndex], nil
}

// saveRecentlyPlayedTracks 追加最近收听的歌曲并统计每日收听量, 并以 *Map 的 JSON 格式存储信息
func (c *Client) saveRecentlyPlayedTracks(dbc dbClient) error {
	days := map[string]int{}

	recentlyPlayedTracks, err := c.getRecentlyPlayedTracksFromSpotify()
	if err != nil {
		return err
	}

	truncatedPlayedHistory, err := c.truncate(dbc, recentlyPlayedTracks)
	if err != nil {
		return err
	}

	if len(truncatedPlayedHistory) == 0 {
		return nil
	}

	slices.Reverse(truncatedPlayedHistory)

	var ph []string

	for _, playedTrack := range truncatedPlayedHistory {
		j, err := json.Marshal(&PlayedHistory{playedTrack.ID, playedTrack.PlayedAt})
		if err != nil {
			return err
		}

		// 日期部分
		days[playedTrack.PlayedAt[:10]]++

		ph = append(ph, string(j))
	}

	err = dbc.AppendSlice("played-history", ph)
	if err != nil {
		return err
	}

	for day, count := range days {
		t, err := time.Parse(time.DateOnly, day)
		if err != nil {
			return err
		}

		err = c.savePlayedRangeOnADay(dbc, t, count)
		if err != nil {
			return err
		}

		slog.Debug("最近收听的歌曲保存成功", "日期", day, "数量", count)
	}

	var truncatedRecentlyPlayedTrack []PlayedTrack

	for _, played := range truncatedPlayedHistory {
		track, err := c.getTrackCache(dbc, played.ID)
		if err != nil {
			return err
		}

		truncatedRecentlyPlayedTrack = append(truncatedRecentlyPlayedTrack, PlayedTrack{*track, played.PlayedAt})
	}

	return c.savePlayedCount(dbc, truncatedRecentlyPlayedTrack)
}

func (c *Client) GetPlayedHistory(dbc dbClient, start, stop int64) ([]PlayedTrack, error) {
	playedHistory, err := dbc.GetSlice("played-history", start, stop)
	if err != nil {
		return nil, err
	}

	var res []PlayedTrack
	ph := PlayedHistory{}

	for _, played := range playedHistory {
		err = json.Unmarshal([]byte(played), &ph)
		if err != nil {
			return nil, err
		}

		track, err := c.getTrackCache(dbc, ph.ID)
		if err != nil {
			return nil, err
		}

		if track == nil {
			continue
		}

		res = append(res, PlayedTrack{*track, ph.PlayedAt})
	}

	return res, nil
}

// GetPlayedHistoryByIndex 若播放记录中的 ID 对应的 Track 不存在会返回 nil
func (c *Client) GetPlayedHistoryByIndex(dbc dbClient, index int64) (*PlayedTrack, error) {
	played, err := dbc.GetSliceByIndex("played-history", index)
	if err != nil {
		return nil, err
	}

	if played == "" {
		return nil, nil
	}

	ph := PlayedHistory{}

	err = json.Unmarshal([]byte(played), &ph)
	if err != nil {
		return nil, err
	}

	track, err := c.getTrackCache(dbc, ph.ID)
	if err != nil {
		return nil, err
	}

	if track == nil {
		return nil, nil
	}

	return &PlayedTrack{*track, ph.PlayedAt}, nil
}

func (c *Client) GetPlayedHistoryIDs(dbc dbClient, start, stop int64) ([]string, error) {
	playedHistory, err := dbc.GetSlice("played-history", start, stop)
	if err != nil {
		return nil, err
	}

	var res []string
	ph := PlayedHistory{}

	for _, played := range playedHistory {
		err = json.Unmarshal([]byte(played), &ph)
		if err != nil {
			return nil, err
		}

		res = append(res, ph.ID)
	}

	return res, nil
}

// GetPlayedHistoryIDByIndex 若播放记录中的 index 对应的信息不存在会返回 nil
func (c *Client) GetPlayedHistoryIDByIndex(dbc dbClient, index int64) (string, error) {
	played, err := dbc.GetSliceByIndex("played-history", index)
	if err != nil {
		return "", err
	}

	if played == "" {
		return "", nil
	}

	ph := PlayedHistory{}

	err = json.Unmarshal([]byte(played), &ph)
	if err != nil {
		return "", err
	}

	return ph.ID, nil
}
