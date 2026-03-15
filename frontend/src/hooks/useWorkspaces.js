// src/hooks/useWorkspaces.js
import { useEffect, useState, useCallback } from "react";
import { api } from "../api";

/**
 * useWorkspaces
 * - enabled: 認証完了後に true にする（それ以外は動かない）
 * - onPickInitial(tab: string): 初回ロード時の初期タブ選択コールバック
 */
export function useWorkspaces({ enabled, onPickInitial }) {
  const [workspaces, setWorkspaces] = useState([]);

  const load = useCallback(async () => {
    try {
      const list = await api.getWorkspaceList();
      setWorkspaces(list || []);

      // 初回選択（必要に応じて）
      if (onPickInitial && list?.length > 0) {
        const first = list[0];
        if (first?.channels?.length > 0) {
          onPickInitial(first.channels[0]);
        } else if (first?.id) {
          onPickInitial(first.id);
        }
      }
    } catch (e) {
      console.error("ワークスペース読み込み失敗:", e);
      setWorkspaces([]);
    }
  }, [onPickInitial]);

  const reloadFromJson = useCallback(async () => {
    try {
      await api.loadChannelConfigs();
      await load();
    } catch (e) {
      console.error("チャンネル設定リロード失敗:", e);
    }
  }, [load]);

  useEffect(() => {
    if (!enabled) return;
    load();
  }, [enabled, load]);

  return { workspaces, reloadFromJson };
}
