# Spotify Insights

## 介绍
收集并分析Spotify收听数据

## 使用示例
### 你需要设置[SPOTIFY_SECRET和SPOTIFY_SECRET](https://github.com/zmb3/spotify)进环境变量
### 本项目要求一个SPOTIFY_KEY用于加密Spotify令牌, 执行```openssl rand -base64 32```生成
```
package main

import (
	"crypto/tls"
	"log/slog"
	"os"
	"time"

	"github.com/HenTaku321/spotify-insights-go"
	"github.com/HenTaku321/valkey-go"
)

func main() {
	vPasswd := os.Getenv("VALKEY_PASSWD")
	vc, err := valkey.NewClient("valkey.customdom.eu.org:6379", vPasswd, 0, true, &tls.Config{})
	if err != nil {
		if vPasswd == "" {
			defer slog.Info("程序退出, 但与服务器的连接可能未被关闭") // 
		}
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
GetHourlyPlayedCount
```
