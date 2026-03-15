package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ChannelRules: メールの抽出条件（素材）を定義

type ChannelRules struct {
	Domains       []string `json:"domains"`        // ["@company.com", "@partner.jp"]
	Keywords      []string `json:"keywords"`       // ["見積", "重要"]
	ImportanceMin int      `json:"importance_min"` // 4以上（重要）など
	TTLdays       int      `json:"ttl_days"`
	IsUnreadOnly  bool     `json:"is_unread_only"`
}

// ChannelConfig: 1つのワークスペース（または固定チャンネル）の定義
type ChannelConfig struct {
	ID    string       `json:"id"`    // internal, sales, priority など
	Name  string       `json:"name"`  // 🏢 社内, 💼 営業 など
	Type  string       `json:"type"`  // "auto_group" (人別に展開) か "fixed" (そのまま表示)
	Rules ChannelRules `json:"rules"` // 抽出ルール
}

// 🌟 Reactへ返すための「階層型」データ構造
type WorkspaceFolder struct {
	Id        string       `json:"id"`
	GroupName string       `json:"group_name"` // "🏢 社内" など
	Type      string       `json:"type"`       // auto_group , fixed
	Channels  []string     `json:"channels"`   // ["田中太郎 <tanaka@...>", "佐藤次郎 <sato@...>"] など
	Rules     ChannelRules `json:"rules"`      // 抽出ルール
}

func (a *App) LoadChannelConfigs() ([]ChannelConfig, error) {
	if _, err := os.Stat(channelsFile); os.IsNotExist(err) {
		// target が存在しない場合、example があるか確認
		if data, err := os.ReadFile(channelsExample); err == nil {
			// example の中身を target に書き込む（＝コピー）
			os.WriteFile(channelsFile, data, 0644)
			fmt.Println("📝 example から設定ファイルを作成しました")
		} else {
			// example もない場合は、「最低限のデフォルト」を作成
			defaultChannels := `[
				{"id": "institutes001", "name": "🏢 gmail", "type": "auto_group", "rules": { "domains": ["@%gmail.com"], "keywords": [], "importance_min": 0, "ttl_days" : 0}},
				{"id": "institutes002", "name": "🏢 outlook", "type": "auto_group", "rules": { "domains": ["@%outlook.com", "@hotmail.com"], "keywords": [], "importance_min": 0,"ttl_days" : 0 } },
			]`
			os.WriteFile(channelsFile, []byte(defaultChannels), 0644)
			fmt.Println("⚠️ デフォルト設定を作成しました")
		}
	}

	// 🌟 1. 前に作った定数 channelsFile ("config/channels.json") を読み込む
	data, err := os.ReadFile(channelsFile)
	if err != nil {
		return nil, fmt.Errorf("設定ファイル読み込み失敗: %w", err)
	}

	// 🌟 2. JSON を構造体の配列にパースする
	var configs []ChannelConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("JSON解析失敗: %w", err)
	}

	// 🌟 3. もし中身が空なら、「空配列」を返す
	if configs == nil {
		configs = []ChannelConfig{}
	}

	return configs, nil
}

