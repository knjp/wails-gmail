// src/components/emailView/EmailHeader.jsx
import React from "react";

export default function EmailHeader({ selectedMsg, isDirect, onToggleRelated, showRelated }) {
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
          <span className="detail-to">
            {selectedMsg.recipient || "（宛先なし）"}
          </span>
        </div>

        <span className="detail-date">
          📅 {new Date(selectedMsg.timestamp).toLocaleString("ja-JP")}
        </span>

      </div>
    </div>
  );
}
