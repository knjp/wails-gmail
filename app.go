package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	_ "modernc.org/sqlite"

	"github.com/ollama/ollama/api"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type MessageSummary struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	Recipient  string `json:"recipient"`
	Subject    string `json:"subject"`
	Snippet    string `json:"snippet"`
	IsRead     int64  `json:"is_read"`
	Importance int64  `json:"importance"`
	//Date       string `json:"date"`
	Timestamp int64  `json:"timestamp"`
	Deadline  string `json:"deadline"`
}

type ChannelConfig struct {
	Name    string `json:"name"`
	Query   string `json:"query"`
	TTLdays int    `json:"ttl_days"`
}

type Channel struct {
	Name string `json:"name"`
}

type App struct {
	ctx        context.Context
	srv        *gmail.Service
	db         *sql.DB
	store      *Store
	ollama     *api.Client
	isCleaning bool
}

const (
	channelsFile    = "config/channels.json"
	channelsExample = "config/channels.json.example"
	settingsFile    = "config/settings.json"
	tokenFile       = "config/token.json"
	credentialsFile = "config/credentials.json"
	mailDBFile      = "db/mail_cache.db"
)

type SearchResult struct {
	ID    string  `json:"id"`
	Score float32 `json:"score"`
}

type Config struct {
	MyAddress    string `json:"my_address"`
	OllamaHost   string `json:"ollama_host"`
	OllamaModel  string `json:"ollama_model"`
	EmbedModel   string `json:"embed_model"`
	SyncInterval int    `json:"sync_interval"`
}

var globalConfig Config

func NewApp() *App {
	return &App{}
}

func (a *App) GetConfig() Config {
	return globalConfig
}

func (a *App) LoadChannelsFromJson() error {
	if _, err := os.Stat(channelsFile); os.IsNotExist(err) {
		// target が存在しない場合、example があるか確認
		if data, err := os.ReadFile(channelsExample); err == nil {
			// example の中身を target に書き込む（＝コピー）
			os.WriteFile(channelsFile, data, 0644)
			fmt.Println("📝 example から設定ファイルを作成しました")
		} else {
			// example もない場合は、「最低限のデフォルト」を作成
			defaultChannels := `[
	{"name": "📥 受信トレイ", "query": "1=1", "ttl_days": 0},
	{"name": "📥 00未読", "query": "id IN (SELECT id FROM messages WHERE sender NOT LIKE '%メルマガ%') AND is_read = 0", "ttl_days": 0},
	{"name": "gmail", "query": "sender LIKE '%gmail%' OR sender LIKE '%google%'", "ttl_days": 0},
	{"name": "amazon", "query": "sender LIKE '%amazon.co.jp%' OR sender LIKE '%amazon.com%'", "ttl_days": 0}
]`
			os.WriteFile(channelsFile, []byte(defaultChannels), 0644)
			fmt.Println("⚠️ デフォルト設定を作成しました")
		}
	}

	data, err := os.ReadFile(channelsFile)
	if err != nil {
		return fmt.Errorf("ファイルの読み込みに失敗: %w", err) // 🌟 黙って return しない
	}

	var configs []ChannelConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("JSONの解析に失敗: %w", err)
	}

	// 🌟 念のための空チェック（ここを通れば DB は守られます）
	if len(configs) == 0 {
		return fmt.Errorf("設定ファイルが空です")
	}

	// 🌟 トランザクションで「全消し」と「書き込み」をセットにする
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}

	tx.Exec("DELETE FROM channels")
	for _, c := range configs {
		_, err := tx.Exec("INSERT INTO channels (name, sql_condition, ttl_days) VALUES (?, ?, ?)",
			c.Name, c.Query, c.TTLdays)
		if err != nil {
			tx.Rollback() // 失敗したら元に戻す
			return err
		}
	}
	return tx.Commit() // 🌟 ここで初めて DB が書き換わる
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 1. フォルダと設定の準備
	a.initDirs()
	a.loadSettings()

	// 2. データベースのセットアップ (テーブル・インデックス作成含む)
	if err := a.initDB(); err != nil {
		log.Fatalf("❌ データベース初期化失敗: %v", err)
	}

	// 3. AI (Ollama) の準備
	a.initAI()

	// 4. Gmail サービスの初期化 (認証がある場合のみ)
	// 🌟 ここで失敗しても、React側のモーダルで対応するので return で OK
	if err := a.initGmailService(); err != nil {
		fmt.Printf("💡 認証待ちの状態です: %v\n", err)
	}

	// 5. バックグラウンドタスクの開始
	go a.startBackgroundTasks()
}

