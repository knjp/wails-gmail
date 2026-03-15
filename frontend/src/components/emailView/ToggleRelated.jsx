import React from "react";

export default function ToggleRelated({ onToggleRelated, showRelated }) {
  return (
    <button
      className="toggle-related-btn"
      onClick={onToggleRelated}
      aria-pressed={!!showRelated}
      aria-controls="related-pane"
      title={showRelated ? "関連ペインを閉じる" : "関連ペインを開く"}
    >
   {showRelated ? "関連を隠す" : "関連を開く"}
  </button>
  )
}
