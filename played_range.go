package spotify

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"time"
)

type PlaybackRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (c *Client) GetTotalPlaybackHistoryCount(dbc dbClient) (int64, error) {
	return dbc.GetSliceLen("playback-history")
}

// getTime 返回 PlaybackHistory 的日期部分
func getTime(layout, timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, nil
	}

	if layout == time.DateTime {
		return time.Parse(layout, timeStr[44:63])
	}
	return time.Parse(layout, timeStr[44:54])
}

// getTotalPlayedCountInAType 获取一段时间内一个类型的收听量, 若其中一个日期没有数据会返回 nil
//func (c *Client) getTotalPlayedCountInAType(dbc dbClient, t1, t2 time.Time) (int64, error) {
//
//}

// savePlaybackRangeOnADay 存储指定日期的收听量
func (c *Client) savePlaybackRangeOnADay(dbc dbClient, date time.Time, count int) error {
	dateDateOnly := date.Format(time.DateOnly)

	rangeToday, err := c.GetPlaybackRangeOnADay(dbc, date)
	if err != nil {
		return err
	}

	rangeYesterday, err := c.GetPlaybackRangeOnADay(dbc, date.AddDate(0, 0, -1))
	if err != nil {
		return err
	}

	// 今天第一次统计
	if rangeToday == nil {
		rangeToday = &PlaybackRange{End: count}
		if rangeYesterday != nil {
			rangeToday.Start = rangeYesterday.End + 1
			rangeToday.End += rangeYesterday.End
		} else {
			rangeToday.End--
		}
	} else {
		rangeToday.End += count
	}

	slog.Debug("每日播放量保存成功", "日期", dateDateOnly, "新增", count, "总共", rangeToday.End-rangeToday.Start)

	j, err := json.Marshal(rangeToday)
	if err != nil {
		return err
	}

	err = dbc.SetMap("daily-playback-ranges", dateDateOnly, string(j))
	if err != nil {
		return err
	}

	rangeTodayStart, err := dbc.GetSliceByIndex("playback-history", int64(rangeToday.Start))
	if err != nil {
		return err
	}

	rangeTodayEnd, err := dbc.GetSliceByIndex("playback-history", int64(rangeToday.End))
	if err != nil {
		return err
	}

	rangeTodayStartMinusOne, err := dbc.GetSliceByIndex("playback-history", int64(rangeToday.Start-1))
	if err != nil {
		return err
	}

	rangeTodayEndPlusOne, err := dbc.GetSliceByIndex("playback-history", int64(rangeToday.End+1))
	if err != nil {
		return err
	}

	lastPlayed, err := dbc.GetSliceByIndex("playback-history", -1)
	if err != nil {
		return err
	}

	rangeTodayStartTimeStr := rangeTodayStart[44:54]
	rangeTodayEndTimeStr := rangeTodayEnd[44:54]
	rangeTodayStartMinusOneTimeStr := rangeTodayStartMinusOne[44:54]
	lastPlayedTimeStr := lastPlayed[44:54]

	if rangeTodayStartTimeStr != dateDateOnly || rangeTodayEndTimeStr != dateDateOnly || rangeTodayStartMinusOne != "" && rangeTodayStartMinusOneTimeStr == dateDateOnly && rangeToday.Start > 0 || dateDateOnly == lastPlayedTimeStr && rangeTodayEndPlusOne != "" {
		slog.Warn("每日播放量统计不匹配, 可能是运行时退出导致, 正在修复")
		defer slog.Info("每日播放量统计修复完成")
		err = dbc.Delete("daily-playback-ranges")
		if err != nil {
			return err
		}

		playbackRanges := map[time.Time]*PlaybackRange{}

		for i := 0; ; i += 50 {
			playbackHistory, err := dbc.GetSlice("playback-history", int64(i), int64(i+49))
			if err != nil {
				return err
			}

			if len(playbackHistory) == 0 {
				break
			}

			for _, playback := range playbackHistory {
				playbackTime, err := getTime(time.DateOnly, playback)
				if err != nil {
					return err
				}

				// 第一次统计这个日期
				if playbackRanges[playbackTime] == nil {
					playbackRanges[playbackTime] = &PlaybackRange{}
					if playbackRanges[playbackTime.AddDate(0, 0, -1)] != nil {
						playbackRanges[playbackTime].Start = playbackRanges[playbackTime.AddDate(0, 0, -1)].End + 1
						playbackRanges[playbackTime].End = playbackRanges[playbackTime].Start
					}
				} else {
					playbackRanges[playbackTime].End++
				}
			}

			// 到尾了
			if len(playbackHistory) != 50 {
				break
			}
		}

		for day, pr := range playbackRanges {
			j, err = json.Marshal(pr)
			if err != nil {
				return err
			}

			err = dbc.SetMap("daily-playback-ranges", day.Format(time.DateOnly), string(j))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetPlaybackRangeOnADay 获取指定日期的收听量, 若不存在会返回 nil
func (c *Client) GetPlaybackRangeOnADay(dbc dbClient, date time.Time) (*PlaybackRange, error) {
	r, err := dbc.GetMapStr("daily-playback-ranges", date.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}

	if r == "" {
		return nil, nil
	}

	rangeOnADay := &PlaybackRange{}

	err = json.Unmarshal([]byte(r), rangeOnADay)
	if err != nil {
		return nil, err
	}

	return rangeOnADay, nil
}

// GetPlaybackRangeDuringATime 获取一段时间内的收听量, 若其中一个日期没有数据会返回 nil
func (c *Client) GetPlaybackRangeDuringATime(dbc dbClient, t1, t2 time.Time) (*PlaybackRange, error) {
	start, err := c.GetPlaybackRangeOnADay(dbc, t1)
	if err != nil {
		return nil, err
	}

	if start == nil {
		return nil, nil
	}

	end, err := c.GetPlaybackRangeOnADay(dbc, t2)
	if err != nil {
		return nil, err
	}

	if end == nil {
		return nil, nil
	}

	return &PlaybackRange{start.Start, end.End}, nil
}

// savePlaybackCount 存储歌曲和专辑和艺术家的收听量
func (c *Client) savePlaybackCounts(dbc dbClient, tracks []PlayedTrack) error {
	for _, track := range tracks {
		counts, err := dbc.GetMapInt64("track-playback-counts", track.ID)
		if err != nil {
			return err
		}

		err = dbc.SetMap("track-playback-counts", track.ID, strconv.Itoa(int(counts+1)))
		if err != nil {
			return err
		}

		counts, err = dbc.GetMapInt64("album-playback-counts", track.Album.ID)
		if err != nil {
			return err
		}

		err = dbc.SetMap("album-playback-counts", track.Album.ID, strconv.Itoa(int(counts+1)))
		if err != nil {
			return err
		}

		for _, artist := range track.Artists {
			counts, err = dbc.GetMapInt64("artist-playback-counts", artist.ID)
			if err != nil {
				return err
			}

			err = dbc.SetMap("artist-playback-counts", artist.ID, strconv.Itoa(int(counts+1)))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// saveHourlyPlaybackCounts TODO: 算法需要增强
// saveHourlyPlaybackCounts 存储每小时的收听量
func (c *Client) saveHourlyPlaybackCounts(dbc dbClient) error {
	lastSavedPlaybackTimeStr, err := dbc.GetMapStr("hourly-playback-counts", "last-saved-playback-time")
	if err != nil {
		return err
	}

	lastSavedPlaybackTime := time.Time{}
	if lastSavedPlaybackTimeStr != "" {
		lastSavedPlaybackTime, err = time.Parse(time.DateTime, lastSavedPlaybackTimeStr)
		if err != nil {
			return err
		}
	}

	var playbackHistory []string
	playbackTime := time.Time{}

	// 倒数每 50 组中第一次的播放记录中的时间在上次保存的时间之后
	for i := -50; playbackTime.After(lastSavedPlaybackTime.Add(-time.Second)) || i == -50 || lastSavedPlaybackTime.IsZero(); i -= 50 {
		morePlaybackHistory, err := dbc.GetSlice("playback-history", int64(i), int64(i+49))
		if err != nil {
			return err
		}

		if len(morePlaybackHistory) == 0 {
			break
		}

		playbackHistory = append(morePlaybackHistory, playbackHistory...)

		playbackTime, err = getTime(time.DateTime, playbackHistory[0])
		if err != nil {
			return err
		}

		// 到头了
		if len(morePlaybackHistory) != 50 {
			break
		}
	}

	counts := map[int]int{}
	lastPlaybackTime := ""

	// 最后会用来当作此次最后保存的时间
	for _, playback := range playbackHistory {
		playbackTime, err = getTime(time.DateTime, playback)
		if err != nil {
			return err
		}

		if playbackTime.After(lastSavedPlaybackTime.Add(time.Second)) {
			counts[playbackTime.Hour()]++
			lastPlaybackTime = playbackTime.Format(time.DateTime)
		}
	}

	if lastPlaybackTime != "" {
		err = dbc.SetMap("hourly-playback-counts", "last-saved-playback-time", lastPlaybackTime)
		if err != nil {
			return err
		}
	}

	for hour, count := range counts {
		count2, err := dbc.GetMapInt64("hourly-playback-counts", strconv.Itoa(hour))
		if err != nil {
			return err
		}

		total := count + int(count2)
		err = dbc.SetMap("hourly-playback-counts", strconv.Itoa(hour), strconv.Itoa(total))
		if err != nil {
			return err
		}

		if count > 0 {
			slog.Debug("每小时收听量保存成功", "小时", hour, "新增", count, "总共", total)
		}
	}

	allCounts, err := dbc.GetMapAll("hourly-playback-counts")
	if err != nil {
		return err
	}

	totalCount := 0
	for _, count := range allCounts {
		if i, err := strconv.Atoi(count); err == nil {
			totalCount += i
		}
	}

	t, err := c.GetTotalPlaybackHistoryCount(dbc)
	if err != nil {
		return err
	}

	if totalCount != int(t) {
		slog.Warn("每小时播放量统计错误, 可能是运行时退出导致, 正在修复")
		defer slog.Info("每小时播放量统计修复完成")
		err = dbc.Delete("hourly-playback-counts")
		if err != nil {
			return err
		}
		return c.saveHourlyPlaybackCounts(dbc)
	}

	return nil
}

// GetHourlyPlayBackCounts 返回每个小时的收听量
func (c *Client) GetHourlyPlayBackCounts(dbc dbClient) (map[int]int, error) {
	res := map[int]int{}

	for t := 0; t <= 24; t++ {
		count, err := dbc.GetMapInt64("hourly-playback-counts", strconv.Itoa(t))
		if err != nil {
			return nil, err
		}

		res[t] = int(count)
	}

	return res, nil
}