func (a *App) initDirs() {
	os.MkdirAll("db", 0755)
	os.MkdirAll("config", 0755)
}

func (a *App) loadSettings() {
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		// ファイルがない場合はデフォルト値をセットして保存しておく（親切設計）
		globalConfig = Config{
			MyAddress:    "your-email@gmail.com",
			OllamaHost:   "http://localhost:11434",
			OllamaModel:  "qwen2.5:1.5b",
			EmbedModel:   "nomic-embed-text",
			SyncInterval: 60,
		}
		defaultData, _ := json.MarshalIndent(globalConfig, "", "  ")
		os.WriteFile(settingsFile, defaultData, 0644)
		fmt.Println("📝 デフォルト設定ファイルを作成しました")
	} else {
		// 既存ファイルを構造体に流し込む
		json.Unmarshal(data, &globalConfig)
		fmt.Println("🚀 設定を読み込みました:", globalConfig.OllamaModel)
	}
}

func (a *App) initDB() error {
	db, err := sql.Open("sqlite", mailDBFile)
	if err != nil {
		log.Fatal(err)
		return err
	}

	a.db = db
	a.db.SetMaxIdleConns(1) // 待機中の接続を5個キープ
	a.db.Exec("PRAGMA busy_timeout=10000")
	a.db.Exec("PRAGMA journal_mode=WAL;")

	a.db.Exec(`CREATE TABLE IF NOT EXISTS channels (id INTEGER PRIMARY KEY, name TEXT UNIQUE, sql_condition TEXT, ttl_days INTEGER);`)
	a.LoadChannelsFromJson()

	// テーブル作成
	a.db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY, sender TEXT,
		recipient TEXT,
		subject TEXT,
		snippet TEXT,
		timestamp INTEGER,
		body TEXT,
		summary TEXT,
		is_read INTEGER DEFAULT 0,
		importance INTEGER DEFAULT 0,
		deadline DATETIME
	);`)

	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_deadline ON messages(deadline);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_is_read ON messages(deadline);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_importance ON messages(deadline);")
	fmt.Println("✅ インデックスの作成/確認が完了しました")

	s, err := NewStore(a.db)
	if err != nil {
		panic(err)
	}
	a.store = s
	return nil
}

func (a *App) initAI() error {
	u, err := url.Parse(globalConfig.OllamaHost)
	if err != nil {
		return fmt.Errorf("OllamaHostの形式が不正です: %w", err)
	}
	client := api.NewClient(u, http.DefaultClient)
	a.ollama = client
	fmt.Printf("🤖 AI 接続完了: %s (モデル: %s)\n", globalConfig.OllamaHost, globalConfig.OllamaModel)
	return nil
}

func (a *App) initGmailService() error {

	// Gmail API の初期化 (credentials.json と token.json がある前提)
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		// log.Printf("credentials.json 読み込み失敗: %v", err)
		return err
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailModifyScope)
	if err != nil {
		log.Printf("OAuth config 作成失敗: %v", err)
		return err
	}

	// getClient 関数を使って http.Client を取得
	client, err := a.getClient(config)
	if err != nil {
		return err
	}
	// サービスを構造体のフィールドに代入（これで「API未初期化」が消えます）
	srv, err := gmail.NewService(a.ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Printf("Gmail サービス作成失敗: %v", err)
		return err
	}
	a.srv = srv
	return nil
}

func (a *App) startBackgroundTasks() {
	go func() {
		time.Sleep(3 * time.Minute)
		for {
			a.RunAutoCleanup()
			// 次のお掃除まで1時間休む（config.jsonから読み込んでもOK）
			time.Sleep(1 * time.Hour)
		}

	}()

	go func() {
		interval := time.Duration(globalConfig.SyncInterval) * time.Second
		for {
			a.SyncMessages()
			time.Sleep(interval) // 🌟 設定値で待機
		}
	}()

}

func (a *App) GetAuthURL() (string, error) {

	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		return "MISSING_CREDENTIALS", nil
	}

	_, err := os.Stat(tokenFile)
	if err == nil {
		// 🌟 token.json が既に存在するなら、認証URLは不要
		return "", nil
	}

	// 存在しない場合は、新しい認証URLを生成して返す
	config, err := a.getOAuthConfig()
	if err != nil {
		return "", err
	}
	return config.AuthCodeURL("state-token", oauth2.AccessTypeOffline), nil
}

func (a *App) CompleteAuth(code string) error {
	config, err := a.getOAuthConfig()
	if err != nil {
		return err
	}
	tok, err := config.Exchange(context.TODO(), code)
	if err != nil {
		return err
	}
	saveToken(tokenFile, tok)
	client := config.Client(context.Background(), tok)
	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}
	a.srv = srv

	return nil
}

func (a *App) getOAuthConfig() (*oauth2.Config, error) {
	// 1. ダウンロードした秘密の鍵ファイルを読み込む
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("credentials.json が見つかりません: %v", err)
	}

	// 2. Google のライブラリを使って設定オブジェクトに変換
	// スコープは「メールの読み書き・削除」ができる GmailModify を指定
	config, err := google.ConfigFromJSON(b, gmail.GmailModifyScope)
	if err != nil {
		return nil, fmt.Errorf("認証設定の解析に失敗: %v", err)
	}
	return config, nil
}

// / getClient: トークンを読み込んで Client を返す（なければエラーを返す）
func (a *App) getClient(config *oauth2.Config) (*http.Client, error) {

	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("token.json がありません。認証が必要です")
	}

	tok := &oauth2.Token{}
	if err := json.Unmarshal(data, tok); err != nil {
		return nil, err
	}

	return config.Client(context.Background(), tok), nil
}

// saveToken: トークンを保存
func saveToken(path string, token *oauth2.Token) {
	data, _ := json.MarshalIndent(token, "", "  ") // 綺麗に整形して保存
	// 🌟 os.WriteFile で一撃保存（パーミッション 0600 もここで指定）
	if err := os.WriteFile(path, data, 0600); err != nil {
		fmt.Printf("⚠️ 保存失敗: %v\n", err)
	}
}

func (a *App) SyncMessages() error {
	if a.srv == nil {
		return fmt.Errorf("API未初期化")
	}

	// 1. 🌟 Q("newer_than:1d") で直近のメールだけに絞り、効率化
	// MaxResults は 20->50 くらいに増やしても、重複をスキップするので高速です
	res, err := a.srv.Users.Messages.List("me").Q("newer_than:1d").MaxResults(50).Do()
	if err != nil {
		return err
	}

	for _, m := range res.Messages {
		// 2. 🌟 「事前チェック」 🌟
		// すでに DB にあるメールなら、以降の重い処理（Get や AI学習）をスキップ！
		var exists int
		a.db.QueryRow("SELECT COUNT(*) FROM messages WHERE id = ?", m.Id).Scan(&exists)
		if exists > 0 {
			continue // 既に持っているので次のメールへ
		}

		// --- ここから先は「本当に新しいメール」だけが通れる聖域 ---
		msg, err := a.srv.Users.Messages.Get("me", m.Id).Format("metadata").Do()
		if err != nil {
			continue
		}

		isRead := 1
		for _, label := range msg.LabelIds {
			if label == "UNREAD" {
				isRead = 0
				break
			}
		}

		var sender, subject, to, cc string
		for _, h := range msg.Payload.Headers {
			if strings.EqualFold(h.Name, "From") {
				sender = h.Value
			}
			if strings.EqualFold(h.Name, "Subject") {
				subject = h.Value
			}
			if strings.EqualFold(h.Name, "To") {
				to = h.Value
			}
			if strings.EqualFold(h.Name, "Cc") {
				cc = h.Value
			}
		}
		combinedRecipient := to + " " + cc

		// 3. 🌟 INSERT OR IGNORE を活用
		_, err = a.db.Exec(`INSERT OR IGNORE INTO messages (id, sender, recipient, subject, snippet, timestamp, is_read) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			msg.Id, sender, combinedRecipient, subject, msg.Snippet, msg.InternalDate, isRead)
		if err != nil {
			continue
		}

		// 4. 🌟 新しいメールだけを Ollama に学習させる
		go func(id string, subject string, sender string, recipient string, snippet string) {
			if snippet != "" && subject == "" {
				return
			}
			// 🌟 情報の「盛り合わせ」を作る 🌟
			// 形式はAIが理解しやすい自然な形に
			combinedText := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\nSnippet: %s",
				sender, recipient, subject, snippet)
			limit := 4000
			if len(combinedText) > limit {
				combinedText = combinedText[:limit]
			}
			// (略: 強化ベクトル化ロジック)
			//combinedText := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\nSnippet: %s", sender, recipient, subject, snippet)
			err := a.SyncEmailVector(id, combinedText)
			if err != nil {
				fmt.Printf("強化ベクトル化失敗: %v\n", err)
			}

		}(m.Id, subject, sender, combinedRecipient, msg.Snippet)
	}
	return nil
}

