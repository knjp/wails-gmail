package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"google.golang.org/api/gmail/v1"
)

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

func (a *App) GetMessagesByChannel(channelID string) ([]MessageSummary, error) {
	workspaces, _ := a.GetWorkspaceList()
	for _, ws := range workspaces {
		if ws.Id == channelID {
			return a.GetMessagesByRules(ws.Rules)
		}
	}

	var condition string
	var args []interface{}

	if strings.Contains(channelID, "@") {
		condition = "sender = ?"
		args = append(args, channelID)
	} else {
		// どれにも該当しない場合は空リスト
		return []MessageSummary{}, nil
	}

	//query := fmt.Sprintf("SELECT id, thread_id, message_id, references_ids, sender, recipient, subject, snippet, importance, deadline, timestamp, is_read FROM messages WHERE %s ORDER BY timestamp DESC", condition)

	query := fmt.Sprintf("SELECT %s FROM messages WHERE %s ORDER BY timestamp DESC", MessageSelectFields, condition)
	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MessageSummary
	for rows.Next() {
		/*
			var m MessageSummary
			var deadlineNull sql.NullString
			err := rows.Scan(&m.ID, &m.ThreadID, &m.MessageID, &m.ReferencesIDs, &m.From, &m.Recipient, &m.Subject, &m.Snippet, &m.Importance, &deadlineNull, &m.Timestamp, &m.IsRead)
			if err != nil {
				fmt.Println("Scan Error: ", err)
				continue
			}

			if deadlineNull.Valid {
				m.Deadline = deadlineNull.String
			} else {
				m.Deadline = ""
			}
		*/
		if m, err := a.scanMessageSummary(rows); err == nil {
			results = append(results, m)
		}
	}
	return results, nil
}

// GetMessagesByRules: ルールに合致するメッセージをすべて取得する
func (a *App) GetMessagesByRules(rules ChannelRules) ([]MessageSummary, error) {
	whereClause, args := a.BuildWhereClause(rules)

	/*
		query := fmt.Sprintf(
			"SELECT id, thread_id, message_id, references_ids, sender, recipient, subject, snippet, importance, deadline, timestamp, is_read FROM messages WHERE %s ORDER BY timestamp DESC LIMIT 100",
			whereClause,
		)
	*/
	query := fmt.Sprintf("SELECT %s FROM messages WHERE %s ORDER BY timestamp DESC LIMIT 100", MessageSelectFields, whereClause)

	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ルールクエリ失敗: %w", err)
	}
	defer rows.Close()

	var msgs []MessageSummary
	for rows.Next() {
		/*
			var m MessageSummary
			var deadlineNull sql.NullString
			err := rows.Scan(&m.ID, &m.ThreadID, &m.MessageID, &m.ReferencesIDs, &m.From, &m.Recipient, &m.Subject, &m.Snippet, &m.Importance, &deadlineNull, &m.Timestamp, &m.IsRead)
			if err == nil {
		*/
		if m, err := a.scanMessageSummary(rows); err == nil {
			msgs = append(msgs, m)
		}
	}

	if msgs == nil {
		msgs = []MessageSummary{}
	}
	return msgs, nil
}

