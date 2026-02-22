//go:build server

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	http.HandleFunc("/api/config", a.HandleGetConfig) // 👈 関数自体を外に出す
	http.HandleFunc("/api/channels", a.HandleGetChannels)
	http.HandleFunc("/api/reload-channels", a.HandleReloadChannels)
	http.HandleFunc("/api/messages", a.HandleGetMessages)
	http.HandleFunc("/api/auth-url", a.HandleGetAuthURL)
	http.HandleFunc("/api/message-body", a.HandleGetMessageBody)
	http.HandleFunc("/api/sync", a.HandleSyncMessages)
	http.HandleFunc("/api/sync-historical", a.HandleSyncHistoricalMessages)
	http.HandleFunc("/api/summarize", a.HandleSummarizeEmail)
	http.HandleFunc("/api/set-importance", a.HandleSetManualImportance)
	http.HandleFunc("/api/trash", a.HandleTrash)
	http.HandleFunc("/api/ai-search", a.HandleAISearch)
	http.HandleFunc("/api/mark-read", a.HandleMarkRead)
	http.HandleFunc("/auth/callback", a.HandleAuthCallback)
	http.HandleFunc("/api/complete-auth", a.HandleCompleteAuth)

}

// 🌟 1. メールの同期（最新件数）
func (a *App) HandleSyncMessages(w http.ResponseWriter, r *http.Request) {
	if a.srv == nil {
		http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
		return
	}
	err := a.SyncMessages()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// 🌟 2. 過去メールの同期（Load More用）
func (a *App) HandleSyncHistoricalMessages(w http.ResponseWriter, r *http.Request) {
	if a.srv == nil {
		http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
		return
	}
	token := r.URL.Query().Get("token")
	// 既存の関数を呼び出し、新しいトークンを返す
	newToken, err := a.SyncHistoricalMessages(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(newToken))
}

// 🌟 3. AI ベクトル検索（関連メール）
func (a *App) HandleAISearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	// 既存のベクトル検索ロジックを呼び出す
	results, err := a.GetAISearchResults(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// 🌟 4. ゴミ箱へ移動
func (a *App) HandleTrash(w http.ResponseWriter, r *http.Request) {
	if a.srv == nil {
		http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
		return
	}
	id := r.URL.Query().Get("id")
	err := a.TrashMessage(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "trashed", "id": id})
}

// 🌟 5. AI 要約
func (a *App) HandleSummarize(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	summary, err := a.SummarizeEmail(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(summary))
}

// 🌟 1. AI 要約の実行
func (a *App) HandleSummarizeEmail(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	// 既存の要約ロジックを呼び出す
	summary, err := a.SummarizeEmail(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 要約文はプレーンテキストなのでそのまま返す
	w.Write([]byte(summary))
}

// 🌟 2. 重要度の手動設定（1〜5ボタン）
func (a *App) HandleSetManualImportance(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	levelStr := r.URL.Query().Get("level")

	level, err := strconv.Atoi(levelStr)
	if err != nil || id == "" {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	// 既存の重要度更新ロジックを呼び出す
	err = a.SetManualImportance(id, level)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 成功の合図をJSONで返す（Reactのエラーを防ぐため）
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"id":         id,
		"importance": level,
	})
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

func (a *App) HandleReloadChannels(w http.ResponseWriter, r *http.Request) {
	// 1. JSONファイルからDBへ再読み込みを実行
	err := a.LoadChannelsFromJson()
	if err != nil {
		http.Error(w, "リロード失敗: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. 🌟 Reactが喜ぶ「オブジェクト形式」でDBから再取得
	rows, err := a.db.Query("SELECT name FROM channels ORDER BY id ASC")
	if err != nil {
		http.Error(w, "再取得失敗", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Reactに渡すための型をその場で定義
	type ChannelResp struct {
		Name string `json:"name"`
	}
	channels := []ChannelResp{} // 🌟 空配列 [] で初期化

	for rows.Next() {
		var name string
		rows.Scan(&name)
		channels = append(channels, ChannelResp{Name: name})
	}

	// 3. JSONで返却 (例: [{"name": "📥 受信トレイ"}, ...])
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channels)

	fmt.Printf("♻️ チャンネル設定をリロード完了: %d 件\n", len(channels))
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
}

func (a *App) HandleGetMessageBody(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	body, err := a.GetMessageBody(id) // 既存の関数を呼ぶだけ！
	if err != nil {
		fmt.Printf("Body err: %s\n", err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(body))
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

func (a *App) HandleMarkRead(w http.ResponseWriter, r *http.Request) {
	// 1. クエリパラメータから ID を取得
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	// 2. 既存の既読化ロジックを実行
	// ※内部で a.db.Exec("UPDATE messages SET is_read = 1 ...") をしているはずです
	err := a.MarkAsRead(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 🌟 3. 現代的な返信（空っぽだと React がエラーを吐くので）
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "id": id})

	fmt.Printf("📖 既読にしました: %s\n", id)
}

func (a *App) HandleCompleteAuth(w http.ResponseWriter, r *http.Request) {
	// 1. URLパラメータまたはボディから code を取得
	// React側が api.js の fetchApi で送ってくる形式に合わせます
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "認証コードが空です", http.StatusBadRequest)
		return
	}

	// 2. 🌟 既存の CompleteAuth を呼び出す (ここで token.json 保存 & a.srv 起動)
	err := a.CompleteAuth(code)
	if err != nil {
		fmt.Printf("❌ 認証完了処理に失敗: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. 成功のレスポンスを返す
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})

	fmt.Println("🔓 Web経由での認証が正常に完了しました")
}
