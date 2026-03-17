// src/components/emailView/EmailHeader.jsx
import React from "react";

export default function EmailHeader({ selectedMsg, isDirect, onToggleRelated, showRelated }) {
  const formatRecipient = (recipientStr) => {
    if (!recipientStr) return "（宛先なし）";
    
    // カンマやスペースで分割（宛先を配列にする）
    const addresses = recipientStr.split(/[ ,]+/).filter(Boolean);
    
    // 3名以上いる場合は、最初の2名だけ出して「ほか X 名」と表示
    if (addresses.length > 2) {
      return `${addresses.slice(0, 2).join(", ")} ほか ${addresses.length - 2} 名`;
    }
    return addresses.join(", ");
  };

  return (
    <div className="header-main">
      {!isDirect && <span className="ref-badge">参考情報</span>}
      <h2 className="detail-subject">{selectedMsg.subject}</h2>

      <div className="detail-meta">
        <div className="meta-row-meta">
          <span className="meta-label">From:</span>
          <span className="detail-from">{selectedMsg.from}</span>
        </div>

        <div className="meta-row">
          <span className="meta-label">To:</span>
          <span className="detail-to truncate" title={selectedMsg.recipient}>
            {formatRecipient(selectedMsg.recipient)}
          </span>
        </div>

        <span className="detail-date">
          📅 {new Date(selectedMsg.timestamp).toLocaleString("ja-JP")}
        </span>

      </div>
    </div>
  );
}