// BuildWhereClause: ルールから SQL の WHERE 句と引数リストを生成する共通部品
func (a *App) BuildWhereClause(rules ChannelRules) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	// 1. ドメイン条件
	if len(rules.Domains) > 0 {
		var domainParts []string
		for _, d := range rules.Domains {
			domainParts = append(domainParts, "sender LIKE ?")
			args = append(args, "%"+d+"%")
		}
		conditions = append(conditions, "("+strings.Join(domainParts, " OR ")+")")
	}

	// 2. キーワード条件
	if len(rules.Keywords) > 0 {
		var kwParts []string
		for _, k := range rules.Keywords {
			kwParts = append(kwParts, "(subject LIKE ? OR snippet LIKE ?)")
			args = append(args, "%"+k+"%", "%"+k+"%")
		}
		conditions = append(conditions, "("+strings.Join(kwParts, " OR ")+")")
	}

	// 3. 重要度条件
	if rules.ImportanceMin > 0 {
		conditions = append(conditions, "manual_importance >= ?")
		args = append(args, rules.ImportanceMin)
	}

	if rules.IsUnreadOnly {
		conditions = append(conditions, "is_read = 0")
	}

	// 条件がない場合は全件マッチ
	whereClause := "1=1"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}

	return whereClause, args
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

		var sender, subject, to, cc, msgID string
		involvedMap := make(map[string]bool)
		refIDMap := make(map[string]bool)

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
			if strings.EqualFold(h.Name, "From") || strings.EqualFold(h.Name, "To") || strings.EqualFold(h.Name, "Cc") {
				for _, addr := range strings.Fields(h.Value) {
					involvedMap[addr] = true
				}
			}
			if strings.EqualFold(h.Name, "Message-ID") {
				msgID = h.Value
			}
			if strings.EqualFold(h.Name, "References") || strings.EqualFold(h.Name, "In-Reply-To") {
				for _, rid := range strings.Fields(h.Value) {
					refIDMap[rid] = true
				}
			}

		}
		combinedRecipient := to + " " + cc

		delete(refIDMap, msgID)
		var involvedList, refList []string
		for k := range involvedMap {
			involvedList = append(involvedList, k)
		}
		for k := range refIDMap {
			refList = append(refList, k)
		}
		allInvolved := strings.Join(involvedList, " ")
		references := strings.Join(refList, " ")

		// 3. 🌟 INSERT OR IGNORE を活用
		_, err = a.db.Exec(`INSERT OR IGNORE INTO messages (id, thread_id, sender, recipient, all_involved_adresses, message_id, references_ids, subject, snippet, timestamp, is_read) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			msg.Id, msg.ThreadId, sender, combinedRecipient, allInvolved, msgID, references, subject, msg.Snippet, msg.InternalDate, isRead)
		if err != nil {
			fmt.Printf("SyncMessages: error %s\n", err)
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
		var sender, subject, to, cc, msgID string
		involvedMap := make(map[string]bool)
		refIDMap := make(map[string]bool)
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
			if strings.EqualFold(h.Name, "Message-ID") {
				msgID = h.Value
			}
			if strings.EqualFold(h.Name, "From") || strings.EqualFold(h.Name, "To") || strings.EqualFold(h.Name, "Cc") {
				for _, addr := range strings.Fields(h.Value) {
					involvedMap[addr] = true
				}
			}

			if strings.EqualFold(h.Name, "References") || strings.EqualFold(h.Name, "In-Reply-To") {
				for _, rid := range strings.Fields(h.Value) {
					refIDMap[rid] = true
				}
			}
		}
		combinedRecipient := to + " " + cc
		delete(refIDMap, msgID)

		// 🌟 文字列へ結合
		var involvedList, refList []string
		for k := range involvedMap {
			involvedList = append(involvedList, k)
		}
		for k := range refIDMap {
			refList = append(refList, k)
		}

		allInvolved := strings.Join(involvedList, " ")
		references := strings.Join(refList, " ")

		// 【重要】INSERT OR REPLACE で、既読状態も最新に更新
		_, err = a.db.Exec(`
			INSERT OR REPLACE INTO messages (id, thread_id, sender, recipient, all_involved_adresses, message_id, references_ids, subject, snippet, timestamp, is_read) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			msg.Id, msg.ThreadId, sender, combinedRecipient, allInvolved, msgID, references, subject, msg.Snippet, msg.InternalDate, isRead)
		if err != nil {
			fmt.Printf("SyncHistoricalMessages: error %s\n", err)
			continue
		}

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
	_, err := a.srv.Users.Messages.Trash("me", id).Do()
	if err != nil {
		return fmt.Errorf("Gmailサーバーでのゴミ箱移動に失敗: %v", err)
	}

	// 2. サーバー側が成功した時のみ、ローカルの SQLite からも削除
	_, err = a.db.Exec("DELETE FROM messages WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ローカルDBの更新に失敗: %v", err)
	}

	fmt.Printf("🗑️ ゴミ箱へ移動完了: %s\n", id)
	return nil
}

func (a *App) executeAllCleanUp() {
	configs, err := a.LoadChannelConfigs()
	if err != nil {
		return
	}

	for _, conf := range configs {
		if err := a.CleanUpByRule(conf.Rules); err != nil {
			fmt.Printf("⚠️ お掃除エラー (%s): %v\n", conf.Name, err)
		}
	}
}

