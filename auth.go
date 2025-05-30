package spotify

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zmb3/spotify/v2"
	"github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const redirectURI = "http://127.0.0.1:8080/callback"

var (
	auth = spotifyauth.New(spotifyauth.WithRedirectURL(redirectURI), spotifyauth.WithScopes(
		spotifyauth.ScopePlaylistReadPrivate,
		spotifyauth.ScopeUserFollowModify,
		spotifyauth.ScopeUserFollowRead,
		spotifyauth.ScopeUserReadPrivate,
		spotifyauth.ScopeUserReadCurrentlyPlaying,
		spotifyauth.ScopeUserReadPlaybackState,
		spotifyauth.ScopeUserModifyPlaybackState,
		spotifyauth.ScopeUserReadRecentlyPlayed,
		spotifyauth.ScopeUserTopRead,
		spotifyauth.ScopeStreaming,
	))
	ch    = make(chan *spotify.Client) // 等待用户登录成功
	state = rand.Text()
)

type StoredSpotifyToken struct {
	EncryptedAccessToken  string    `json:"encrypted_access_token"`
	AccessTokenNonce      string    `json:"access_token_nonce"`
	EncryptedRefreshToken string    `json:"encrypted_refresh_token"`
	RefreshTokenNonce     string    `json:"refresh_token_nonce"`
	TokenType             string    `json:"token_type"`
	Expiry                time.Time `json:"expiry"`
}

// encrypt 使用 AES-GCM 加密数据
// key AES-128 AES-192 AES-256
func encrypt(key []byte, plaintext []byte) (ciphertext []byte, nonce []byte, err error) {
	// 创建 AES 分组密码实例
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 创建 GCM 认证加密模式的 AEAD 实例
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 GCM 实例失败: %w", err)
	}

	// Nonce 不需要保密
	nonce = make([]byte, gcm.NonceSize()) // 通常返回 12
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("生成 Nonce 失败: %w", err)
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)

	return ciphertext, nonce, nil
}

// decrypt 使用 AES-GCM 解密数据
// key 和 nonce 必须与加密时使用的相同
func decrypt(key []byte, ciphertext []byte, nonce []byte) (plaintext []byte, err error) {
	// 创建 AES 分组密码实例
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 创建 GCM 模式的 AEAD 实例
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 实例失败: %w", err)
	}

	// 检查 Nonce 长度是否与 GCM 期望的一致
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("Nonce 长度错误，期望 %d, 得到 %d", gcm.NonceSize(), len(nonce))
	}

	plaintext, err = gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败 (可能密钥、Nonce错误或数据被篡改): %w", err)
	}

	return plaintext, nil
}

func saveToken(dbc dbClient, token *oauth2.Token, key []byte) error {
	cipherAccessToken, accessTokenNonce, err := encrypt(key, []byte(token.AccessToken))
	if err != nil {
		return fmt.Errorf("加密 Access Token 失败: %w", err)
	}

	cipherRefreshToken, refreshTokenNonce, err := encrypt(key, []byte(token.RefreshToken))
	if err != nil {
		return fmt.Errorf("加密 Refresh Token 失败: %w", err)
	}

	storableToken := StoredSpotifyToken{
		EncryptedAccessToken:  base64.StdEncoding.EncodeToString(cipherAccessToken),
		AccessTokenNonce:      base64.StdEncoding.EncodeToString(accessTokenNonce),
		EncryptedRefreshToken: base64.StdEncoding.EncodeToString(cipherRefreshToken),
		RefreshTokenNonce:     base64.StdEncoding.EncodeToString(refreshTokenNonce),
		TokenType:             token.TokenType,
		Expiry:                token.Expiry,
	}

	b, err := json.MarshalIndent(storableToken, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化令牌失败: %w", err)
	}

	err = dbc.SetString("spotify-token", string(b), nil)
	if err != nil {
		return fmt.Errorf("存储令牌到数据库失败: %w", err)
	}
	return nil
}

