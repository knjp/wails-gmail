//go:build server

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	// 1. Appインスタンスの作成と初期化（DB接続や設定読み込み）
	app := NewApp()
	app.startup(context.Background()) // 🌟 既存の startup をそのまま流用！
	app.registerHandlers()

	fmt.Println("🚀 サーバー起動中: http://localhost:8080")
	// 🌟 0.0.0.0 で待ち受けることで、Dockerや外部ブラウザからもアクセス可能に
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func (a *App) registerHandlers() {
	// 静的ファイル (React)
	fs := http.FileServer(http.Dir("frontend/dist"))
	http.Handle("/", fs)

	// API 群
	http.HandleFunc("/api/config", a.HandleGetConfig)     // 👈 関数自体を外に出す
	http.HandleFunc("/api/channels", a.HandleGetChannels) // 👈 これから増える分
	http.HandleFunc("/api/messages", a.HandleGetMessages)
	http.HandleFunc("/auth/callback", a.HandleAuthCallback)
	http.HandleFunc("/api/auth-url", a.HandleGetAuthURL)

	// 2. 認証URL取得o窓口
	/*
		http.HandleFunc("/api/auth-url", func(w http.ResponseWriter, r *http.Request) {
			url, _ := a.GetAuthURL() // token.jsonがあれば空、なければURLが返る
			fmt.Fprint(w, url)
		})
	*/
}

// HandleGetConfig: 設定を返す窓口
func (a *App) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a.GetConfig())
}

// HandleGetChannels: チャンネルを返す窓口
func (a *App) HandleGetChannels(w http.ResponseWriter, r *http.Request) {
	// 1. DBからチャンネル名（文字列配列）を取得
	// 既存の loadChannelsFromJson または DBクエリの結果を使います
	channels, err := a.GetChannels()
	if err != nil {
		http.Error(w, "チャンネル取得失敗", http.StatusInternalServerError)
		return
	}

	// 2. JSONで返却
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)
}

func (a *App) HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	// 1. URLパラメータからチャンネル名を取得
	channelName := r.URL.Query().Get("name")
	if channelName == "" {
		http.Error(w, "チャンネル名が指定されていません", http.StatusBadRequest)
		return
	}

	// 2. 既存のロジックでDBからメールを取得
	messages, err := a.GetMessagesByChannel(channelName)
	if err != nil {
		http.Error(w, "メッセージ取得失敗: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. JSONで返却
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)

	/*
		if a.srv == nil {
			http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
			return
		}
		channel := r.URL.Query().Get("name")
		messages, _ := a.GetMessagesByChannel(channel)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	*/
}

func (a *App) HandleGetAuthURL(w http.ResponseWriter, r *http.Request) {
	// 🌟 タイムアウトを設定したコンテキストで実行するのが現代的作法
	_, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	fmt.Println("🔍 認証URLをチェック中...")

	// 🌟 既存の GetAuthURL を呼ぶが、もし重いならここで return させる
	url, err := a.GetAuthURL()
	if err != nil {
		fmt.Printf("❌ AuthURL取得失敗: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 文字列を確実に返す
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(url))
	fmt.Printf("✅ AuthURLを返却しました: [%s]\n", url)
}
