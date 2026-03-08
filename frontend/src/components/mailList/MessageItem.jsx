import React, { memo, useCallback } from "react";

function MessageItemBase({ m, isSelected, myAddress, onSelect }) {
  const to = m?.recipient ?? "";
  const isDirect = typeof to === "string" && to.includes(myAddress);
  const isML = to && !isDirect;

  const handleClick = useCallback(() => onSelect(m), [onSelect, m]);
  const handleKeyDown = useCallback(
    (e) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        onSelect(m);
      }
    },
    [onSelect, m]
  );

  return (
    <div role="listitem" aria-selected={isSelected}>
      <div
        className={`mail-item 
          ${isSelected ? "selected" : ""} 
          ${m.is_read === 0 ? "unread-item" : ""} 
          importance-${m.importance}`}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        tabIndex={0}
        data-testid={`message-item-${m.id}`}
        aria-label={`${m.subject || "件名なし"} - ${m.from || ""}`}
      >
        <div className="subject">
          {isDirect ? (
            <span className="recipient-badge direct">TO ME</span>
          ) : isML ? (
            <span className="recipient-badge ml">ML</span>
          ) : null}
          {m.subject}
          {m.importance >= 4 && (
            <span className={`importance-badge level-${m.importance}`}>
              {m.importance === 5 ? "🔥 CRITICAL" : "⚡ IMPORTANT"}
            </span>
          )}
        </div>
        <div className="list-snippet">{m.snippet}</div>
        <div className="from">{m.from}</div>
        <div className="mail-date">
          {new Date(m.timestamp).toLocaleString("ja-JP", {
            year: "numeric",
            month: "2-digit",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
          })}
        </div>
      </div>
    </div>
  );
}

// 再レンダ抑制：選択状態・未読/重要度・主要表示項目に変化がない限り再描画しない
export default memo(MessageItemBase, (prev, next) => {
  if (prev.isSelected !== next.isSelected) return false;
  if (prev.myAddress !== next.myAddress) return false;
  const a = prev.m, b = next.m;
  return (
    a.id === b.id &&
    a.is_read === b.is_read &&
    a.importance === b.importance &&
    a.snippet === b.snippet &&
    a.subject === b.subject &&
    a.from === b.from &&
    a.timestamp === b.timestamp &&
    a.recipient === b.recipient
  );
});
