package spotify

import (
	"encoding/json"
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
//func (c *Client) getTotalPlayedCountInAType(valkeyClient *valkey.Client, t1, t2 time.Time) (int64, error) {
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