func (a *App) GetChannels() ([]Channel, error) {
	rows, err := a.db.Query("SELECT name FROM channels")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Channel
	for rows.Next() {
		var c Channel
		rows.Scan(&c.Name)
		res = append(res, c)
	}
	return res, nil
}

func (a *App) GetMessagesByChannel(channelName string) ([]MessageSummary, error) {
	var condition string
	err := a.db.QueryRow("SELECT sql_condition FROM channels WHERE name = ?", channelName).Scan(&condition)
	if err != nil {
		condition = "1=1"
	}

	query := fmt.Sprintf("SELECT id, sender, recipient, subject, snippet, importance, deadline, timestamp, is_read FROM messages WHERE %s ORDER BY timestamp DESC", condition)
	rows, err := a.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MessageSummary
	for rows.Next() {
		var m MessageSummary
		var deadlineNull sql.NullString
		err := rows.Scan(&m.ID, &m.From, &m.Recipient, &m.Subject, &m.Snippet, &m.Importance, &deadlineNull, &m.Timestamp, &m.IsRead)
		if err != nil {
			fmt.Println("Scan Error: ", err)
			continue
		}

		if deadlineNull.Valid {
			m.Deadline = deadlineNull.String
		} else {
			m.Deadline = ""
		}
		results = append(results, m)
	}
	return results, nil
}

