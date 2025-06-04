package spotify

import (
	"context"
	"encoding/json"
	"github.com/zmb3/spotify/v2"
	"log/slog"
	"time"
)

type Client struct {
	C   *spotify.Client
	Ctx context.Context
}

const (
	TypeArtist = 'a'
	TypeTrack  = 'i'
	TypeAlbum  = 'A'
)

type Artist struct {
	Name       string          `json:"name"`
	ID         string          `json:"id"`
	Popularity int             `json:"popularity"`
	Genres     []string        `json:"genres"`
	Followers  int             `json:"followers"`
	Images     []spotify.Image `json:"images"`
}

type Track struct {
	Album    Album    `json:"album"`
	Artists  []Artist `json:"artists"`
	Duration string   `json:"duration"`
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	//ExternalIDs map[string]string `json:"external_ids"`
	Popularity int `json:"popularity"`
	//IsPlayable  *bool             `json:"is_playable"`
	//LinkedFrom  *spotify.LinkedFromInfo   `json:"linked_from"
}

// Album 的 Track 字段中的 Album 字段应直接等于此 Album, 否则会造成死循环
type Album struct {
	Name        string          `json:"name"`
	Artists     []Artist        `json:"artists"`
	ID          string          `json:"id"`
	Images      []spotify.Image `json:"images"`
	ReleaseDate string          `json:"release_date"`
	TotalTracks int             `json:"total_tracks"`
	//Copyrights  []Copyright       `json:"copyrights"`
	//Genres     []string `json:"genres"` // 已被 Spotify 弃用
	Popularity int `json:"popularity"`
	//Tracks     []Track  `json:"tracks"`
	//ExternalIDs map[string]string `json:"external_ids"`
}

func (a *Artist) toMap() *ArtistMap {
	return &ArtistMap{
		Name:       a.Name,
		Popularity: a.Popularity,
		Genres:     a.Genres,
		Followers:  a.Followers,
		Images:     a.Images,
	}
}

func (a *Album) toMap() *AlbumMap {
	return &AlbumMap{
		Name:        a.Name,
		ArtistsIDs:  fmtArtistsIDs(a.Artists),
		Images:      a.Images,
		ReleaseDate: a.ReleaseDate,
		TotalTracks: a.TotalTracks,
		Popularity:  a.Popularity,
	}
}

func (t *Track) toMap() *TrackMap {
	return &TrackMap{
		AlbumID:    t.Album.ID,
		ArtistsIDs: fmtArtistsIDs(t.Artists),
		Duration:   t.Duration,
		Name:       t.Name,
		Popularity: t.Popularity,
	}
}

type PlayedTrack struct {
	Track
	PlayedAt string `json:"played_at"`
	//EmbedURL string      `json:"embed_url"`
}

func (c *Client) runGroupShort(dbc dbClient) {
	slog.Info("开始保存播放记录与其它信息, 请勿在结束前退出程序")
	defer slog.Info("保存播放记录与其它信息结束")

	err := c.saveRecentlyPlayedTracks(dbc)
	for err != nil {
		slog.Warn("Spotify 获取或存储最近播放失败, 一分钟后重试", "error", err)
		time.Sleep(time.Minute)
		err = c.saveRecentlyPlayedTracks(dbc)
	}

	err = c.saveHourlyPlayedCount(dbc)
	for err != nil {
		slog.Warn("Spotify 存储每小时收听量失败, 一分钟后重试", "error", err)
		time.Sleep(time.Minute)
		err = c.saveHourlyPlayedCount(dbc)
	}

}

func (c *Client) runGroupLong(dbc dbClient) {
	err := c.saveTopArtists(dbc)
	for err != nil {
		slog.Warn("Spotify 获取或存储热门艺术家失败, 一分钟后重试", "error", err)
		time.Sleep(time.Minute)
		err = c.saveTopArtists(dbc)
	}

	err = c.saveTopTracks(dbc)
	for err != nil {
		slog.Warn("Spotify 获取或存储热门曲目失败, 一分钟后重试", "error", err)
		time.Sleep(time.Minute)
		err = c.saveTopTracks(dbc)
	}
}

// Run 运行所有定时任务, 阻塞, 若不清楚参数则推荐设置为 time.Hour
func (c *Client) Run(dbc dbClient, recentlyPlayedDuration time.Duration) {
	c.runGroupShort(dbc)
	c.runGroupLong(dbc)

	recentlyPlayedTicker := time.NewTicker(recentlyPlayedDuration)
	defer recentlyPlayedTicker.Stop()

	go func() {
		for range recentlyPlayedTicker.C {
			c.runGroupShort(dbc)
		}
	}()

	topsTicker := time.NewTicker(time.Hour * 24)
	defer topsTicker.Stop()

	for range topsTicker.C {
		c.runGroupLong(dbc)
	}
}

// 存储格式为 ArtistMap TrackMap AlbumMap
func saveID(dbc dbClient, id string, data interface{}) error {
	j, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = dbc.SetMap("spotify-ids", id, string(j))
	if err != nil {
		return err
	}

	return nil
}

func fmtArtistsIDs(artists []Artist) []string {
	var res []string
	for _, artist := range artists {
		res = append(res, artist.ID)
	}
	return res
}

func (c *Client) convertArtist(artist *spotify.FullArtist) *Artist {
	return &Artist{
		Name:       artist.Name,
		ID:         artist.ID.String(),
		Popularity: int(artist.Popularity),
		Genres:     artist.Genres,
		Followers:  int(artist.Followers.Count),
		Images:     artist.Images,
	}
}

func (c *Client) convertAlbum(dbc dbClient, album *spotify.FullAlbum) (*Album, error) {
	var artists []Artist

	for _, artist := range album.Artists {
		a, err := c.getArtistCache(dbc, string(artist.ID))
		if err != nil {
			return nil, err
		}

		artists = append(artists, *a)
	}

	return &Album{
		Name:        album.Name,
		Artists:     artists,
		ID:          album.ID.String(),
		Images:      album.Images,
		ReleaseDate: album.ReleaseDate,
		TotalTracks: int(album.TotalTracks),
		Popularity:  int(album.Popularity),
	}, nil
}

func (c *Client) convertTrack(dbc dbClient, track *spotify.FullTrack) (*Track, error) {
	var albumArtists []Artist

	for _, artist := range track.Album.Artists {
		a, err := c.getArtistCache(dbc, string(artist.ID))
		if err != nil {
			return nil, err
		}

		albumArtists = append(albumArtists, *a)
	}

	album, err := c.getAlbumCache(dbc, string(track.Album.ID))
	if err != nil {
		return nil, err
	}

	var trackArtists []Artist

	for _, artist := range track.Artists {
		a, err := c.getArtistCache(dbc, string(artist.ID))
		if err != nil {
			return nil, err
		}

		trackArtists = append(trackArtists, *a)
	}

	return &Track{
		Album:      *album,
		Artists:    trackArtists,
		Duration:   time.UnixMilli(int64(track.Duration)).UTC().Format(time.TimeOnly),
		ID:         track.ID.String(),
		Name:       track.Name,
		Popularity: int(track.Popularity),
	}, nil
}

var garbageWords = []string{"remastered", "remaster", "remix", "reissue"}
