package spotify

import (
	"encoding/json"
	"github.com/zmb3/spotify/v2"
	"log/slog"
	"time"
)

type dbClient interface {
	SetString(key string, value string, ex *time.Duration) error
	GetString(key string) (string, error)
	SetMap(key, field, value string) error
	GetMapStr(key, field string) (string, error)
	GetMapInt64(key, field string) (int64, error)
	CheckIfMapFieldExists(key, field string) (bool, error)
	SetSlice(key string, value []string) error
	GetSlice(key string, start, stop int64) ([]string, error)
	GetSliceByIndex(key string, index int64) (string, error)
	GetSliceLen(key string) (int64, error)
}

// ArtistMap 是存到数据库列表 spotify-ids 中的存储格式, 与 Artist 相比移除了 ID 字段
type ArtistMap struct {
	Name       string          `json:"name"`
	Popularity int             `json:"popularity"`
	Genres     []string        `json:"genres"`
	Followers  int             `json:"followers"`
	Images     []spotify.Image `json:"images"`
}

// TrackMap 是存到数据库列表 spotify-ids 中的存储格式, 与 Track 相比移除了 ID 字段, 且替换 Artists 字段为 []string, 即 ID
type TrackMap struct {
	AlbumID    string   `json:"album_id"`
	ArtistsIDs []string `json:"artists_ids"`
	Duration   string   `json:"duration"`
	Name       string   `json:"name"`
	Popularity int      `json:"popularity"`
}

// AlbumMap 是存到数据库列表 spotify-ids 中的存储格式, 与 Album 相比移除了 ID 字段, 且替换 Artists 与 Tracks 字段为 []string, 即 ID
// AlbumMap 会自动存储 TrackID, 即专辑中的所有歌曲(未实现)
type AlbumMap struct {
	Name        string          `json:"name"`
	ArtistsIDs  []string        `json:"artists_ids"`
	Images      []spotify.Image `json:"images"`
	ReleaseDate string          `json:"release_date"`
	TotalTracks int             `json:"total_tracks"`
	//Genres      []string        `json:"genres"`
	Popularity int `json:"popularity"`
	//TracksIDs   []string        `json:"tracks_ids"`
}

// getInfoByID 的 idType 参数应使用 TypeArtist TypeTrack TypeAlbum, 若目标不存在会返回 nil
func getInfoByID(dbc dbClient, id string, idType rune) (interface{}, error) {
	info, err := dbc.GetMapStr("spotify-ids", id)
	if err != nil {
		return nil, err
	}

	if info == "" {
		//slog.Warn("数据库缺少此 ID 信息")
		return nil, nil
	}

	var i interface{}

	if idType == TypeArtist {
		i = &ArtistMap{}
	} else if idType == TypeTrack {
		i = &TrackMap{}
	} else {
		i = &AlbumMap{}
	}

	err = json.Unmarshal([]byte(info), i)
	if err != nil {
		return nil, err
	}

	return i, nil
}

func (c *Client) getArtistCache(dbc dbClient, id string) (*Artist, error) {
	exists, err := dbc.CheckIfMapFieldExists("spotify-ids", id)
	if err != nil {
		return nil, err
	}

	if exists {
		info, err := getInfoByID(dbc, id, TypeArtist)
		if err != nil {
			return nil, err
		}

		m := info.(*ArtistMap)

		return &Artist{
			Name:       m.Name,
			ID:         id,
			Popularity: m.Popularity,
			Genres:     m.Genres,
			Followers:  m.Followers,
			Images:     m.Images,
		}, nil
	}

	//slog.Debug("数据库中缺少此 ID 信息, 从 Spotify 同步并存储", "ID", id, "类型", "Artist")

	artist, err := c.C.GetArtist(c.Ctx, spotify.ID(id))
	if err != nil {
		return nil, err
	}

	convertedArtist := c.convertArtist(artist)

	err = saveID(dbc, id, convertedArtist.toMap())
	if err != nil {
		return nil, err
	}

	slog.Debug("同步并存储成功", "名称", convertedArtist.Name, "ID", id, "类型", "Artist")

	return convertedArtist, nil
}

func (c *Client) getAlbumCache(dbc dbClient, id string) (*Album, error) {
	exists, err := dbc.CheckIfMapFieldExists("spotify-ids", id)
	if err != nil {
		return nil, err
	}

	if exists {
		info, err := getInfoByID(dbc, id, TypeAlbum)
		if err != nil {
			return nil, err
		}

		m := info.(*AlbumMap)

		var artists []Artist

		for _, artistID := range m.ArtistsIDs {
			a, err := c.getArtistCache(dbc, artistID)
			if err != nil {
				return nil, err
			}

			artists = append(artists, *a)
		}

		return &Album{
			Name:        m.Name,
			Artists:     artists,
			ID:          id,
			Images:      m.Images,
			ReleaseDate: m.ReleaseDate,
			TotalTracks: m.TotalTracks,
			Popularity:  m.Popularity,
		}, nil
	}

	//slog.Debug("数据库中缺少此 ID 信息, 从 Spotify 同步并存储", "ID", id, "类型", "Album")

	album, err := c.C.GetAlbum(c.Ctx, spotify.ID(id))
	if err != nil {
		return nil, err
	}

	convertedAlbum, err := c.convertAlbum(dbc, album)
	if err != nil {
		return nil, err
	}

	convertedAlbumM := convertedAlbum.toMap()

	err = saveID(dbc, id, convertedAlbumM)
	if err != nil {
		return nil, err
	}

	slog.Debug("同步并存储成功", "名称", convertedAlbum.Name, "ID", id, "类型", "Album")

	return convertedAlbum, nil
}

func (c *Client) getTrackCache(dbc dbClient, id string) (*Track, error) {
	exists, err := dbc.CheckIfMapFieldExists("spotify-ids", id)
	if err != nil {
		return nil, err
	}

	if exists {
		info, err := getInfoByID(dbc, id, TypeTrack)
		if err != nil {
			return nil, err
		}

		m := info.(*TrackMap)

		album, err := c.getAlbumCache(dbc, m.AlbumID)
		if err != nil {
			return nil, err
		}

		var artists []Artist

		for _, artistID := range m.ArtistsIDs {
			a, err := c.getArtistCache(dbc, artistID)
			if err != nil {
				return nil, err
			}

			artists = append(artists, *a)
		}

		return &Track{
			Album:      *album,
			Artists:    artists,
			Duration:   m.Duration,
			ID:         id,
			Name:       m.Name,
			Popularity: m.Popularity,
		}, nil
	}

	//slog.Debug("数据库中缺少此 ID 信息, 从 Spotify 同步并存储", "ID", id, "类型", "Track")

	track, err := c.C.GetTrack(c.Ctx, spotify.ID(id))
	if err != nil {
		return nil, err
	}

	convertedTrack, err := c.convertTrack(dbc, track)
	if err != nil {
		return nil, err
	}

	convertedTrackM := convertedTrack.toMap()

	err = saveID(dbc, id, convertedTrackM)
	if err != nil {
		return nil, err
	}

	slog.Debug("同步并存储成功", "名称", convertedTrack.Name, "ID", id, "类型", "Track")

	return convertedTrack, nil
}
