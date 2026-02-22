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
        if (isWeb) {
            rawData = await fetch("/api/channels").then(r => r.json());
        } else {
            rawData = await window?.go?.main?.App?.GetChannels();
        }
        // データ整形ロジック
        if (Array.isArray(rawData) && rawData.length > 0 && typeof rawData[0] === 'object') {
            return rawData.map(item => item.name);
        }
        return rawData;
    },

    getMessageBody: async (id) => {
        if (isWeb) {
            // 🌐 Web版：サーバーから本文を fetch
            const response = await fetch(`/api/message-body?id=${encodeURIComponent(id)}`);
            if (!response.ok) throw new Error("本文取得失敗");
            return await response.text(); // HTML/Textなので .text() で受ける
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

    // 🌟 設定再読み込み
    loadChannelsFromJson: async () => {
        if (isWeb) return fetchApi("/api/reload-channels", {}, 'POST');
        return window?.go?.main?.App?.LoadChannelsFromJson();
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
    }
};
