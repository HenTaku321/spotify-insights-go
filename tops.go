package spotify

import (
	"encoding/json"
	"github.com/zmb3/spotify/v2"
	"log/slog"
	"sort"
	"strconv"
	"time"
)

func (c *Client) getTopArtistsFromSpotify(r spotify.Range) ([]Artist, error) {
	fap, err := c.C.CurrentUsersTopArtists(c.Ctx, spotify.Limit(50), spotify.Timerange(r))
	if err != nil {
		return nil, err
	}

	var res []Artist

	for _, artist := range fap.Artists {
		res = append(res, *c.convertArtist(&artist))
	}

	return res, nil
}

func (c *Client) saveTopArtists(dbc dbClient) error {
	m, err := dbc.GetMapInt64("updated-times", "monthly-top-artists")
	if err != nil {
		return err
	}

	h, err := dbc.GetMapInt64("updated-times", "half-yearly-top-artists")
	if err != nil {
		return err
	}

	y, err := dbc.GetMapInt64("updated-times", "yearly-top-artists")
	if err != nil {
		return err
	}

	// 距离上次更新超过一个月 或 第一次更新
	if int64(time.Now().Month()) > m || m == 0 {
		slog.Debug("正在更新 Spotify 艺术家月榜")

		artists, err := c.getTopArtistsFromSpotify(spotify.ShortTermRange)
		if err != nil {
			return err
		}

		var res []string

		for _, artist := range artists {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", artist.ID)
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, artist.ID, artist.toMap())
				if err != nil {
					return err
				}
			}
			res = append(res, artist.ID)
		}

		j, err := json.Marshal(res)
		if err != nil {
			return err
		}

		err = dbc.SetMap("monthly-top-artists", strconv.Itoa(int(time.Now().Month())), string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "monthly-top-artists", strconv.Itoa(int(time.Now().Month())))
		if err != nil {
			return err
		}

		slog.Debug("Spotify 艺术家月榜更新成功")
	}

	if int64(time.Now().Month())-h >= 6 || h == 0 {
		slog.Debug("正在更新 Spotify 艺术家半年榜")

		artists, err := c.getTopArtistsFromSpotify(spotify.MediumTermRange)
		if err != nil {
			return err
		}

		var res []string

		for _, artist := range artists {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", artist.ID)
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, artist.ID, artist.toMap())
				if err != nil {
					return err
				}
			}
			res = append(res, artist.ID)
		}

		j, err := json.Marshal(res)
		if err != nil {
			return err
		}

		err = dbc.SetMap("half-yearly-top-artists", strconv.Itoa(int(time.Now().Month())), string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "half-yearly-top-artists", strconv.Itoa(int(time.Now().Month())))
		if err != nil {
			return err
		}

		slog.Debug("Spotify 艺术家半年榜更新成功")
	}

	if int64(time.Now().Year())-y >= 1 || h == 0 {
		slog.Debug("正在更新 Spotify 艺术家年榜")

		artists, err := c.getTopArtistsFromSpotify(spotify.LongTermRange)
		if err != nil {
			return err
		}

		var res []string

		for _, artist := range artists {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", artist.ID)
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, artist.ID, artist.toMap())
				if err != nil {
					return err
				}
			}
			res = append(res, artist.ID)
		}

		j, err := json.Marshal(res)
		if err != nil {
			return err
		}

		err = dbc.SetMap("yearly-top-artists", strconv.Itoa(time.Now().Year()), string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "yearly-top-artists", strconv.Itoa(time.Now().Year()))
		if err != nil {
			return err
		}

		slog.Debug("Spotify 艺术家年榜更新成功")
	}

	return nil
}

func (c *Client) getTopTracksFromSpotify(dbc dbClient, r spotify.Range) ([]Track, error) {
	ftp, err := c.C.CurrentUsersTopTracks(c.Ctx, spotify.Limit(50), spotify.Timerange(r))
	if err != nil {
		return nil, err
	}

	var res []Track

	for _, track := range ftp.Tracks {
		convertedTrack, err := c.convertTrack(dbc, &track)
		if err != nil {
			return nil, err
		}

		res = append(res, *convertedTrack)
	}

	return res, nil
}

