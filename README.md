# Spotify Insights

## 介绍
收集并分析Spotify收听数据

## 使用示例
### 你需要设置[SPOTIFY_SECRET和SPOTIFY_SECRET](https://github.com/zmb3/spotify)进环境变量
### 本项目要求一个SPOTIFY_KEY用于加密Spotify令牌, 执行```openssl rand -base64 32```生成
```
package main

import (
    "github.com/HenTaku321/spotify-insights-go"
    "github.com/HenTaku321/valkey-go"
    "log/slog"
    "os"
    "time"
)

func main() {
	vc, err := valkey.NewClient("127.0.0.1:6379", "", 0, true)
	if err != nil {
		panic(err)
	}
	defer vc.C.Close()

	sc := spotify.GetClient(vc, []byte(os.Getenv("SPOTIFY_KEY")))
	sc.Run(vc, time.Hour)
}

```
### 目前可用的功能(更新中):
```
Run - 运行需要的定时任务
GetPlayedRangeDuringATime
GetTopAlbumsIDs
GetTopArtistsIDs
GetTopTracksIDs
GetPlayedHistory
GetPlayedHistoryByIndex
GetPlayedHistoryIDs
GetPlayedHistoryIDByIndex
GetCurrentlyPlayingTrack
GetPlayedRangeOnADay
GetTotalPlayedCount
```
