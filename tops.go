package spotify

import (
	"encoding/json"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/zmb3/spotify/v2"
)

// TODO: DailyTrack, Album

func (c *Client) saveTopArtists(dbc dbClient) error {
	m, err := dbc.GetMapStr("updated-times", "monthly-top-artists")
	if err != nil {
		return err
	}

	h, err := dbc.GetMapStr("updated-times", "half-yearly-top-artists")
	if err != nil {
		return err
	}

	y, err := dbc.GetMapStr("updated-times", "yearly-top-artists")
	if err != nil {
		return err
	}

	t := time.Time{}

	if m != "" {
		t, err = time.Parse(time.DateOnly, m)
		if err != nil {
			return err
		}
	}

	tn := time.Now()
	tns := tn.Format(time.DateOnly)

	if int(tn.Month()) > int(t.Month()) || tn.Year() > t.Year() || m == "" {
		slog.Debug("正在更新 Spotify 艺术家月榜")

		fap, err := c.C.CurrentUsersTopArtists(c.Ctx, spotify.Limit(50), spotify.Timerange(spotify.ShortTermRange))
		if err != nil {
			return err
		}

		var artists []string

		for _, artist := range fap.Artists {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", artist.ID.String())
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, artist.ID.String(), c.convertArtist(&artist).toMap())
				if err != nil {
					return err
				}
			}
			artists = append(artists, artist.ID.String())
		}

		j, err := json.Marshal(artists)
		if err != nil {
			return err
		}

		err = dbc.SetMap("monthly-top-artists", tns, string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "monthly-top-artists", tns)
		if err != nil {
			return err
		}

		slog.Debug("Spotify 艺术家月榜更新成功")
	}

	if h != "" {
		t, err = time.Parse(time.DateOnly, h)
		if err != nil {
			return err
		}
	}

	if int(tn.Month()) > int(t.Month()) || tn.Year() > t.Year() || h == "" {
		slog.Debug("正在更新 Spotify 艺术家半年榜")

		fap, err := c.C.CurrentUsersTopArtists(c.Ctx, spotify.Limit(50), spotify.Timerange(spotify.MediumTermRange))
		if err != nil {
			return err
		}

		var artists []string

		for _, artist := range fap.Artists {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", artist.ID.String())
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, artist.ID.String(), c.convertArtist(&artist).toMap())
				if err != nil {
					return err
				}
			}
			artists = append(artists, artist.ID.String())
		}

		j, err := json.Marshal(artists)
		if err != nil {
			return err
		}

		err = dbc.SetMap("half-yearly-top-artists", tns, string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "half-yearly-top-artists", tns)
		if err != nil {
			return err
		}

		slog.Debug("Spotify 艺术家半年榜更新成功")
	}

	if y != "" {
		t, err = time.Parse(time.DateOnly, y)
		if err != nil {
			return err
		}
	}

	if int(tn.Month()) > int(t.Month()) || tn.Year() > t.Year() || y == "" {
		slog.Debug("正在更新 Spotify 艺术家年榜")

		fap, err := c.C.CurrentUsersTopArtists(c.Ctx, spotify.Limit(50), spotify.Timerange(spotify.LongTermRange))
		if err != nil {
			return err
		}

		var artists []string

		for _, artist := range fap.Artists {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", artist.ID.String())
			if err != nil {
				return err
			}

			if !exists {
				err = saveID(dbc, artist.ID.String(), c.convertArtist(&artist).toMap())
				if err != nil {
					return err
				}
			}
			artists = append(artists, artist.ID.String())
		}

		j, err := json.Marshal(artists)
		if err != nil {
			return err
		}

		err = dbc.SetMap("yearly-top-artists", tns, string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "yearly-top-artists", tns)
		if err != nil {
			return err
		}

		slog.Debug("Spotify 艺术家年榜更新成功")
	}

	return nil
}

func (c *Client) correctTrack(dbc dbClient, track spotify.FullTrack, timeNow time.Time, lookbackDuration time.Time) (*Track, error) {
	searchResults, err := c.C.Search(c.Ctx, "artist:"+track.Artists[0].Name+" track:"+track.Name, spotify.SearchTypeTrack)
	if err != nil {
		return nil, err
	}

	var correctSearchResults []spotify.FullTrack

	for _, searchResult := range searchResults.Tracks.Tracks {
		if strings.HasPrefix(track.Name, searchResult.Name) && searchResult.Artists[0].ID.String() == track.Artists[0].ID.String() || strings.HasPrefix(searchResult.Name, track.Name) && searchResult.Artists[0].ID.String() == track.Artists[0].ID.String() {
			correctSearchResults = append(correctSearchResults, searchResult)
		}
	}

	tops, err := c.GetTopTracksIDs(dbc, lookbackDuration, timeNow, 50)
	if err != nil {
		return nil, err
	}

	for _, top := range tops {
		for _, correctSearchResult := range correctSearchResults {
			if top.ID == correctSearchResult.ID.String() {
				return c.getTrackCache(dbc, top.ID)
			}
		}
	}

	// 按照是否点赞补救
	for _, v := range correctSearchResults {
		isSaved, err := c.C.UserHasTracks(c.Ctx, v.ID)
		if err != nil {
			return nil, err
		}

		if isSaved[0] {
			return c.convertTrack(dbc, &v)
		}
	}

	return nil, nil
}