func (c *Client) saveTopTracks(dbc dbClient) error {
	m, err := dbc.GetMapInt64("updated-times", "monthly-top-tracks")
	if err != nil {
		return err
	}

	h, err := dbc.GetMapInt64("updated-times", "half-yearly-top-tracks")
	if err != nil {
		return err
	}

	y, err := dbc.GetMapInt64("updated-times", "yearly-top-tracks")
	if err != nil {
		return err
	}

	if int64(time.Now().Month()) > m || m == 0 {
		slog.Debug("正在更新 Spotify 曲目月榜")

		tracks, err := c.getTopTracksFromSpotify(dbc, spotify.ShortTermRange)
		if err != nil {
			return err
		}

		var res []string

		for _, track := range tracks {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", track.ID)
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, track.ID, track.toMap())
				if err != nil {
					return err
				}
			}
			res = append(res, track.ID)
		}

		j, err := json.Marshal(res)
		if err != nil {
			return err
		}

		err = dbc.SetMap("monthly-top-tracks", strconv.Itoa(int(time.Now().Month())), string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "monthly-top-tracks", strconv.Itoa(int(time.Now().Month())))
		if err != nil {
			return err
		}

		slog.Debug("Spotify 曲目月榜更新成功")
	}

	if int64(time.Now().Month())-h >= 6 || h == 0 {
		slog.Debug("正在更新 Spotify 曲目半年榜")

		tracks, err := c.getTopTracksFromSpotify(dbc, spotify.MediumTermRange)
		if err != nil {
			return err
		}

		var res []string

		for _, track := range tracks {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", track.ID)
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, track.ID, track.toMap())
				if err != nil {
					return err
				}
			}
			res = append(res, track.ID)
		}

		j, err := json.Marshal(res)
		if err != nil {
			return err
		}

		err = dbc.SetMap("half-yearly-top-tracks", strconv.Itoa(int(time.Now().Month())), string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "half-yearly-top-tracks", strconv.Itoa(int(time.Now().Month())))
		if err != nil {
			return err
		}

		slog.Debug("Spotify 曲目半年榜更新成功")
	}

	if int64(time.Now().Year())-y >= 1 || h == 0 {
		slog.Debug("正在更新 Spotify 曲目年榜")

		tracks, err := c.getTopTracksFromSpotify(dbc, spotify.LongTermRange)
		if err != nil {
			return err
		}

		var res []string

		for _, track := range tracks {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", track.ID)
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, track.ID, track.toMap())
				if err != nil {
					return err
				}
			}
			res = append(res, track.ID)
		}

		j, err := json.Marshal(res)
		if err != nil {
			return err
		}

		err = dbc.SetMap("yearly-top-tracks", strconv.Itoa(time.Now().Year()), string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "yearly-top-tracks", strconv.Itoa(time.Now().Year()))
		if err != nil {
			return err
		}

		slog.Debug("Spotify 曲目年榜更新成功")
	}

	return nil
}

type Tops struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// GetTopTracksIDs TODO: 算法需要增强
// GetTopTracksIDs 返回一段时间内的热门曲目ID(包括t1和t2), 若其中一个日期没有数据会返回nil, 若播放记录中的ID对应的信息不存在会跳过, limit为0则不限制
func (c *Client) GetTopTracksIDs(dbc dbClient, t1, t2 time.Time, limit int) ([]Tops, error) {
	rangeFromT1ToT2, err := c.GetPlayedRangeDuringATime(dbc, t1, t2)
	if err != nil {
		return nil, err
	}

	if rangeFromT1ToT2 == nil {
		return nil, nil
	}

	ph, err := c.GetPlayedHistory(dbc, int64(rangeFromT1ToT2.Start), int64(rangeFromT1ToT2.End))
	if err != nil {
		return nil, err
	}

	if ph == nil {
		return nil, nil
	}

	trackCount := make(map[string]int)

	for _, track := range ph {
		trackCount[track.ID]++
	}

	var tops []Tops

	for id, count := range trackCount {
		tops = append(tops, Tops{id, count})
	}

	sort.Slice(tops, func(i, j int) bool {
		return tops[i].Count > tops[j].Count
	})

	if len(tops) > limit {
		tops = tops[:limit]
	}

	return tops, nil
}

// GetTopArtistsIDs TODO: 算法需要增强
// GetTopArtistsIDs 返回一段时间内的热门艺术家ID(包括t1和t2), 若其中一个日期没有数据会返回nil, 若播放记录中的ID对应的信息不存在会跳过, limit为0则不限制
func (c *Client) GetTopArtistsIDs(dbc dbClient, t1, t2 time.Time, limit int) ([]Tops, error) {
	rangeFromT1ToT2, err := c.GetPlayedRangeDuringATime(dbc, t1, t2)
	if err != nil {
		return nil, err
	}

	if rangeFromT1ToT2 == nil {
		return nil, nil
	}

	pt, err := c.GetPlayedHistory(dbc, int64(rangeFromT1ToT2.Start), int64(rangeFromT1ToT2.End))
	if err != nil {
		return nil, err
	}

	if pt == nil {
		return nil, nil
	}

	artistCount := make(map[string]int)

	for _, track := range pt {
		for _, artist := range track.Artists {
			artistCount[artist.ID]++
		}
	}

	var tops []Tops

	for id, count := range artistCount {
		tops = append(tops, Tops{id, count})
	}

	sort.Slice(tops, func(i, j int) bool {
		return tops[i].Count > tops[j].Count
	})

	if len(tops) > limit {
		tops = tops[:limit]
	}

	return tops, nil
}

// GetTopAlbumsIDs TODO: 算法需要增强
// GetTopAlbumsIDs 返回一段时间内的热门专辑ID(包括t1和t2), 若其中一个日期没有数据会返回nil, 若播放记录中的ID对应的信息不存在会跳过, limit为0则不限制
func (c *Client) GetTopAlbumsIDs(dbc dbClient, t1, t2 time.Time, limit int) ([]Tops, error) {
	rangeFromT1ToT2, err := c.GetPlayedRangeDuringATime(dbc, t1, t2)
	if err != nil {
		return nil, err
	}

	if rangeFromT1ToT2 == nil {
		return nil, nil
	}

	ph, err := c.GetPlayedHistory(dbc, int64(rangeFromT1ToT2.Start), int64(rangeFromT1ToT2.End))
	if err != nil {
		return nil, err
	}

	if ph == nil {
		return nil, nil
	}

	albumCount := make(map[string]int)

	for _, track := range ph {
		albumCount[track.Album.ID]++
	}

	var tops []Tops

	for id, count := range albumCount {
		tops = append(tops, Tops{id, count})
	}

	sort.Slice(tops, func(i, j int) bool {
		return tops[i].Count > tops[j].Count
	})

	if len(tops) > limit {
		tops = tops[:limit]
	}

	return tops, nil
}
