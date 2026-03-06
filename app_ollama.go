package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
)

func (a *App) ExtractDeadlines(id string) error {
	// Ollamaが無効なら何もせずに戻る
	if a.ollama == nil {
		return nil
	}
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

// AISearch は「あいまい検索」を実行して、スコアの高い順に ID を返します
func (a *App) AISearch(query string) ([]SearchResult, error) {
	if a.ollama == nil {
		// AIが利用できない場合は空結果
		return []SearchResult{}, nil
	}
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
	if a.ollama == nil {
		// AIが使えないなら検索自体をスキップ
		return []MessageSummary{}, nil
	}
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
