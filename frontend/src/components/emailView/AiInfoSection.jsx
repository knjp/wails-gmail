// src/components/emailView/AiInfoSection.jsx
import React from "react";

export default function AiInfoSection({ daysLeft, deadline, summary }) {
  const show = daysLeft !== null || !!summary;
  if (!show) return null;

  const bannerClass =
    daysLeft === null
      ? ""
      : daysLeft < 0
      ? "overdue"
      : daysLeft <= 3
      ? "urgent"
      : "";

  const bannerText =
    daysLeft === null
      ? ""
      : daysLeft < 0
      ? `期限切れ (${Math.abs(daysLeft)}日経過)`
      : daysLeft === 0
      ? "本日締切！"
      : `${deadline} まであと ${daysLeft} 日`;

  return (
    <div className="ai-info-section">
      {daysLeft !== null && (
        <div className={`deadline-banner ${bannerClass}`}>
          <span className="icon">📅</span>
          <span className="text">{bannerText}</span>
        </div>
      )}
      {summary && <div className="ai-summary-content">{summary}</div>}
    </div>
  );
}