func (a *App) MarkAsRead(id string) error {
	if a.srv == nil {
		return nil
	}
	// ラベル変更リクエストの作成
	batch := &gmail.BatchModifyMessagesRequest{
		RemoveLabelIds: []string{"UNREAD"},
		Ids:            []string{id},
	}
	// Googleサーバーへ送信
	err := a.srv.Users.Messages.BatchModify("me", batch).Do()
	if err != nil {
		return err
	}

	_, err = a.db.Exec("UPDATE messages SET is_read = 1 WHERE id = ?", id)
	return err
}

func (a *App) GetMessageBody(id string) (string, error) {
	return a.GetMessageBodyWithContext(context.Background(), id)
}

func (a *App) GetMessageBodyWithContext(ctx context.Context, id string) (string, error) {
	// 1. まずは SQLite に本文が保存されていないか確認
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	var cachedBody string
	err := a.db.QueryRowContext(ctx, "SELECT body FROM messages WHERE id = ?", id).Scan(&cachedBody)

	// DBに本文（長さ1以上）があれば、それを即座に返す
	if err == nil && len(cachedBody) > 0 {
		fmt.Printf("Cache Hit! ID: %s (SQLiteから取得)\n", id)
		return cachedBody, nil
	}

	// 2. なければ Gmail API から取得
	fmt.Printf("Cache Miss! ID: %s (APIから取得中...)\n", id)
	msg, err := a.srv.Users.Messages.Get("me", id).Format("full").Do()
	if err != nil {
		return "", err
	}

	// gmail で既読に変更
	go func() {
		err := a.MarkAsRead(id)
		if err != nil {
			fmt.Printf("既読同期失敗: %v\n", err)
		}
	}()

	body := a.extractBody(msg.Payload)

	// 3. 次回のために SQLite に保存（キャッシュ）しておく
	go func() {
		_, err = a.db.Exec("UPDATE messages SET body = ? WHERE id = ?", body, id)
		if err != nil {
			fmt.Printf("キャッシュ保存エラー: %v\n", err)
		}
	}()

	var subject, sender string
	a.db.QueryRow("SELECT subject, sender FROM messages WHERE id = ?", id).Scan(&subject, &sender)

	// 🌟 これらを全部混ぜて「完全版ベクトル」にする 🌟
	fullText := fmt.Sprintf("From: %s\nSubject: %s\nBody: %s", sender, subject, body)
	limit := 4000
	if len(fullText) > limit {
		fullText = fullText[:limit]
	}

	go func(msgID string, text string) {
		if text != "" {
			// スニペット版をこの「完全版」で上書き！
			err := a.SyncEmailVector(msgID, text)
			if err != nil {
				fmt.Printf("完全版AI学習失敗: %v\n", err)
			}
		}
	}(id, fullText)

	go func(msgID string, content string) {
		if content != "" {
			fmt.Printf("🤖 Ollama 締め切り抽出開始: %s\n", msgID)
			err := a.ExtractDeadlines(msgID)
			if err != nil {
				fmt.Printf("Ollama 締め切り抽出失敗: %v\n", err)
			} else {
				fmt.Printf("✅ Ollama 締め切り抽出完了: %s\n", msgID)
				// runtime.EventsEmit(a.ctx, "summary_ready", msgID)
			}
		}
	}(id, body)

	return body, nil
}

