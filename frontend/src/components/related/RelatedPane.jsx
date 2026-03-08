// src/components/related/RelatedPane.jsx
import React, { memo } from "react";

function RelatedPaneBase({ relatedMsgs, onSelect }) {
  return (
    <div className="related-pane">
      <div className="pane-header">🔗 関連・過去の経緯</div>
      <div className="related-list-container">
        {(!relatedMsgs || relatedMsgs.length === 0) && (
          <div className="info">関連なし</div>
        )}

        {relatedMsgs?.map((rm) => (
          <div
            key={rm.id}
            className="mail-item related-item"
            onClick={() => onSelect(rm)}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onSelect(rm);
              }
            }}
            aria-label={`${rm.subject || "件名なし"} - ${rm.from || ""}`}
          >
            <div className="subject-small">{rm.subject}</div>
            <div className="list-snippet">{rm.snippet}</div>
            <div className="from">{rm.from}</div>
            <div className="mail-date">Time</div>
            <div className="date-small">
              {new Date(rm.timestamp).toLocaleDateString()}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// 余計な再レンダを避けるため memo 化（配列参照が変わらない間は再描画しない）
export default memo(RelatedPaneBase);
