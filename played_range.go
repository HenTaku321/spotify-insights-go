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

// getTotalPlayedCountInAType 获取一段时间内一个类型的收听量, 若其中一个日期没有数据会返回 nil
//func (c *Client) getTotalPlayedCountInAType(dbc dbClient, t1, t2 time.Time) (int64, error) {
//
//}

// savePlayedRangeOnADay 存储指定日期的收听量
func (c *Client) savePlayedRangeOnADay(dbc dbClient, date time.Time, count int) error {
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
		}
	} else {
		rangeToday.End += count
	}

	j, err := json.Marshal(rangeToday)
	if err != nil {
		return err
	}

	return dbc.SetMap("daily-played-range", date.Format(time.DateOnly), string(j))
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

	var lastSavedPlayedTime time.Time
	if lastSavedPlayedTimeStr != "" {
		lastSavedPlayedTime, err = time.Parse(time.DateTime, lastSavedPlayedTimeStr)
		if err != nil {
			return err
		}
	}

	counts := map[int]int{}

	playedTracks, err := dbc.GetSlice("played-history", -50, -1)
	if err != nil {
		return err
	}

	// 最后会用来当作此次最后保存的时间
	playedTimeStr := ""
	for _, item := range playedTracks {
		// 年月日部分
		playedTimeStr = item[len(item)-21 : len(item)-2]

		playedTime, err := time.Parse(time.DateTime, playedTimeStr)
		if err != nil {
			return err
		}

		if playedTime.After(lastSavedPlayedTime.Add(time.Second)) {
			counts[playedTime.Hour()]++
		}
	}

	err = dbc.SetMap("hourly-played-count", "last-saved-played-time", playedTimeStr)
	if err != nil {
		return err
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
