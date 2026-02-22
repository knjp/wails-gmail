//go:build server

package main

import (
	"fmt"
	"net/http"
)

// HandleAuthCallback: Googleからの帰り道（Redirect URI）を処理する
func (a *App) HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	// 1. URLパラメータから 'code' を取得
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	// 2. 既存の CompleteAuth を使い回して token.json を作成 & srv を初期化
	err := a.CompleteAuth(code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Auth failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 3. 成功したら React の画面へリダイレクト
	fmt.Println("✅ Web版の認証に成功しました")
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
