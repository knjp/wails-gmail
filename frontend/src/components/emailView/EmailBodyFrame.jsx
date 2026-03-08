// src/components/emailView/EmailBodyFrame.jsx
import React from "react";

export default function EmailBodyFrame({ messageId, srcDoc }) {
  return (
    <iframe
      key={`${messageId || "empty"}`}
      title="body"
      className="email-body-frame"
      srcDoc={srcDoc}
      // セキュリティ：最小限の許可。外部遷移は親側の postMessage→api.openExternal で処理
      sandbox="allow-popups allow-popups-to-escape-sandbox allow-scripts"
    />
  );
}
