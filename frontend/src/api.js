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

    getWorkspaceList: async () => {
        let rawData;
        if (!window.go) {
            const response = await fetch("/api/workspace-list");
            if (!response.ok) throw new Error("Workspace list fetch failed");
            rawData = await response.json();
        } else {
            // 🖥️ Desktop版: Wails経由で新関数を呼ぶ
            rawData = await window?.go?.main?.App?.GetWorkspaceList();
        }

        // console.log("📥 受信したワークスペース構造:", rawData);
        return Array.isArray(rawData) ? rawData : [];
    },

    // リロード時も新しいリストを返すように
    loadChannelConfigs: async () => {
        if (!window.go) {
            const response = await fetch("/api/reload-channels", { method: 'POST' });
            return await response.json();
        }
        console.log("hello hello")
        return window?.go?.main?.App?.ReloadAndGetWorkspaces();
    },

    getMessageDetail: async (id) => {
        if (isWeb) {
            // 🌐 Web版：サーバーから本文を fetch
            const response = await fetch(`/api/message-detail?id=${encodeURIComponent(id)}`);
            if (!response.ok) throw new Error("本文取得失敗");
            const bodyText = await response.json(); 
            console.log("📥 本文を受信しました (サイズ:", bodyText.length, ")");
            return bodyText; 
        }
        // 🖥️ Wails版：安全に呼び出す
        return window?.go?.main?.App?.GetMessageDetail(id);
    },

    getAttachment: async (msgId, attachId) => {
        if (isWeb) {
            const response = await fetch(`/api/attachment?msg_id=${msgId}&attach_id=${attachId}`);
            if (!response.ok) throw new Error("添付取得失敗");
            return await response.text(); // Base64 文字列として受け取る
        }
        return window?.go?.main?.App?.GetAttachment(msgId, attachId);
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

    getThreadHistory: async (msgId, threadId, refs, subject_string) => {
        // refs はスペース区切りなので、念のため URL エンコード
        const params = new URLSearchParams({
            message_id: msgId || "",
            thread_id: threadId || "",
            references: refs || "",
            subject: subject_string || ""
        });

        if (!window.go) {
            // 🌐 Web版 (Docker)
            const response = await fetch(`/api/thread-history?${params.toString()}`);
            if (!response.ok) throw new Error("Thread history fetch failed");
            return await response.json();
        } else {
            // 🖥️ Desktop版 (Wails)
            return await window?.go?.main?.App?.GetThreadHistory(msgId, threadId, refs, subject_string);
        }
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
   changeAImadeImportance: async (id, level) => {
        if (isWeb) return fetchApi("/api/change-ai-importance", { id, level }, 'POST');
        return window?.go?.main?.App?.ChangeAImadeImportance(id, level);
    },

    // 🌟 重要度の上書き
    setManualImportance: async (id, level) => {
        if (isWeb) return fetchApi("/api/set-manual-importance", { id, level }, 'POST');
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
        return window?.go?.main?.App?.GetChannelsRaw();
    },

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
    },

    getSettingsRaw: async () => {
        if (isWeb) {
            return fetch("/api/settings/raw").then(r => r.text()); // JSONではなくtextで受ける
        }
        return window?.go?.main?.App?.GetSettingsRaw();
    },

    saveSettingsRaw: async (jsonText) => {
        if (isWeb) {
            const response = await fetch("/api/settings/save", {
                method: 'POST',
                body: jsonText // そのまま文字列を投げる
            });
            if (!response.ok) {
                const errMsg = await response.text();
                throw new Error(errMsg);
            }
            return await response.json();
        }
        return window?.go?.main?.App?.SaveSettingsRaw(jsonText);
    }

};
