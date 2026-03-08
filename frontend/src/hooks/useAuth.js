// src/hooks/useAuth.js
import { useEffect, useState, useCallback } from "react";
import { api } from "../api";

/**
 * useAuth
 * - 設定（my_address）読み込み
 * - 認証が必要な場合の authURL 取得とモーダル制御
 * - 入力コード送信→リロード
 */
export function useAuth() {
  const [myAddress, setMyAddress] = useState("");
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [authURL, setAuthURL] = useState("");
  const [inputCode, setInputCode] = useState("");

  // 初期化：設定を読み込み、認証が必要ならモーダルを開く
  useEffect(() => {
    const init = async () => {
      try {
        const cfg = await api.getConfig();            // -> { my_address: string }
        setMyAddress(cfg.my_address || "");
        const url = await api.getAuthURL();           // 認証必要時は URL が返る / 済なら空
        if (url) {
          setAuthURL(url);
          setShowAuthModal(true);
        }
      } catch (e) {
        console.error("初期化エラー:", e);
        // 失敗時も authURL の取得を試みる
        const url = await api.getAuthURL().catch(() => "");
        if (url) {
          setAuthURL(url);
          setShowAuthModal(true);
        }
      }
    };
    init();
  }, []);

  // 認証完了：コード送信→全体リロード
  const completeAuth = useCallback(async () => {
    await api.completeAuth(inputCode);
    // リロードにより Wails/Go 側も含めて新しいトークンで再起動
    window.location.reload();
  }, [inputCode]);

  return {
    myAddress,
    showAuthModal, setShowAuthModal,
    authURL,
    inputCode, setInputCode,
    completeAuth,
  };
}