func (a *App) CleanUpByRule(rules ChannelRules) error {
	if rules.TTLdays <= 0 {
		return nil
	}

	// 🌟 1. まず、削除対象の ID リストを DB から取得する
	whereClause, args := a.BuildWhereClause(rules)
	selectQuery := fmt.Sprintf(
		"SELECT id FROM messages WHERE (%s) AND (timestamp / 1000) < unixepoch('now', '-%d day')",
		whereClause, rules.TTLdays,
	)

	rows, err := a.db.Query(selectQuery, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return nil
	}

	// 🌟 2. Gmail サーバー側のゴミ箱へ移動（一括処理）
	// 大量にある場合は loop で a.TrashMessage(id) を呼ぶか、Batch API を検討
	fmt.Printf("🚀 %d 件の古いメールを Gmail のゴミ箱へ移動中...\n", len(ids))
	for _, id := range ids {
		_ = a.TrashMessage(id)
	}

	// 🌟 3. 最後に DB から物理削除（以前のコード）
	deleteQuery := fmt.Sprintf(
		"DELETE FROM messages WHERE (%s) AND (timestamp / 1000) < unixepoch('now', '-%d day')",
		whereClause, rules.TTLdays,
	)
	_, err = a.db.Exec(deleteQuery, args...)

	fmt.Printf("🧹 サーバーとDBの両方から %d 日前のメールを掃除しました\n", rules.TTLdays)
	return err
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
		//		a.LoadChannelsFromJson() // DB反映
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

// GetThreadHistory: 指定されたメールに関連するスレッド履歴を物理的な ID 鎖から抽出する
func (a *App) GetThreadHistory(targetMessageID string, threadID string, references string) ([]MessageSummary, error) {
	// 🌟 1. References 文字列を個別の ID 配列に分割 (スペースや改行で区切られている)
	refIDs := strings.Fields(references)

	// もし自分の ID があれば検索対象に加える（相手から見て自分は Ref になるため）
	if targetMessageID != "" {
		refIDs = append(refIDs, targetMessageID)
	}

	// 検索対象が何もない場合は空で返す（おもてなし）
	if len(refIDs) == 0 && threadID == "" {
		return []MessageSummary{}, nil
	}

	var conditions []string
	var args []interface{}

	// 🌟 2. Message-ID による物理的な鎖の検索 (IN 句の動的生成)
	if len(refIDs) > 0 {
		placeholders := make([]string, len(refIDs))
		for i, id := range refIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		// 「自分の ID が相手の Ref にある」か「相手の ID が自分の Ref にある」かを網羅
		conditions = append(conditions, fmt.Sprintf(
			"(message_id IN (%s) OR references_ids LIKE ?)",
			strings.Join(placeholders, ","),
		))
		// LIKE 用の引数を追加（References 内に自分の ID が含まれるか）
		args = append(args, "%"+targetMessageID+"%")
	}

	// 🌟 3. Gmail Thread ID による検索 (予備の強力な紐付け)
	if threadID != "" {
		conditions = append(conditions, "thread_id = ?")
		args = append(args, threadID)
	}

	// SQL 組み立て
	whereClause := strings.Join(conditions, " OR ")
	/*
		query := fmt.Sprintf(`
			SELECT id, thread_id, message_id, references_ids, sender, recipient, subject, snippet, importance, deadline, timestamp, is_read, manual_importance
			FROM messages
			WHERE %s
			ORDER BY timestamp ASC`, // 🌟 過去から未来へ時系列順に並べる
			whereClause,
		)
	*/
	query := fmt.Sprintf("SELECT %s FROM messages WHERE %s ORDER BY timestamp ASC", MessageSelectFields, whereClause)

	// 🌟 4. 実行とスキャン
	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("スレッド履歴取得失敗: %w", err)
	}
	defer rows.Close()

	var results []MessageSummary
	for rows.Next() {
		/*
			var m MessageSummary
			var deadlineNull sql.NullString
			err := rows.Scan(&m.ID, &m.ThreadID, &m.MessageID, &m.ReferencesIDs, &m.From, &m.Recipient, &m.Subject, &m.Snippet,
				&m.Importance, &deadlineNull, &m.Timestamp, &m.IsRead, &m.ManualImportance)
			if err != nil {
				fmt.Printf("Related err: %s\n", err)
				continue
			}
			if deadlineNull.Valid {
				m.Deadline = deadlineNull.String
			}
		*/

		if m, err := a.scanMessageSummary(rows); err == nil {
			results = append(results, m)
		}
	}

	return results, nil
}
