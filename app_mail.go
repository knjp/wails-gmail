package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

	query := fmt.Sprintf("SELECT %s FROM messages WHERE %s ORDER BY timestamp DESC", MessageSelectFields, condition)
	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MessageSummary
	for rows.Next() {
		if m, err := a.scanMessageSummary(rows); err == nil {
			results = append(results, m)
		}
	}
	return results, nil
}

// GetMessagesByRules: ルールに合致するメッセージをすべて取得する
func (a *App) GetMessagesByRules(rules ChannelRules) ([]MessageSummary, error) {
	whereClause, args := a.BuildWhereClause(rules, false)
	query := fmt.Sprintf("SELECT %s FROM messages WHERE %s ORDER BY timestamp DESC LIMIT 100", MessageSelectFields, whereClause)
	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ルールクエリ失敗: %w", err)
	}
	defer rows.Close()

	var msgs []MessageSummary
	for rows.Next() {
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
func (a *App) BuildWhereClause(rules ChannelRules, onlySender bool) (string, []interface{}) {
	var mainConditions []string
	var args []interface{}
	var orParts []string

	// 1. ドメイン条件
	if len(rules.Domains) > 0 {
		var domainParts []string
		for _, d := range rules.Domains {
			if onlySender {
				domainParts = append(domainParts, "sender LIKE ?")
				args = append(args, "%"+d+"%")
			} else {
				domainParts = append(domainParts, "(sender LIKE ? OR recipient LIKE ?)")
				args = append(args, "%"+d+"%", "%"+d+"%")

			}
		}
		orParts = append(orParts, "("+strings.Join(domainParts, " OR ")+")")
	}

	// 2. キーワード条件
	if len(rules.Keywords) > 0 {
		var kwParts []string
		for _, k := range rules.Keywords {
			kwParts = append(kwParts, "(subject LIKE ? OR snippet LIKE ?)")
			args = append(args, "%"+k+"%", "%"+k+"%")
		}
		orParts = append(orParts, "("+strings.Join(kwParts, " OR ")+")")
	}

	if len(orParts) > 0 {
		mainConditions = append(mainConditions, "("+strings.Join(orParts, " OR ")+")")
	}

	// 3. 重要度条件
	if rules.ImportanceMin > 0 {
		mainConditions = append(mainConditions, "manual_importance >= ?")
		args = append(args, rules.ImportanceMin)
	}

	if rules.IsUnreadOnly {
		mainConditions = append(mainConditions, "is_read = 0")
	}

	// 条件がない場合は全件マッチ
	whereClause := "1=1"
	if len(mainConditions) > 0 {
		whereClause = strings.Join(mainConditions, " AND ")
	}

	// fmt.Printf("WHERE: %s\nargs: %s\n\n", whereClause, args)
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

func (a *App) GetMessageDetail(id string) (*MessageDetail, error) {
	//	fmt.Printf("GetmessageDetail(): %s\n\n\n\n", id)
	return a.GetMessageDetailWithContext(context.Background(), id)
}

func (a *App) GetMessageDetailWithContext(ctx context.Context, id string) (*MessageDetail, error) {
	var cachedBody string
	err := a.db.QueryRowContext(ctx, "SELECT body FROM messages WHERE id = ?", id).Scan(&cachedBody)

	/*
		// DBに本文（長さ1以上）があれば、それを即座に返す
		if err == nil && len(cachedBody) > 0 {
			fmt.Printf("Cache Hit! ID: %s (SQLiteから取得)\n", id)
			return cachedBody, nil
		}
	*/

	fmt.Printf("Cache Miss! ID: %s (APIから取得中...)\n", id)
	msg, err := a.srv.Users.Messages.Get("me", id).Format("full").Do()
	if err != nil {
		return nil, err
	}

	var attachs []AttachmentInfo
	a.findAttachmentsRecursive(msg.Payload.Parts, &attachs)

	body := cachedBody
	if len(body) == 0 {
		body = a.extractBody(msg.Payload)
		a.runBackgroundAnalysis(id, body)
	}

	// gmail で既読に変更
	go func() {
		err := a.MarkAsRead(id)
		if err != nil {
			fmt.Printf("既読同期失敗: %v\n", err)
		}
	}()

	// 3. 次回のために SQLite に保存（キャッシュ）しておく
	go func() {
		_, err = a.db.Exec("UPDATE messages SET body = ? WHERE id = ?", body, id)
		if err != nil {
			fmt.Printf("キャッシュ保存エラー: %v\n", err)
		}
	}()

	return &MessageDetail{
		Body:        body,
		Attachments: attachs,
	}, nil
}

func (a *App) findAttachmentsRecursive(parts []*gmail.MessagePart, results *[]AttachmentInfo) {
	for _, part := range parts {
		if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
			*results = append(*results, AttachmentInfo{
				ID:       part.Body.AttachmentId,
				FileName: part.Filename,
			})
		}
		if len(part.Parts) > 0 {
			a.findAttachmentsRecursive(part.Parts, results)
		}
	}
}

func (a *App) GetAttachment(msgID string, attachID string) (string, error) {
	attach, err := a.srv.Users.Messages.Attachments.Get("me", msgID, attachID).Do()
	if err != nil {
		return "", err
	}

	decoded, err := base64.URLEncoding.DecodeString(attach.Data)
	if err != nil {
		return "", fmt.Errorf("デコード失敗: %v", err)
	}

	return base64.StdEncoding.EncodeToString(decoded), nil
}

func (a *App) OpenAttachmentExternally(fileName string, base64Data string) error {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return err
	}

	tmpPath := filepath.Join(os.TempDir(), fileName)
	err = os.WriteFile(tmpPath, data, 0644)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", tmpPath)
	case "darwin":
		cmd = exec.Command("open", tmpPath)
	default: // linux
		cmd = exec.Command("xdg-open", tmpPath)
	}

	return cmd.Run()
}

