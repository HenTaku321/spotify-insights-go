package spotify

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/zmb3/spotify/v2"
)

// PlaybackEntry 是数据库列表 playback-history 中的存储格式
type PlaybackEntry struct {
	ID       string `json:"id"`
	PlayedAt string `json:"played_at"`
}

func (c *Client) getRecentlyPlayedTracksFromSpotify() ([]PlaybackEntry, error) {
	recentlyPlayedTracks, err := c.C.PlayerRecentlyPlayedOpt(c.Ctx, &spotify.RecentlyPlayedOptions{Limit: 50})
	if err != nil {
		return nil, err
	}

	var playbackHistory []PlaybackEntry

	for _, item := range recentlyPlayedTracks {
		playbackHistory = append(playbackHistory, PlaybackEntry{item.Track.ID.String(), item.PlayedAt.Local().Format(time.DateTime)})
	}

	return playbackHistory, nil
}

func (c *Client) truncate(dbc dbClient, playbackHistory []PlaybackEntry) ([]PlaybackEntry, error) {
	lastPlayed, err := dbc.GetSliceByIndex("playback-history", -1)
	if err != nil {
		return nil, err
	}

	if lastPlayed == "" {
		return playbackHistory, nil
	}

	pe := PlaybackEntry{}

	err = json.Unmarshal([]byte(lastPlayed), &pe)
	if err != nil {
		return nil, err
	}

	lastPlayedIndex := 0

	for ; lastPlayedIndex < len(playbackHistory); lastPlayedIndex++ {
		if playbackHistory[lastPlayedIndex].PlayedAt == pe.PlayedAt {
			break
		}
	}

	return playbackHistory[:lastPlayedIndex], nil
}

// saveRecentlyPlayedTracks 追加最近收听的歌曲并统计每日收听量, 并以 *Map 的 JSON 格式存储信息
func (c *Client) saveRecentlyPlayedTracks(dbc dbClient) error {
	recentlyPlayedTracks, err := c.getRecentlyPlayedTracksFromSpotify()
	if err != nil {
		return err
	}

	truncatedPlaybackHistory, err := c.truncate(dbc, recentlyPlayedTracks)
	if err != nil {
		return err
	}

	if len(truncatedPlaybackHistory) == 0 {
		return nil
	}

	slices.Reverse(truncatedPlaybackHistory)

	days := map[string]int{}
	var playbackHistory []string

	for _, entry := range truncatedPlaybackHistory {
		j, err := json.Marshal(&PlaybackEntry{entry.ID, entry.PlayedAt})
		if err != nil {
			return err
		}

		// 日期部分
		days[entry.PlayedAt[:10]]++

		playbackHistory = append(playbackHistory, string(j))
	}

	err = dbc.AppendSlice("playback-history", playbackHistory)
	if err != nil {
		return err
	}

	for day, count := range days {
		t, err := time.Parse(time.DateOnly, day)
		if err != nil {
			return err
		}

		err = c.savePlaybackRangeOnADay(dbc, t, count)
		if err != nil {
			return err
		}
	}

	var truncatedRecentlyPlayedTracks []PlayedTrack

	for _, entry := range truncatedPlaybackHistory {
		track, err := c.getTrackCache(dbc, entry.ID)
		if err != nil {
			return err
		}

		truncatedRecentlyPlayedTracks = append(truncatedRecentlyPlayedTracks, PlayedTrack{*track, entry.PlayedAt})
	}

	return c.savePlaybackCounts(dbc, truncatedRecentlyPlayedTracks)
}

func (c *Client) GetPlaybackHistory(dbc dbClient, start, stop int64) ([]PlayedTrack, error) {
	playbackHistory, err := dbc.GetSlice("playback-history", start, stop)
	if err != nil {
		return nil, err
	}

	var playedTracks []PlayedTrack

	for _, entry := range playbackHistory {
		// ID 部分
		track, err := c.getTrackCache(dbc, entry[7:29])
		if err != nil {
			return nil, err
		}

		if track == nil {
			continue
		}

		// 日期部分
		playedTracks = append(playedTracks, PlayedTrack{*track, entry[44:63]})
	}

	return playedTracks, nil
}

// GetPlaybackHistoryByIndex 若播放记录中的 ID 对应的 Track 不存在会返回 nil
func (c *Client) GetPlaybackHistoryByIndex(dbc dbClient, index int64) (*PlayedTrack, error) {
	entry, err := dbc.GetSliceByIndex("playback-history", index)
	if err != nil {
		return nil, err
	}

	if entry == "" {
		return nil, nil
	}

	pe := &PlaybackEntry{}

	err = json.Unmarshal([]byte(entry), pe)
	if err != nil {
		return nil, err
	}

	track, err := c.getTrackCache(dbc, pe.ID)
	if err != nil {
		return nil, err
	}

	if track == nil {
		return nil, nil
	}

	return &PlayedTrack{*track, pe.PlayedAt}, nil
}

func (c *Client) GetPlaybackHistoryIDs(dbc dbClient, start, stop int64) ([]string, error) {
	playbackHistorySli, err := dbc.GetSlice("playback-history", start, stop)
	if err != nil {
		return nil, err
	}

	var res []string
	playbackHistory := &PlaybackEntry{}

	for _, entry := range playbackHistorySli {
		err = json.Unmarshal([]byte(entry), playbackHistory)
		if err != nil {
			return nil, err
		}

		res = append(res, playbackHistory.ID)
	}

	return res, nil
}

// GetPlaybackHistoryIDByIndex 若播放记录中的 index 对应的信息不存在会返回 nil
func (c *Client) GetPlaybackHistoryIDByIndex(dbc dbClient, index int64) (string, error) {
	entry, err := dbc.GetSliceByIndex("playback-history", index)
	if err != nil {
		return "", err
	}

	if entry == "" {
		return "", nil
	}

	pe := &PlaybackEntry{}

	err = json.Unmarshal([]byte(entry), pe)
	if err != nil {
		return "", err
	}

	return pe.ID, nil
}
