// src/components/emailView/EmailActions.jsx
import React from "react";

export default function EmailActions({ isSummarizing, onSummarize, onDelete }) {
  return (
    <div className="main-actions">
      <button
        onClick={onSummarize}
        disabled={isSummarizing}
        className="summary-btn"
      >
        {isSummarizing ? "⌛ 要約中..." : "✨ AI要約"}
      </button>
      <button onClick={onDelete} className="delete-btn" title="ゴミ箱へ移動">
        🗑️
      </button>
    </div>
  );
}
