// src/components/emailView/EmailView.jsx
import React from "react";
import EmailHeader from "./EmailHeader";
import EmailActions from "./EmailActions";
import ImportancePicker from "./ImportancePicker";
import ToggleRelated from "./ToggleRelated";
import AiInfoSection from "./AiInfoSection";
import EmailBodyFrame from "./EmailBodyFrame";

export default function EmailView({
  selectedMsg,
  isDirect,
  summary,
  isSummarizing,
  onSummarize,
  onDelete,
  onChangeImportance,
  daysLeft,
  fullBody,
  loadingBody, // 予備（スケルトン等を入れたい時に使用）
  onToggleRelated,
  showRelated,
  attachments,
  previewContent,
  setPreviewContent,
  onViewAttachment,
}) {
  if (!selectedMsg) {
    return <div className="empty-state">メールを選択してください</div>;
  }

  return (
    <div
      className={`email-view 
        ${selectedMsg.is_read === 0 ? "unread-view" : ""} 
        ${!isDirect ? "reference-view" : ""}`}
    >
      {/* 1) ヘッダー（左）＋ アクション＆重要度（右） */}
      <div className="email-header-top">
        <EmailHeader
          selectedMsg={selectedMsg}
          isDirect={isDirect}
          attachments={attachments}
          previewContent={previewContent}
          setPreviewContent={setPreviewContent}
          onViewAttachment={onViewAttachment}
        />
        <div className="header-actions-container">
          <EmailActions
            isSummarizing={isSummarizing}
            onSummarize={onSummarize}
            onDelete={() => onDelete(selectedMsg)}
          />
          <ToggleRelated
            onToggleRelated={onToggleRelated}
            showRelated={showRelated}
          />
          <ImportancePicker
            current={selectedMsg.importance}
            onChange={onChangeImportance}
          />
        </div>
      </div>

      {/* 2) AI 情報（期限バナー＋要約） */}
      <AiInfoSection
        daysLeft={daysLeft}
        deadline={selectedMsg.deadline}
        summary={summary}
      />

      {/* 3) 本文 */}
      <div className="email-body-container">
        <EmailBodyFrame messageId={selectedMsg.id} srcDoc={fullBody} />
        {/* 例：loadingBody && <div className="body-loading">読み込み中...</div> */}
      </div>
    </div>
  );
}