func (a *App) GetWorkspaceList() ([]WorkspaceFolder, error) {
	// 1. channels.json を読み込む (ChannelConfig の配列)
	configs, _ := a.LoadChannelConfigs()

	rows, _ := a.db.Query(`
        SELECT SUBSTR(sender, INSTR(sender, '@')) as domain 
        FROM messages 
        GROUP BY domain 
        ORDER BY COUNT(*) DESC LIMIT 3`)

	i := 1
	for rows.Next() {
		var d string
		rows.Scan(&d)
		autoID := fmt.Sprintf("recommend%03d", i)

		configs = append(configs, ChannelConfig{
			ID:    autoID,
			Name:  "✨ 推奨: " + d,
			Type:  "recommend",
			Rules: ChannelRules{Domains: []string{d}},
		})
		i++
	}

	defaultWorkspaces := []ChannelConfig{
		{
			ID:    "inbox",
			Name:  "📥 全てのメール",
			Type:  "default",
			Rules: ChannelRules{TTLdays: 0}, // 抽出条件は GetMessagesByChannel 側で判定
		},
		{
			ID:    "unread",
			Name:  "📧 未読のみ",
			Type:  "default",
			Rules: ChannelRules{IsUnreadOnly: true, TTLdays: 0},
		},
		{
			ID:    "priority",
			Name:  "🔥 最優先（手動設定）",
			Type:  "default",
			Rules: ChannelRules{ImportanceMin: 4, TTLdays: 0},
		},
	}

	allConfigs := append(defaultWorkspaces, configs...)

	var result []WorkspaceFolder

	for _, conf := range allConfigs {
		// 2. 各ルールの「中身（人）」を GetDynamicChannels に聞きに行く
		senders, _ := a.GetDynamicChannels(conf.Rules)

		// 3. Reactが喜ぶ「グループ名」と「人リスト」のペアを作る
		folder := WorkspaceFolder{
			Id:        conf.ID,
			GroupName: conf.Name,
			Type:      conf.Type,
			Channels:  senders,
			Rules:     conf.Rules,
		}
		result = append(result, folder)
	}

	return result, nil
}

// GetDynamicChannels: ルールに基づいて、合致する「送信者(sender)」を重複なく抽出する
func (a *App) GetDynamicChannels(rules ChannelRules) ([]string, error) {
	var conditions []string
	var args []interface{}

	// 🌟 1. ドメイン条件 (@company.com など)
	if len(rules.Domains) > 0 {
		var domainParts []string
		for _, d := range rules.Domains {
			domainParts = append(domainParts, "sender LIKE ?")
			args = append(args, "%"+d+"%")
		}
		// ドメイン同士は OR で繋ぐ
		conditions = append(conditions, "("+strings.Join(domainParts, " OR ")+")")
	}

	// 🌟 2. キーワード条件 (件名やスニペット)
	if len(rules.Keywords) > 0 {
		var kwParts []string
		for _, k := range rules.Keywords {
			kwParts = append(kwParts, "(subject LIKE ? OR snippet LIKE ?)")
			args = append(args, "%"+k+"%", "%"+k+"%")
		}
		// キーワード同士も OR で繋ぐ
		conditions = append(conditions, "("+strings.Join(kwParts, " OR ")+")")
	}

	// 🌟 3. 重要度条件 (4以上など)
	if rules.ImportanceMin > 0 {
		conditions = append(conditions, "manual_importance >= ?")
		args = append(args, rules.ImportanceMin)
	}

	// 🌟 4. 全件マッチの安全策 (条件が何もない場合)
	whereClause := "1=1"
	if len(conditions) > 0 {
		// 各カテゴリ（ドメイン、キーワード、重要度）は AND で結ぶのが現代的
		whereClause = strings.Join(conditions, " AND ")
	}

	// 🌟 5. SQL実行：DISTINCT で重複を除去！
	query := fmt.Sprintf(
		"SELECT DISTINCT sender FROM messages WHERE %s ORDER BY sender ASC",
		whereClause,
	)

	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("クエリ失敗: %w", err)
	}
	defer rows.Close()

	var senders []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			senders = append(senders, s)
		}
	}

	if senders == nil {
		senders = []string{}
	}
	return senders, nil
}

// ReloadAndGetWorkspaces: Wailsから呼ばれる「再読み込み ＋ 一覧生成」の合体技
func (a *App) ReloadAndGetWorkspaces() ([]WorkspaceFolder, error) {
	// 1. 設定ファイルを再ロード（内部変数を更新）
	_, err := a.LoadChannelConfigs()
	if err != nil {
		return nil, err
	}
	// 2. 最新の設定に基づき、Slack風の階層リストを生成して返す
	return a.GetWorkspaceList()
}
