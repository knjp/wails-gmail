// src/components/modals/AuthModal.jsx
export default function AuthModal({
  authURL,
  inputCode,
  onChangeCode,
  onOpenExternal,
  onComplete,
  onClose,
}) {
  const missing = authURL === "MISSING_CREDENTIALS";

  return (
    <div className="auth-overlay">
      <div className="auth-card">
        {missing ? (
          <div className="error-state">
            <h2>📁 credentials.json が必要です</h2>
            <p>Google Cloud Console で「デスクトップアプリ」用の認証情報を作成し、JSONをダウンロードしてください。</p>
            <div className="action-buttons">
              <button className="console-link-btn" onClick={() => onOpenExternal("https://console.cloud.google.com")}>
                🌐 Google Cloud Console を開く
              </button>
              <button className="retry-btn" onClick={() => window.location.reload()}>
                🔄 ファイルを置いたので再読み込み
              </button>
            </div>
            <p className="path-hint">保存先: <code>config/credentials.json</code></p>
          </div>
        ) : (
          <div className="auth-steps">
            <h2>🔑 Google ログイン</h2>
            <p>アプリを使用するために認証が必要です。</p>
            <button onClick={() => onOpenExternal(authURL)}>ブラウザを開いて承認</button>
            <input
              placeholder="表示されたコードを入力"
              value={inputCode}
              onChange={(e) => onChangeCode(e.target.value)}
            />
            <button onClick={onComplete}>認証を完了する</button>
          </div>
        )}
        <button className="close-x" onClick={onClose}>×</button>
      </div>
    </div>
  );
}