func getToken(dbc dbClient, key []byte) (*oauth2.Token, error) {
	str, err := dbc.GetString("spotify-token")
	if err != nil {
		return nil, fmt.Errorf("从数据库获取令牌字符串失败: %w", err)
	}
	if str == "" {
		return nil, errors.New("加载的令牌无效 (数据库中字符串为空)")
	}

	var storedToken StoredSpotifyToken
	err = json.Unmarshal([]byte(str), &storedToken)
	if err != nil {
		return nil, fmt.Errorf("反序列化存储的令牌失败: %w", err)
	}

	cipherAccessToken, err := base64.StdEncoding.DecodeString(storedToken.EncryptedAccessToken)
	if err != nil {
		return nil, fmt.Errorf("Base64 解码 Access Token 失败: %w", err)
	}
	accessTokenNonce, err := base64.StdEncoding.DecodeString(storedToken.AccessTokenNonce)
	if err != nil {
		return nil, fmt.Errorf("Base64 解码 Access Token Nonce 失败: %w", err)
	}
	accessToken, err := decrypt(key, cipherAccessToken, accessTokenNonce)
	if err != nil {
		return nil, fmt.Errorf("解密 Access Token 失败: %w", err)
	}

	cipherRefreshToken, err := base64.StdEncoding.DecodeString(storedToken.EncryptedRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("Base64 解码 Refresh Token 失败: %w", err)
	}
	refreshTokenNonce, err := base64.StdEncoding.DecodeString(storedToken.RefreshTokenNonce)
	if err != nil {
		return nil, fmt.Errorf("Base64 解码 Refresh Token Nonce 失败: %w", err)
	}
	refreshToken, err := decrypt(key, cipherRefreshToken, refreshTokenNonce)
	if err != nil {
		return nil, fmt.Errorf("解密 Refresh Token 失败: %w", err)
	}

	finalToken := &oauth2.Token{
		AccessToken:  string(accessToken),
		RefreshToken: string(refreshToken),
		TokenType:    storedToken.TokenType,
		Expiry:       storedToken.Expiry,
	}

	if finalToken.AccessToken == "" && finalToken.RefreshToken == "" {
		return nil, errors.New("解密后的令牌无效 (缺少 AccessToken 和 RefreshToken)")
	}
	return finalToken, nil
}

func GetClient(dbc dbClient, key []byte) *Client {
	if key == nil || len(key) == 0 {
		str, err := dbc.GetString("spotify-token")
		if err != nil {
			panic(err)
		}

		if str == "" {
			slog.Warn("初次运行此程序, 请在退出后运行openssl rand -base64 32, 用来生成密钥以加密 Spotify 令牌, 请妥善保管, 下次启动程序时将从环境变量(SPOTIFY_KEY)中读取密钥, 若密钥泄漏则需要清空数据库中的 spotify-token 并重新生成密钥")
			os.Exit(0)
		} else {
			slog.Error("未设置环境变量 SPOTIFY_KEY")
			os.Exit(1)
		}
	}

	var err error
	key, err = base64.StdEncoding.DecodeString(string(key))
	if err != nil {
		slog.Error("Base64 解码 SPOTIFY_KEY 失败", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	persistedToken, err := getToken(dbc, key)
	if err == nil && persistedToken.Valid() {
		httpClient := auth.Client(ctx, persistedToken)
		spotifyClient := spotify.New(httpClient)
		return &Client{C: spotifyClient, Ctx: ctx}
	}

	if err == nil && persistedToken.RefreshToken != "" {
		slog.Debug("访问令牌已过期或无效，但有刷新令牌，尝试刷新...")

		newToken, err := auth.RefreshToken(ctx, persistedToken)
		httpClient := auth.Client(ctx, newToken)
		spotifyClient := spotify.New(httpClient)

		_, testErr := spotifyClient.CurrentUser(ctx)
		if testErr == nil {
			slog.Debug("使用已加载/已刷新的令牌创建客户端成功")

			if err = saveToken(dbc, newToken, key); err != nil {
				slog.Warn("无法保存令牌", "error", err)
				// 即使保存失败，本次会话仍然可以使用这个令牌
			}

			return &Client{C: spotifyClient, Ctx: ctx}
		}
		slog.Warn("使用加载的令牌创建客户端后, 测试API调用失败, 将进行网页授权", "error", testErr)
	}

	if err != nil {
		slog.Info("加载令牌失败或令牌无效, 将启动网页授权流程", "error", err)
	}

	server := &http.Server{Addr: ":8080"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		completeAuthAndSaveToken(dbc, w, r, server, key) // 传入 server 以便关闭它
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务器错误: ", "error", err)
		}
	}()

	url := auth.AuthURL(state)
	slog.Info("请登录", "url", url)

	spotifyClient := <-ch
	slog.Info("通过网页授权成功获取 Spotify 客户端")
	return &Client{C: spotifyClient, Ctx: ctx}
}

func completeAuthAndSaveToken(dbc dbClient, w http.ResponseWriter, r *http.Request, server *http.Server, key []byte) {
	ctx := r.Context()
	tok, err := auth.Token(ctx, state, r)
	if err != nil {
		http.Error(w, "无法获得令牌", http.StatusForbidden)
		slog.Error("无法获得令牌", "error", err)
		ch <- nil // 发送一个nil表示失败，或者根本不发送然后让主流程超时
		return
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		slog.Error("State 不匹配", "得到", st, "期望", state)
		ch <- nil
		return
	}

	// 令牌获取成功
	if err := saveToken(dbc, tok, key); err != nil {
		slog.Warn("无法保存令牌", "error", err)
		// 即使保存失败，本次会话仍然可以使用这个令牌
	}

	client := spotify.New(auth.Client(ctx, tok))

	_, err = w.Write([]byte("登录成功"))
	if err != nil {
		slog.Error("写入响应失败", "error", err)
	}

	ch <- client

	go func() {
		if err := server.Shutdown(context.Background()); err != nil {
			slog.Error("关闭回调服务器失败", "error", err)
		}
	}()
}
