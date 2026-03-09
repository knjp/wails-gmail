// src/hooks/useMessageBody.js
import { useRef, useState, useCallback } from "react";
import { api } from "../api";

/**
 * useMessageBody
 * - 本文取得（api.getMessageBody）
 * - AbortController によるキャンセル制御
 * - 関連検索（api.getAISearchResults）
 * - UI で使うのは返り値と selectMessage() だけ
 */
export function useMessageBody() {
  const [fullBody, setFullBody] = useState("");
  const [loadingBody, setLoadingBody] = useState(false);
  const [relatedMsgs, setRelatedMsgs] = useState([]);
  const [threadMsgs, setThreadMsgs,] = useState([]);
  const abortRef = useRef(null);

  const selectMessage = useCallback(
    async (msg, { onMarkRead } = {}) => {
      // 既存リクエストがあれば中断
      if (abortRef.current) abortRef.current.abort();
      abortRef.current = new AbortController();
      const { signal } = abortRef.current;

      // 初期表示を即座に反映
      setLoadingBody(true);
      setRelatedMsgs([]);
      setThreadMsgs([]);
      setFullBody("読み込み中...");

      // 1) 本文取得
      try {
        const body = await api.getMessageBody(msg.id, { signal });
        setFullBody(body);
        onMarkRead?.(msg.id);     // 既読化（親から受け取る）
        api.markAsRead(msg.id);   // サーバ側更新（失敗してもUIの既読は維持）

        api.getThreadHistory(msg.message_id, msg.thread_id, msg.references_ids)
          .then(history => setThreadMsgs(history || []))
          .catch(e => console.error("スレッド履歴エラー:", e));

        api.getAISearchResults(msg.snippet)
          .then(related => setRelatedMsgs((related || []).filter(r => r.id !== msg.id)))
          .catch(e => console.error("関連検索エラー:", e));

      } catch (e) {
        if (e.name !== "AbortError") setFullBody("エラーが発生しました。");
      } finally {
        setLoadingBody(false);
      }

    },
    []
  );

  return { fullBody, loadingBody, relatedMsgs, threadMsgs, selectMessage };
}