func (a *App) runBackgroundAnalysis(id string, body string) {
	var subject, sender string
	a.db.QueryRow("SELECT subject, sender FROM messages WHERE id = ?", id).Scan(&subject, &sender)

	// 1. AI学習（ベクトル化）
	go func() {
		fullText := fmt.Sprintf("From: %s\nSubject: %s\nBody: %s", sender, subject, body)
		if len(fullText) > 4000 {
			fullText = fullText[:4000]
		}
		if fullText != "" {
			a.SyncEmailVector(id, fullText)
		}
	}()

	// 2. 締め切り抽出
	go func() {
		if body != "" {
			fmt.Printf("🤖 Ollama 締め切り抽出開始: %s\n", id)
			_ = a.ExtractDeadlines(id)
		}
	}()
}

// フロントエンドから呼ばれる関数
func (a *App) OpenExternalLink(url string) {
	wailsRuntime.BrowserOpenURL(a.ctx, url)
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

	res, err := a.srv.Users.Messages.List("me").Q("newer_than:1d").MaxResults(50).Do()
	if err != nil {
		return err
	}

	for _, m := range res.Messages {
		var exists int
		a.db.QueryRow("SELECT COUNT(*) FROM messages WHERE id = ?", m.Id).Scan(&exists)
		if exists > 0 {
			continue // 既に持っているので次のメールへ
		}

		msg, _ := a.srv.Users.Messages.Get("me", m.Id).Format("metadata").Do()
		if err != nil {
			fmt.Printf("⚠️ メール取得失敗 (%s): %v\n", m.Id, err)
			continue
		}
		a.processSingleMessage(msg, false)
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
			fmt.Printf("⚠️ メール取得失敗 (%s): %v\n", m.Id, err)
			continue // この 1 通は諦めて次へ

		}
		a.processSingleMessage(msg, true)
	}
	// 次のページの合言葉を返す
	return res.NextPageToken, nil
}

