const isWeb = !window.go;

const fetchApi = async (path, params = {}, method = 'GET') => {
    const url = new URL(path, window.location.origin);
    Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));
    const response = await fetch(url, { method });
    if (!response.ok) throw new Error(`API Error: ${response.status}`);
    const contentType = response.headers.get("content-type");
    return contentType && contentType.includes("application/json") ? response.json() : response.text();
};

export const api = {
    getConfig: async () => {
        if (isWeb) return fetch("/api/config").then(r => r.json());
        // 🌟 実行時にグローバルから直接引く（?. を使う）
        return window?.go?.main?.App?.GetConfig();
    },

    getMessages: async (channel) => {
        if (isWeb) {
            const response = await fetch(`/api/messages?name=${encodeURIComponent(channel)}`);
            return await response.json();
        }
        return window?.go?.main?.App?.GetMessagesByChannel(channel);
    },

    getChannels: async () => {
        let rawData;
        try {
            if (isWeb) {
                // 🌐 Web版: GoサーバーからJSON(オブジェクト配列)を取得
                const response = await fetch("/api/channels");
                if (!response.ok) throw new Error("Fetch error");
                rawData = await response.json(); 
            } else {
                // 🖥️ Desktop版: Wails経由で取得
                // 🌟 直接 window.go を見に行くことで import エラーを回避
                rawData = await window?.go?.main?.App?.GetChannels();
            }

            console.log("📥 受信データ(raw):", rawData);

            // 🛡️ 現代的な型ガードと整形 🛡️
            if (Array.isArray(rawData)) {
                // もし [{name: "全受信"}, ...] というオブジェクト配列なら、文字列配列 ["全受信", ...] に変換
                // そうでなければ(すでに文字列配列なら)そのまま使う
                return rawData.map(item => {
                    if (typeof item === 'object' && item !== null && item.name) {
                        return item.name;
                    }
                    return item; // すでに文字列ならそのまま
                });
            }
            return []; // 配列ですらない場合は空配列を返す
        } catch (err) {
            console.error("🚫 getChannels 失敗:", err);
            return [];
        }
    },

    // 🌟 設定再読み込み
    loadChannelsFromJson: async () => {
        if (isWeb) {
            // 🌐 Web版：POSTでリロードを要求し、最新の配列を受け取る
            const response = await fetch("/api/reload-channels", { method: 'POST' });
            if (!response.ok) throw new Error("Reload failed");
            return await response.json(); // 新しい ["受信トレイ", ...] が返る
        }
        return window?.go?.main?.App?.LoadChannelsFromJson();
    },

    getMessageBody: async (id) => {
        if (isWeb) {
            // 🌐 Web版：サーバーから本文を fetch
            const response = await fetch(`/api/message-body?id=${encodeURIComponent(id)}`);
            if (!response.ok) throw new Error("本文取得失敗");
            const bodyText = await response.text(); 
            console.log("📥 本文を受信しました (サイズ:", bodyText.length, ")");
            return bodyText; 
        }
        // 🖥️ Wails版：安全に呼び出す
        return window?.go?.main?.App?.GetMessageBody(id);
    },

    syncMessages: async () => {
        if (isWeb) {
            // 🌐 Web版：サーバー側の API を叩く
            const response = await fetch("/api/sync");
            if (!response.ok) throw new Error("同期失敗");
            return await response.json();
        }
        // 🖥️ Wails版：安全に呼び出す
        return window?.go?.main?.App?.SyncMessages();
    },

    syncHistoricalMessages: async (pageToken) => {
        if (!window.go) {
            // 🌐 Web版：サーバー側の API を叩く (後で Go 側に作成)
            const response = await fetch(`/api/sync-historical?token=${pageToken || ""}`);
            return await response.text(); // 次のトークン（文字列）を返す
        }
        // 🖥️ Wails版：安全なオプショナルチェーンで呼ぶ
        return window?.go?.main?.App?.SyncHistoricalMessages(pageToken);
    },

    getAuthURL: async () => {
        if (isWeb) return fetch("/api/auth-url").then(r => r.text());
        return window?.go?.main?.App?.GetAuthURL();
    },

    openExternal: (url) => {
        if (isWeb) {
            window.open(url, '_blank', 'noopener,noreferrer');
        } else {
            // Wailsのランタイムもグローバルから直接叩く
            window?.runtime?.BrowserOpenURL(url);
        }
    },

    // 🌟 AI検索
    getAISearchResults: async (query) => {
        if (isWeb) return fetchApi("/api/ai-search", { query });
        return window?.go?.main?.App?.GetAISearchResults(query);
    },

    // 🌟 AI要約
    summarizeEmail: async (id) => {
        if (isWeb) return fetchApi("/api/summarize", { id });
        return window?.go?.main?.App?.SummarizeEmail(id);
    },

    // 🌟 ゴミ箱ポイッ
    trashMessage: async (id) => {
        if (isWeb) return fetchApi("/api/trash", { id }, 'POST');
        return window?.go?.main?.App?.TrashMessage(id);
    },

    // 🌟 重要度の上書き
    setManualImportance: async (id, level) => {
        if (isWeb) return fetchApi("/api/set-importance", { id, level }, 'POST');
        return window?.go?.main?.App?.SetManualImportance(id, level);
    },

    // 🌟 既読にする
    markAsRead: async (id) => {
        if (isWeb) return fetchApi("/api/mark-read", { id }, 'POST');
        return window?.go?.main?.App?.MarkAsRead(id);
    },

    // 🌟 認証完了
    completeAuth: async (code) => {
        if (isWeb) return fetchApi("/api/complete-auth", { code }, 'POST');
        return window?.go?.main?.App?.CompleteAuth(code);
    },

    getChannelsRaw: async () => {
        if (isWeb) {
            return fetch("/api/channels/raw").then(r => r.text()); // JSONではなくtextで受ける
        }
        // Wails版（必要ならGo側に GetChannelsRaw を追加してください）
        return window?.go?.main?.App?.GetChannelsRaw();
    },

    // 🌟 2. 編集したテキストを保存
    saveChannelsRaw: async (jsonText) => {
        if (isWeb) {
            const response = await fetch("/api/channels/save", {
                method: 'POST',
                body: jsonText // そのまま文字列を投げる
            });
            if (!response.ok) {
                const errMsg = await response.text();
                throw new Error(errMsg);
            }
            return await response.json();
        }
        return window?.go?.main?.App?.SaveChannelsRaw(jsonText);
    }
};