func (c *Client) saveTopTracks(dbc dbClient) error {
	m, err := dbc.GetMapStr("updated-times", "monthly-top-tracks")
	if err != nil {
		return err
	}

	h, err := dbc.GetMapStr("updated-times", "half-yearly-top-tracks")
	if err != nil {
		return err
	}

	y, err := dbc.GetMapStr("updated-times", "yearly-top-tracks")
	if err != nil {
		return err
	}

	t := time.Time{}

	tn := time.Now()
	tns := tn.Format(time.DateOnly)

	if m != "" {
		t, err = time.Parse(time.DateOnly, m)
		if err != nil {
			return err
		}
	}

	if int(tn.Month()) > int(t.Month()) || tn.Year() > t.Year() || m == "" {
		slog.Debug("正在更新 Spotify 曲目月榜")

		ftp, err := c.C.CurrentUsersTopTracks(c.Ctx, spotify.Limit(50), spotify.Timerange(spotify.ShortTermRange))
		if err != nil {
			return err
		}

		var tracks []string

		for _, track := range ftp.Tracks {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", track.ID.String())
			if err != nil {
				return err
			}

			if !exists {
				converted, err := c.convertTrack(dbc, &track)
				if err != nil {
					return err
				}

				err = saveID(dbc, track.ID.String(), converted.toMap())
				if err != nil {
					return err
				}
			}

			// Spotify 的一个 Bug: 有的曲目会被重置版混音版的同名曲目顶替, 排行榜上统计会混淆
			saved, err := c.C.UserHasTracks(c.Ctx, track.ID)
			if err != nil {
				return err
			}

			if !saved[0] {
				slog.Info("此歌曲未点赞, 可能是 Spotify 的 Bug, 进行补救", "名称", track.Name, "ID", track.ID, "艺术家", track.Artists[0].Name)

				ct, err := c.correctTrack(dbc, track, tn, tn.AddDate(0, -1, 0))
				if err != nil {
					return err
				}

				if ct != nil {
					slog.Info("补救成功, 替换为", "名称", ct.Name, "ID", ct.ID, "艺术家", ct.Artists[0].Name)
					tracks = append(tracks, ct.ID)
				} else {
					slog.Info("补救失败, 未替换")
				}

			} else {
				tracks = append(tracks, track.ID.String())
			}
		}

		j, err := json.Marshal(tracks)
		if err != nil {
			return err
		}

		err = dbc.SetMap("monthly-top-tracks", tns, string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "monthly-top-tracks", tns)
		if err != nil {
			return err
		}

		slog.Debug("Spotify 曲目月榜更新成功")
	}

	if h != "" {
		t, err = time.Parse(time.DateOnly, h)
		if err != nil {
			return err
		}
	}

	if int(tn.Month()) > int(t.Month()) || tn.Year() > t.Year() || h == "" {
		slog.Debug("正在更新 Spotify 曲目半年榜")

		ftp, err := c.C.CurrentUsersTopTracks(c.Ctx, spotify.Limit(50), spotify.Timerange(spotify.MediumTermRange))
		if err != nil {
			return err
		}

		var tracks []string

		for _, track := range ftp.Tracks {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", track.ID.String())
			if err != nil {
				return err
			}

			if !exists {
				converted, err := c.convertTrack(dbc, &track)
				if err != nil {
					return err
				}

				err = saveID(dbc, track.ID.String(), converted.toMap())
				if err != nil {
					return err
				}
			}

			saved, err := c.C.UserHasTracks(c.Ctx, track.ID)
			if err != nil {
				return err
			}

			if !saved[0] {
				slog.Info("此歌曲未点赞, 可能是 Spotify 的 Bug, 进行补救", "名称", track.Name, "ID", track.ID, "艺术家", track.Artists[0].Name)

				ct, err := c.correctTrack(dbc, track, tn, tn.AddDate(0, -6, 0))
				if err != nil {
					return err
				}

				if ct != nil {
					slog.Info("补救成功, 替换为", "名称", ct.Name, "ID", ct.ID, "艺术家", ct.Artists[0].Name)
					tracks = append(tracks, ct.ID)
				} else {
					slog.Info("补救失败, 未替换")
				}

			} else {
				tracks = append(tracks, track.ID.String())
			}

		}

		j, err := json.Marshal(tracks)
		if err != nil {
			return err
		}

		err = dbc.SetMap("half-yearly-top-tracks", tns, string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "half-yearly-top-tracks", tns)
		if err != nil {
			return err
		}

		slog.Debug("Spotify 曲目半年榜更新成功")
	}

	if y != "" {
		t, err = time.Parse(time.DateOnly, y)
		if err != nil {
			return err
		}
	}

	if int(tn.Month()) > int(t.Month()) || tn.Year() > t.Year() || y == "" {
		slog.Debug("正在更新 Spotify 曲目年榜")

		ftp, err := c.C.CurrentUsersTopTracks(c.Ctx, spotify.Limit(50), spotify.Timerange(spotify.LongTermRange))
		if err != nil {
			return err
		}

		var tracks []string

		for _, track := range ftp.Tracks {
			exists, err := dbc.CheckIfMapFieldExists("spotify-ids", track.ID.String())
			if err != nil {
				return err
			}

			if !exists {
				converted, err := c.convertTrack(dbc, &track)
				if err != nil {
					return err
				}

				err = saveID(dbc, track.ID.String(), converted.toMap())
				if err != nil {
					return err
				}
			}

			saved, err := c.C.UserHasTracks(c.Ctx, track.ID)
			if err != nil {
				return err
			}

			if !saved[0] {
				slog.Info("此歌曲未点赞, 可能是 Spotify 的 Bug, 进行补救", "名称", track.Name, "ID", track.ID, "艺术家", track.Artists[0].Name)

				ct, err := c.correctTrack(dbc, track, tn, tn.AddDate(-1, 0, 0))
				if err != nil {
					return err
				}

				if ct != nil {
					slog.Info("补救成功, 替换为", "名称", ct.Name, "ID", ct.ID, "艺术家", ct.Artists[0].Name)
					tracks = append(tracks, ct.ID)
				} else {
					slog.Info("补救失败, 未替换")
				}

			} else {
				tracks = append(tracks, track.ID.String())
			}

		}

		j, err := json.Marshal(tracks)
		if err != nil {
			return err
		}

		err = dbc.SetMap("yearly-top-tracks", tns, string(j))
		if err != nil {
			return err
		}

		err = dbc.SetMap("updated-times", "yearly-top-tracks", tns)
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
	rangeFromT1ToT2, err := c.GetPlaybackRangeDuringATime(dbc, t1, t2)
	if err != nil {
		return nil, err
	}

	if rangeFromT1ToT2 == nil {
		return nil, nil
	}

	ph, err := c.GetPlaybackHistory(dbc, int64(rangeFromT1ToT2.Start), int64(rangeFromT1ToT2.End))
	if err != nil {
		return nil, err
	}

	if ph == nil {
		return nil, nil
	}

	trackCounts := map[string]int{}

	for _, track := range ph {
		trackCounts[track.ID]++
	}

	var tops []Tops

	for id, count := range trackCounts {
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
	rangeFromT1ToT2, err := c.GetPlaybackRangeDuringATime(dbc, t1, t2)
	if err != nil {
		return nil, err
	}

	if rangeFromT1ToT2 == nil {
		return nil, nil
	}

	ph, err := c.GetPlaybackHistory(dbc, int64(rangeFromT1ToT2.Start), int64(rangeFromT1ToT2.End))
	if err != nil {
		return nil, err
	}

	if ph == nil {
		return nil, nil
	}

	artistCounts := map[string]int{}

	for _, track := range ph {
		for _, artist := range track.Artists {
			artistCounts[artist.ID]++
		}
	}

	var tops []Tops

	for id, count := range artistCounts {
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
	rangeFromT1ToT2, err := c.GetPlaybackRangeDuringATime(dbc, t1, t2)
	if err != nil {
		return nil, err
	}

	if rangeFromT1ToT2 == nil {
		return nil, nil
	}

	ph, err := c.GetPlaybackHistory(dbc, int64(rangeFromT1ToT2.Start), int64(rangeFromT1ToT2.End))
	if err != nil {
		return nil, err
	}

	if ph == nil {
		return nil, nil
	}

	albumCounts := map[string]int{}

	for _, track := range ph {
		albumCounts[track.Album.ID]++
	}

	var tops []Tops

	for id, count := range albumCounts {
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