// 🌟 1通のメッセージを解析・保存・AI学習させる
func (a *App) processSingleMessage(msg *gmail.Message, useReplace bool) error {
	// 1. 既読判定
	isRead := 1
	for _, label := range msg.LabelIds {
		if label == "UNREAD" {
			isRead = 0
			break
		}
	}

	// 2. ヘッダー解析
	var sender, subject, to, cc, msgID string
	involvedMap := make(map[string]bool)
	refIDMap := make(map[string]bool)

	for _, h := range msg.Payload.Headers {
		val := h.Value
		if strings.EqualFold(h.Name, "From") {
			sender = val
		}
		if strings.EqualFold(h.Name, "Subject") {
			subject = val
		}
		if strings.EqualFold(h.Name, "To") {
			to = val
		}
		if strings.EqualFold(h.Name, "Cc") {
			cc = val
		}
		if strings.EqualFold(h.Name, "Message-ID") {
			msgID = val
		}

		if strings.EqualFold(h.Name, "From") || strings.EqualFold(h.Name, "To") || strings.EqualFold(h.Name, "Cc") {
			for _, addr := range strings.Fields(val) {
				involvedMap[addr] = true
			}
		}
		if strings.EqualFold(h.Name, "References") || strings.EqualFold(h.Name, "In-Reply-To") {
			for _, rid := range strings.Fields(val) {
				refIDMap[rid] = true
			}
		}
	}

	delete(refIDMap, msgID)
	combinedRecipient := to + " " + cc

	var involvedList, refList []string
	for k := range involvedMap {
		involvedList = append(involvedList, k)
	}
	for k := range refIDMap {
		refList = append(refList, k)
	}

	allInvolved := strings.Join(involvedList, " ")
	references := strings.Join(refList, " ")

	// 3. DB保存（新規なら IGNORE、履歴なら REPLACE）
	verb := "INSERT OR IGNORE"
	if useReplace {
		verb = "INSERT OR REPLACE"
	}

	sql := fmt.Sprintf(`%s INTO messages (
		id,
		thread_id,
		sender,
		recipient,
		all_involved_adresses,
		message_id,
		references_ids,
		subject,
		snippet,
		timestamp,
		is_read
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		verb)

	_, err := a.db.Exec(
		sql, msg.Id, msg.ThreadId, sender, combinedRecipient,
		allInvolved, msgID, references, subject, msg.Snippet, msg.InternalDate, isRead,
	)
	if err != nil {
		return err
	}

	// 4. AI学習（非同期）
	go func() {
		if msg.Snippet == "" && subject == "" {
			return
		}
		text := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\nSnippet: %s", sender, combinedRecipient, subject, msg.Snippet)
		if len(text) > 4000 {
			text = text[:4000]
		}
		a.SyncEmailVector(msg.Id, text)
	}()

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
	whereClause, args := a.BuildWhereClause(rules, true)
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
func (a *App) GetThreadHistory(targetMessageID string, threadID string, references string, subject string) ([]MessageSummary, error) {
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

	cleanSubject := strings.TrimPrefix(subject, "[SPF] ")
	cleanSubject = strings.TrimPrefix(strings.TrimPrefix(cleanSubject, "Re: "), "Fwd: ")

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

	if cleanSubject != "" {
		conditions = append(conditions, "(subject LIKE ?)")
		args = append(args, "%"+cleanSubject+"%")
	}

	// SQL 組み立て
	whereClause := strings.Join(conditions, " OR ")
	query := fmt.Sprintf("SELECT %s FROM messages WHERE %s ORDER BY timestamp ASC", MessageSelectFields, whereClause)

	// 🌟 4. 実行とスキャン
	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("スレッド履歴取得失敗: %w", err)
	}
	defer rows.Close()

	var results []MessageSummary
	for rows.Next() {
		if m, err := a.scanMessageSummary(rows); err == nil {
			results = append(results, m)
		}
	}

	return results, nil
}
