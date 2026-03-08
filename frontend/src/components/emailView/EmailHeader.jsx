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
        <button
          className="toggle-related-btn"
          onClick={onToggleRelated}
          aria-pressed={!!showRelated}
          aria-controls="related-pane"
          title={showRelated ? "関連ペインを閉じる" : "関連ペインを開く"}
        >
          {showRelated ? "関連を隠す" : "関連を開く"}
        </button>
      </div>
    </div>
  );
}
