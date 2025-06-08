package spotify

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"time"
)

type PlayedRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (c *Client) GetTotalPlayedCount(dbc dbClient) (int64, error) {
	return dbc.GetSliceLen("played-history")
}

// getDateTime 返回 PlayedHistory 的日期部分
func getDateTime(layout, timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, nil
	}

	if layout == time.DateTime {
		return time.Parse(layout, timeStr[len(timeStr)-21:len(timeStr)-2])
	}
	return time.Parse(layout, timeStr[len(timeStr)-21:len(timeStr)-11])
}

// getTotalPlayedCountInAType 获取一段时间内一个类型的收听量, 若其中一个日期没有数据会返回 nil
//func (c *Client) getTotalPlayedCountInAType(dbc dbClient, t1, t2 time.Time) (int64, error) {
//
//}

// savePlayedRangeOnADay 存储指定日期的收听量
func (c *Client) savePlayedRangeOnADay(dbc dbClient, date time.Time, count int) error {
	dateDateOnly := date.Format(time.DateOnly)

	rangeToday, err := c.GetPlayedRangeOnADay(dbc, date)
	if err != nil {
		return err
	}

	rangeYesterday, err := c.GetPlayedRangeOnADay(dbc, date.AddDate(0, 0, -1))
	if err != nil {
		return err
	}

	// 今天第一次统计
	if rangeToday == nil {
		rangeToday = &PlayedRange{End: count}
		if rangeYesterday != nil {
			rangeToday.Start = rangeYesterday.End + 1
			rangeToday.End += rangeYesterday.End
		} else {
			rangeToday.End--
		}
	} else {
		rangeToday.End += count
	}

	j, err := json.Marshal(rangeToday)
	if err != nil {
		return err
	}

	err = dbc.SetMap("daily-played-range", dateDateOnly, string(j))
	if err != nil {
		return err
	}

	rangeTodayStart, err := dbc.GetSliceByIndex("played-history", int64(rangeToday.Start))
	if err != nil {
		return err
	}

	rangeTodayEnd, err := dbc.GetSliceByIndex("played-history", int64(rangeToday.End))
	if err != nil {
		return err
	}

	rangeTodayStartMinusOne, err := dbc.GetSliceByIndex("played-history", int64(rangeToday.Start-1))
	if err != nil {
		return err
	}

	rangeTodayEndPlusOne, err := dbc.GetSliceByIndex("played-history", int64(rangeToday.End+1))
	if err != nil {
		return err
	}

	lastPlayed, err := dbc.GetSliceByIndex("played-history", -1)
	if err != nil {
		return err
	}

	rangeTodayStartTime, err := getDateTime(time.DateOnly, rangeTodayStart)
	if err != nil {
		return err
	}

	rangeTodayEndTime, err := getDateTime(time.DateOnly, rangeTodayEnd)
	if err != nil {
		return err
	}

	rangeTodayStartMinusOneTime, err := getDateTime(time.DateOnly, rangeTodayStartMinusOne)
	if err != nil {
		return err
	}

	lastPlayedTime, err := getDateTime(time.DateOnly, lastPlayed)
	if err != nil {
		return err
	}

	if rangeTodayStartTime.Format(time.DateOnly) != dateDateOnly || rangeTodayEndTime.Format(time.DateOnly) != dateDateOnly || rangeTodayStartMinusOne != "" && rangeTodayStartMinusOneTime.Format(time.DateOnly) == dateDateOnly && rangeToday.Start > 0 || dateDateOnly == lastPlayedTime.Format(time.DateOnly) && rangeTodayEndPlusOne != "" {
		slog.Warn("每日播放量统计不匹配, 可能是运行时退出导致, 正在修复")
		defer slog.Info("每日播放量统计修复完成")
		err = dbc.Delete("daily-played-range")
		if err != nil {
			return err
		}

		playedRanges := map[time.Time]*PlayedRange{}

		for i := 0; ; i += 50 {
			playedHistory, err := dbc.GetSlice("played-history", int64(i), int64(i+49))
			if err != nil {
				return err
			}

			if len(playedHistory) == 0 {
				break
			}

			for _, item := range playedHistory {
				playedTime, err := getDateTime(time.DateOnly, item)
				if err != nil {
					return err
				}

				// 第一次统计这个日期
				if playedRanges[playedTime] == nil {
					playedRanges[playedTime] = &PlayedRange{}
					if playedRanges[playedTime.AddDate(0, 0, -1)] != nil {
						playedRanges[playedTime].Start = playedRanges[playedTime.AddDate(0, 0, -1)].End + 1
						playedRanges[playedTime].End = playedRanges[playedTime].Start
					}
				} else {
					playedRanges[playedTime].End++
				}
			}

			// 到头了
			if len(playedHistory) != 50 {
				break
			}
		}

		for day, pr := range playedRanges {
			j, err = json.Marshal(pr)
			if err != nil {
				return err
			}

			err = dbc.SetMap("daily-played-range", day.Format(time.DateOnly), string(j))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetPlayedRangeOnADay 获取指定日期的收听量, 若不存在会返回 nil
func (c *Client) GetPlayedRangeOnADay(dbc dbClient, date time.Time) (*PlayedRange, error) {
	r, err := dbc.GetMapStr("daily-played-range", date.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}

	if r == "" {
		return nil, nil
	}

	rangeOnADay := &PlayedRange{}

	err = json.Unmarshal([]byte(r), rangeOnADay)
	if err != nil {
		return nil, err
	}

	return rangeOnADay, nil
}

// GetPlayedRangeDuringATime 获取一段时间内的收听量, 若其中一个日期没有数据会返回 nil
func (c *Client) GetPlayedRangeDuringATime(dbc dbClient, t1, t2 time.Time) (*PlayedRange, error) {
	start, err := c.GetPlayedRangeOnADay(dbc, t1)
	if err != nil {
		return nil, err
	}

	if start == nil {
		return nil, nil
	}

	end, err := c.GetPlayedRangeOnADay(dbc, t2)
	if err != nil {
		return nil, err
	}

	if end == nil {
		return nil, nil
	}

	return &PlayedRange{start.Start, end.End}, nil
}

// savePlayedCount 存储歌曲和专辑和艺术家的收听量
func (c *Client) savePlayedCount(dbc dbClient, tracks []PlayedTrack) error {
	for _, track := range tracks {
		count, err := dbc.GetMapInt64("tracks-played-count", track.ID)
		if err != nil {
			return err
		}

		err = dbc.SetMap("tracks-played-count", track.ID, strconv.Itoa(int(count+1)))
		if err != nil {
			return err
		}

		count, err = dbc.GetMapInt64("albums-played-count", track.Album.ID)
		if err != nil {
			return err
		}

		err = dbc.SetMap("albums-played-count", track.Album.ID, strconv.Itoa(int(count+1)))
		if err != nil {
			return err
		}

		for _, artist := range track.Artists {
			count, err = dbc.GetMapInt64("artists-played-count", artist.ID)
			if err != nil {
				return err
			}

			err = dbc.SetMap("artists-played-count", artist.ID, strconv.Itoa(int(count+1)))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// saveHourlyPlayedCount TODO: 算法需要增强
// saveHourlyPlayedCount 存储每小时的收听量
func (c *Client) saveHourlyPlayedCount(dbc dbClient) error {
	lastSavedPlayedTimeStr, err := dbc.GetMapStr("hourly-played-count", "last-saved-played-time")
	if err != nil {
		return err
	}

	lastSavedPlayedTime := time.Time{}
	if lastSavedPlayedTimeStr != "" {
		lastSavedPlayedTime, err = time.Parse(time.DateTime, lastSavedPlayedTimeStr)
		if err != nil {
			return err
		}
	}

	var playedHistory []string
	playedTime := time.Time{}

	// 倒数每 50 组中第一次的播放记录中的时间在上次保存的时间之后
	for i := -50; playedTime.After(lastSavedPlayedTime.Add(-time.Second)) || i == -50 || lastSavedPlayedTime.IsZero(); i -= 50 {
		morePlayedHistory, err := dbc.GetSlice("played-history", int64(i), int64(i+49))
		if err != nil {
			return err
		}

		if len(morePlayedHistory) == 0 {
			break
		}

		playedHistory = append(morePlayedHistory, playedHistory...)

		playedTime, err = getDateTime(time.DateTime, playedHistory[0])
		if err != nil {
			return err
		}

		// 到头了
		if len(morePlayedHistory) != 50 {
			break
		}
	}

	counts := map[int]int{}
	lastPlayedTime := ""

	// 最后会用来当作此次最后保存的时间
	for _, item := range playedHistory {
		playedTime, err = getDateTime(time.DateTime, item)
		if err != nil {
			return err
		}

		if playedTime.After(lastSavedPlayedTime.Add(time.Second)) {
			counts[playedTime.Hour()]++
			lastPlayedTime = playedTime.Format(time.DateTime)
		}
	}

	if lastPlayedTime != "" {
		err = dbc.SetMap("hourly-played-count", "last-saved-played-time", lastPlayedTime)
		if err != nil {
			return err
		}
	}

	for hour, count := range counts {
		count2, err := dbc.GetMapInt64("hourly-played-count", strconv.Itoa(hour))
		if err != nil {
			return err
		}

		total := count + int(count2)
		err = dbc.SetMap("hourly-played-count", strconv.Itoa(hour), strconv.Itoa(total))
		if err != nil {
			return err
		}

		if count > 0 {
			slog.Debug("每小时收听量保存成功", "小时", hour, "数量", count, "总共", total)
		}
	}

	all, err := dbc.GetMapAll("hourly-played-count")
	if err != nil {
		return err
	}

	totalCount := 0
	for _, count := range all {
		if i, err := strconv.Atoi(count); err == nil {
			totalCount += i
		}
	}

	t, err := c.GetTotalPlayedCount(dbc)
	if err != nil {
		return err
	}

	if totalCount != int(t) {
		slog.Warn("每小时播放量统计错误, 可能是运行时退出导致, 正在修复")
		defer slog.Info("每小时播放量统计修复完成")
		err = dbc.Delete("hourly-played-count")
		if err != nil {
			return err
		}
		return c.saveHourlyPlayedCount(dbc)
	}

	return nil
}

// GetHourlyPlayedCount 返回每个小时的收听量
func (c *Client) GetHourlyPlayedCount(dbc dbClient) (map[int]int, error) {
	res := map[int]int{}

	for t := 0; t <= 24; t++ {
		count, err := dbc.GetMapInt64("hourly-played-count", strconv.Itoa(t))
		if err != nil {
			return nil, err
		}

		res[t] = int(count)
	}

	return res, nil
}
