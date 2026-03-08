// src/hooks/useMessages.js
import { useEffect, useRef, useState, useCallback } from "react";
import { api } from "../api";

/** 一覧取得・同期・ページング・個別更新 */
export function useMessages(activeTab) {
  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(false);
  const [nextPageToken, setNextPageToken] = useState("");
  const reqRef = useRef(0);

  useEffect(() => {
    const current = ++reqRef.current;
    const load = async () => {
      const data = await api.getMessages(activeTab);
      if (current !== reqRef.current) return;
      setMessages(data || []);
      try {
        await api.syncMessages();
        if (current !== reqRef.current) return;
        const fresh = await api.getMessages(activeTab);
        setMessages(fresh || []);
      } catch (e) {
        console.error("同期エラー:", e);
      }
    };
    load();
  }, [activeTab]);

  const loadMore = useCallback(async () => {
    setLoading(true);
    const token = await api.syncHistoricalMessages(nextPageToken);
    setNextPageToken(token);
    const data = await api.getMessages(activeTab);
    setMessages(data || []);
    setLoading(false);
  }, [activeTab, nextPageToken]);

  const updateOne = useCallback((id, patch) => {
    setMessages(prev => prev.map(m => (m.id === id ? { ...m, ...patch } : m)));
  }, []);
  const removeOne = useCallback((id) => {
    setMessages(prev => prev.filter(m => m.id !== id));
  }, []);

  return { messages, loading, loadMore, updateOne, removeOne, setMessages };
}