// フロントエンドから呼ばれる関数
func (a *App) OpenExternalLink(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// extractBody の最後、return する直前で加工
func (a *App) injectExternalLinkScript(htmlContent string) string {
	// injectExternalLinkScript 内のスクリプト
	script := `
<script>
    document.addEventListener('click', function(e) {
        var a = e.target.closest('a');
        if (a && a.href && a.href.startsWith('http')) {
            e.preventDefault();
            // 親ウィンドウ（React側）に「このURL開いて！」と叫ぶ
            window.parent.postMessage({type: 'open_url', url: a.href}, '*');
        }
    }, true);
</script>`

	return htmlContent + script
}

func (a *App) extractBody(part *gmail.MessagePart) string {
	body := a.findPart(part, "text/html", a.findPart(part, "text/plain", ""))
	body = a.injectExternalLinkScript(body)
	return body
}

// 特定の MimeType を優先的に探す補助関数
func (a *App) findPart(part *gmail.MessagePart, targetType string, fallback string) string {
	if part.MimeType == targetType && part.Body.Data != "" {
		data, _ := base64.URLEncoding.DecodeString(part.Body.Data)
		if targetType == "text/plain" {
			// テキストなら HTML っぽく包んでから返す
			content := html.EscapeString(string(data))
			return "<pre style='white-space: pre-wrap; font-family: sans-serif; font-size: 14px;'>" + content + "</pre>"
		}
		return string(data)
	}

	for _, subPart := range part.Parts {
		if body := a.findPart(subPart, targetType, ""); body != "" {
			return body
		}
	}

	return fallback
}

func (a *App) SyncHistoricalMessages(pageToken string) (string, error) {
	if a.srv == nil {
		return "", fmt.Errorf("SyncHistoricalMessage: API未初期化")
	}

	// 1. 最新500件を取得（pageTokenがあれば続きから）
	req := a.srv.Users.Messages.List("me").MaxResults(500)
	if pageToken != "" {
		req.PageToken(pageToken)
	}
	res, err := req.Do()
	if err != nil {
		return "", err
	}

	// 2. 500通をループして保存・更新
	for _, m := range res.Messages {
		// metadata形式で「ラベル情報」も含めて取得
		msg, err := a.srv.Users.Messages.Get("me", m.Id).Format("metadata").Do()
		if err != nil {
			continue
		}

		// 既読判定（UNREADラベルがあるか）
		isRead := 1
		for _, label := range msg.LabelIds {
			if label == "UNREAD" {
				isRead = 0
				break
			}
		}

		// ヘッダー解析（差出人・件名）
		var sender, subject, to, cc string
		for _, h := range msg.Payload.Headers {
			if strings.EqualFold(h.Name, "From") {
				sender = h.Value
			}
			if strings.EqualFold(h.Name, "Subject") {
				subject = h.Value
			}
			if strings.EqualFold(h.Name, "To") {
				to = h.Value
			}
			if strings.EqualFold(h.Name, "Cc") {
				cc = h.Value
			}
		}
		combinedRecipient := to + " " + cc

		// 【重要】INSERT OR REPLACE で、既読状態も最新に更新
		_, err = a.db.Exec(`
			INSERT OR REPLACE INTO messages (id, sender, recipient, subject, snippet, timestamp, is_read) 
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			msg.Id, sender, combinedRecipient, subject, msg.Snippet, msg.InternalDate, isRead)

		go func(id string, subject string, sender string, recipient string, snippet string) {
			if snippet != "" && subject == "" {
				return
			}
			combinedText := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\nSnippet: %s",
				sender, recipient, subject, snippet)
			limit := 4000
			if len(combinedText) > limit {
				combinedText = combinedText[:limit]
			}

			// これをベクトル化
			err := a.SyncEmailVector(id, combinedText)
			if err != nil {
				fmt.Printf("強化ベクトル化失敗: %v\n", err)
			}

		}(m.Id, subject, sender, combinedRecipient, msg.Snippet)
	}

	// 次のページの合言葉を返す
	return res.NextPageToken, nil
}

// AISearch は「あいまい検索」を実行して、スコアの高い順に ID を返します
func (a *App) AISearch(query string) ([]SearchResult, error) {
	// 1. 検索クエリをベクトル化
	req := &api.EmbeddingRequest{
		Model:  globalConfig.EmbedModel,
		Prompt: query,
	}
	resp, err := a.ollama.Embeddings(context.Background(), req)
	if err != nil {
		return nil, err
	}
	queryVec := resp.Embedding

	// 2. DBから全データを取得
	rows, err := a.db.Query("SELECT id, vector FROM email_vectors")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allResults []SearchResult
	for rows.Next() {
		var id string
		var vecBytes []byte
		rows.Scan(&id, &vecBytes)

		var dbVec []float32
		if err := json.Unmarshal(vecBytes, &dbVec); err != nil {
			continue
		}

		// 3. 類似度（ドット積）の計算
		var score float32
		for i := 0; i < len(queryVec); i++ {
			score += float32(queryVec[i]) * float32(dbVec[i])
		}
		allResults = append(allResults, SearchResult{ID: id, Score: score})
	}

	// 4. スコアが高い順（降順）にソート
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	// 上位10件程度を返す（Wailsのフロントエンドへ）
	if len(allResults) > 10 {
		return allResults[:10], nil
	}
	return allResults, nil
}

// GetAISearchResults は AI 検索の結果を元に、メッセージ詳細のリストを返します
func (a *App) GetAISearchResults(query string) ([]MessageSummary, error) {
	// 1. まずは既存の AISearch ロジックで ID とスコアを取得
	// (先ほど作った AISearch 関数を流用するか、そのロジックをここに書く)
	searchResults, err := a.AISearch(query)
	if err != nil {
		return nil, err
	}

	// 2. ID だけの配列を作る
	var ids []string
	for _, res := range searchResults {
		ids = append(ids, res.ID)
	}

	// 3. DB から詳細情報を取得（a.store は db.go で作った Store）
	msgs, err := a.store.GetMessagesByIDs(ids)
	if err != nil {
		return nil, err
	}

	//fmt.Printf("msgs: %s\n", msgs)
	return msgs, nil
}

func (a *App) SummarizeEmail(id string) (string, error) {
	// 1. キャッシュチェック
	var cached string

	a.db.QueryRow("SELECT summary FROM messages WHERE id = ?", id).Scan(&cached)
	if len(cached) > 0 {
		return cached, nil
	}

	// 2. 本文取得
	var body string
	a.db.QueryRow("SELECT body FROM messages WHERE id = ?", id).Scan(&body)
	if len(body) == 0 {
		return "本文がありません", nil
	}

	// 3. Ollama 呼び出し
	prompt1 := fmt.Sprintf(`
あなたは多忙なビジネスマン専用の要約エージェントです。
以下のルールを厳守し、メールを要約してください。

- 内容を【3行以内】の箇条書きに要約すること。
- 挨拶や「以下が要約です」という説明は一切不要。
- 本文をそのままコピーせず、要点のみを再構成すること。
- 日本語で出力すること。

メール内容: %s`, body)

	req := &api.GenerateRequest{
		Model:  globalConfig.OllamaModel,
		Prompt: prompt1,
		Stream: new(bool), // false
	}

	var summary string
	err := a.ollama.Generate(a.ctx, req, func(resp api.GenerateResponse) error {
		summary = resp.Response
		return nil
	})
	if err != nil {
		return "", err
	}
	// --- 🔴 無粋なタグを掃除する 🔴 ---
	summary = strings.ReplaceAll(summary, "</start_of_turn>", "")
	summary = strings.ReplaceAll(summary, "</end_of_turn>", "")
	summary = strings.TrimSpace(summary) // 前後の余計な改行も消す
	// ------------------------------
	// 4. SQLite にキャッシュ
	a.db.Exec("UPDATE messages SET summary = ?  WHERE id = ?", summary, id)

	return summary, nil
}

func cleanForAI(htmlStr string) string {
	// 1. <script>タグを削除
	reScript := regexp.MustCompile(`(?s)<script.*?>.*?</script>`)
	htmlStr = reScript.ReplaceAllString(htmlStr, "")

	// 2. HTMLタグをすべて削除して純粋なテキストのみにする
	reTag := regexp.MustCompile(`(?s)<.*?>`)
	text := reTag.ReplaceAllString(htmlStr, " ")

	// 3. 署名(Signature)と思われる「-- 」以降をバッサリ切る（解説を防ぐコツ）
	if idx := strings.Index(text, "-- "); idx != -1 {
		text = text[:idx]
	}

	// 4. 空白と改行を整理して1000文字程度に制限（重要度の判定には十分）
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 1000 {
		text = text[:1000]
	}
	return text
}

func (a *App) ExtractDeadlines(id string) error {
	var body string

	var err error
	for i := 0; i < 5; i++ {
		err = a.db.QueryRow("SELECT body FROM messages WHERE id = ?", id).Scan(&body)
		if err == nil {
			break // 成功！
		}
		fmt.Printf("⏳ ExtractDeadlines(SELECT): ロック中、待機します... (%d/5)\n", i+1)
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil || len(body) == 0 {
		fmt.Printf("❌ DBからの本文取得に失敗しました: %v\n", err)
		return nil
	}

	cleanBody := cleanForAI(body)

	/*
			prompt := fmt.Sprintf(`
		あなたは世界一多忙なCEOの冷徹な秘書です。
		以下のメールを解析し、2つの情報を【極めて厳しく】抽出してください。

		1. 【重要度】: 1(不要)から5(至急)の数値
		   - 5: あなたが今すぐ返信しないと会社が潰れるレベルの緊急案件
		   - 3: 本人への確認が必要な、通常の業務連絡
		   - 1: 広告、メルマガ、自動通知、挨拶、後回しで良い報告
		   ※ 迷ったら「1」にしてください。

		2. 【期限】: 最も重要な未来の日付(YYYY-MM-DD)。なければ「なし」

		今日は %s です。
		結果のみを「重要度:数値, 期限:日付」の形式で答えてください。説明は一切不要。

		メール内容: %s`, time.Now().Format("2006-01-02"), cleanBody)
	*/

	prompt := fmt.Sprintf(`
あなたは世界一多忙なCEOの【冷徹な】秘書です。
以下のメールを解析し、2つの情報を【極めて厳しく】抽出してください。

		1. 【重要度】: 1(不要)から5(至急)の数値
		   - 5: あなたが今すぐ返信しないと会社が潰れるレベルの緊急案件
		   - 3: 本人への確認が必要な、通常の業務連絡
		   - 1: 広告、メルマガ、自動通知、挨拶、後回しで良い報告
		   ※ 迷ったら「1」にしてください。

		2. 【期限】: 最も重要な未来の日付(YYYY-MM-DD)。なければ「なし」
ルール：
- 箇条書きや複数の回答は【厳禁】。
- 最も重要な【1組の情報】のみを、一文で出力せよ。
- 挨拶、解説、番号付けは一切不要。

形式：重要度:数値, 期限:YYYY-MM-DD
今日は %s です。

メール内容: %s`, time.Now().Format("2006-01-02"), cleanBody)

	req := &api.GenerateRequest{
		Model:  globalConfig.OllamaModel,
		Prompt: prompt,
		Stream: new(bool),
	}

	var respText string
	err = a.ollama.Generate(a.ctx, req, func(resp api.GenerateResponse) error {
		respText += resp.Response
		return nil
	})
	if err != nil {
		fmt.Printf("Error in ExtractDetadlines: %s\n", err)
		return err
	}

	fmt.Printf("📅 respText を検出: %s (ID: %s)\n", respText, id)

	reImp := regexp.MustCompile(`重要度:?\s*(\d)`)
	impMatch := reImp.FindStringSubmatch(respText)
	importance := 1
	if len(impMatch) > 1 {
		importance, _ = strconv.Atoi(impMatch[1])
	}

	// 最初に見つかった「YYYY-MM-DD」を抽出
	reDate := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	deadline := reDate.FindString(respText) // FindStringは最初のマッチを返す

	if deadline != "" {
		fmt.Printf("📅 期限を検出: %s (ID: %s)\n", deadline, id)
	}

	if deadline != "" && deadline != "なし" {
		// 🌟 現代的な日付バリデーション 🌟
		parsedDate, err := time.Parse("2006-01-02", deadline)
		today, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))

		// 今日より前の日付（過去）なら、AIの幻覚として切り捨てる
		if err != nil || parsedDate.Before(today) {
			fmt.Printf("🚫 過去または無効な日付を拒否: %s\n", deadline)
			deadline = "" // 「なし」として扱う
		}
	}

	// DB更新
	//	a.db.Exec("UPDATE messages SET importance = ?, deadline = ? WHERE id = ?", importance, deadline, id)
	for i := 0; i < 3; i++ {
		_, err = a.db.Exec("UPDATE messages SET importance = ?, deadline = ? WHERE id = ?", importance, deadline, id)
		if err == nil {
			return nil // 成功！
		}
		// ロックされていたら少し待つ
		fmt.Printf("⏳ ExtractDeadlines: DBロック中、リトライします... (%d/3)\n", i+1)
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func (a *App) SetManualImportance(id string, level int) error {
	// 🌟 AIの判定を人間が「上書き」する
	_, err := a.db.Exec("UPDATE messages SET importance = ? WHERE id = ?", level, id)
	return err
}

func (a *App) TrashMessage(id string) error {
	if a.srv == nil {
		return fmt.Errorf("Gmail APIが初期化されていません")
	}

	// 1. Googleサーバー上のメールをゴミ箱(TRASH)へ移動
	// DeleteではなくTrashを使うのが「安全装置」としてのプロの選択
	_, err := a.srv.Users.Messages.Trash("me", id).Do()
	if err != nil {
		return fmt.Errorf("Gmailサーバーでのゴミ箱移動に失敗: %v", err)
	}

	// 2. サーバー側が成功した時のみ、ローカルの SQLite からも削除
	// これにより DB とサーバーの不整合を防ぐ (ストラ氏が喜ぶ整合性)
	_, err = a.db.Exec("DELETE FROM messages WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ローカルDBの更新に失敗: %v", err)
	}

	fmt.Printf("🗑️ ゴミ箱へ移動完了: %s\n", id)
	return nil
}

func (a *App) RunAutoCleanup() {
	// 🌟 すでにお掃除中なら二重に走らせないガード
	if a.isCleaning {
		return
	}
	a.isCleaning = true
	defer func() { a.isCleaning = false }()

	fmt.Println("🧹 お掃除作戦（低速・安定モード）を開始します...")

	rows, err := a.db.Query("SELECT name, sql_condition, ttl_days FROM channels WHERE ttl_days > 0")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name, condition string
		var ttl int
		rows.Scan(&name, &condition, &ttl)

		// 1. IDリストをサッと取得（DBを掴む時間を最小限に）
		var ids []string
		selectQuery := fmt.Sprintf(
			"SELECT id FROM messages WHERE (%s) AND timestamp < (unixepoch('now', '-%d days') * 1000)",
			condition, ttl,
		)
		targetRows, _ := a.db.Query(selectQuery)
		for targetRows.Next() {
			var id string
			targetRows.Scan(&id)
			ids = append(ids, id)
		}
		targetRows.Close()

		// 2. 🌟 本領発揮：1通ずつゆっくり、休み休み掃除する 🌟
		for _, id := range ids {
			// Gmailサーバーのゴミ箱へ
			_, err := a.srv.Users.Messages.Trash("me", id).Do()
			if err == nil {
				// 成功した時だけ、一瞬だけDBを開いて削除
				a.db.Exec("DELETE FROM messages WHERE id = ?", id)
				fmt.Printf("✨ [%s] 整理完了: %s\n", name, id)
			}

			// 🌟 500ミリ秒（0.5秒）の休憩。
			// これにより、ベクトル化やUIの描画が割り込む隙間を作ります。
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// 🌟 デスクトップ・Web共通の「生のJSON読み込み」
func (a *App) GetChannelsRaw() (string, error) {
	data, err := os.ReadFile(channelsFile)
	if err != nil {
		return "", err
	}
	return string(data), err
}

// 🌟 デスクトップ・Web共通の「保存」
func (a *App) SaveChannelsRaw(jsonText string) error {
	// バリデーション (JSONとして正しいか)
	var temp []interface{}
	if err := json.Unmarshal([]byte(jsonText), &temp); err != nil {
		return fmt.Errorf("JSON形式が不正です: %w", err)
	}
	// 保存
	err := os.WriteFile(channelsFile, []byte(jsonText), 0644)
	if err == nil {
		a.LoadChannelsFromJson() // DB反映
	}
	return err
}

// 🌟 デスクトップ・Web共通の「生のJSON読み込み」
func (a *App) GetSettingsRaw() (string, error) {
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return "", err
	}
	return string(data), err
}

func (a *App) SaveSettingsRaw(jsonText string) error {
	var temp Config
	if err := json.Unmarshal([]byte(jsonText), &temp); err != nil {
		return fmt.Errorf("JSON形式が不正です: %w", err)
	}

	if err := os.WriteFile(settingsFile, []byte(jsonText), 0644); err != nil {
		return err
	}

	// 🌟 サーバー側の設定値(globalConfig)も即座に同期
	globalConfig = temp
	fmt.Println("⚙️ settings.json を更新し、メモリに反映しました")
	return nil
}
