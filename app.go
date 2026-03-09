package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	_ "modernc.org/sqlite"

	"github.com/ollama/ollama/api"
)

type MessageSummary struct {
	ID               string `json:"id"`
	ThreadID         string `json:"thread_id"`
	MessageID        string `json:"message_id"`
	ReferencesIDs    string `json:"references_ids"`
	From             string `json:"from"`
	Recipient        string `json:"recipient"`
	Subject          string `json:"subject"`
	Snippet          string `json:"snippet"`
	IsRead           int64  `json:"is_read"`
	Importance       int64  `json:"importance"`
	ManualImportance int64  `json:"manual_importance"`
	//Date       string `json:"date"`
	Timestamp int64  `json:"timestamp"`
	Deadline  string `json:"deadline"`
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
	// a.LoadChannelsFromJson()
	a.LoadChannelConfigs()

	// テーブル作成
	a.db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		thread_id TEXT,
		message_id TEXT,
		references_ids TEXT,
		sender TEXT,
		recipient TEXT,
		all_involved_adresses TEXT,
		subject TEXT,
		snippet TEXT,
		timestamp INTEGER,
		body TEXT,
		summary TEXT,
		is_read INTEGER DEFAULT 0,
		importance INTEGER DEFAULT 0,
		manual_importance INTEGER DEFAULT 0,
		deadline DATETIME
	);`)

	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_deadline ON messages(deadline);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_is_read ON messages(is_read);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_thread_id ON messages(thread_id);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_message_id ON messages(message_id);")
	a.db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_importance ON messages(importance);")
	fmt.Println("✅ インデックスの作成/確認が完了しました")

	s, err := NewStore(a.db)
	if err != nil {
		panic(err)
	}
	a.store = s
	return nil
}

func (a *App) initAI() error {
	// URL構文チェックだけは必ず行う
	u, err := url.Parse(globalConfig.OllamaHost)
	if err != nil {
		return fmt.Errorf("OllamaHostの形式が不正です: %w", err)
	}

	client := api.NewClient(u, http.DefaultClient)

	// 簡易ヘルスチェック: 小さなEmbeddingリクエストを送信して接続確認
	// 接続失敗時は a.ollama を nil にして AI 機能を無効化する
	if _, err := client.Embeddings(context.Background(), &api.EmbeddingRequest{
		Model:  globalConfig.EmbedModel,
		Prompt: "ping",
	}); err != nil {
		fmt.Printf("🤖 AI 接続失敗: %v. AI機能は無効化されます\n", err)
		a.ollama = nil
		return nil
	}

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
	// 🌟 1. お掃除タスク（1時間ごと）
	go func() {
		time.Sleep(3 * time.Minute)
		for {
			fmt.Println("🧹 バックグラウンドお掃除を開始...")
			a.executeAllCleanUp()
			time.Sleep(1 * time.Hour)
		}
	}()

	// 🌟 2. 同期タスク（設定値ごと）
	go func() {
		// globalConfig が読み込まれるのを待つ安全策
		time.Sleep(10 * time.Second)
		for {
			interval := time.Duration(globalConfig.SyncInterval) * time.Second
			if interval <= 0 {
				interval = 10 * time.Minute
			} // 安全装置

			fmt.Println("📡 Gmail 同期を開始...")
			a.SyncMessages()

			time.Sleep(interval)
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
