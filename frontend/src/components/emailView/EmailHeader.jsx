// src/components/emailView/EmailHeader.jsx
import React from "react";
import {useState} from 'react';

export default function EmailHeader({ selectedMsg, isDirect, attachments, previewPdf, setPreviewPdf, onViewAttachment }) {
  const formatRecipient = (recipientStr) => {
    if (!recipientStr) return "（宛先なし）";
    
    // カンマやスペースで分割（宛先を配列にする）
    const addresses = recipientStr.split(/[ ,]+/).filter(Boolean);
    
    // 3名以上いる場合は、最初の2名だけ出して「ほか X 名」と表示
    if (addresses.length > 2) {
      return `${addresses.slice(0, 2).join(", ")} ほかほか ${addresses.length - 2} 名`;
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

        {attachments && attachments.length > 0 && (
          <div className="attachments-bar mt-3 flex flex-wrap gap-2 border-t pt-2">
            {attachments.map((file) => (
              <button
                key={file.id}
                onClick={() => onViewAttachment(selectedMsg.id, file.id, file.file_name)}
                className="flex items-center gap-2 px-3 py-1.5 bg-slate-100 hover:bg-blue-100 
                           text-slate-700 text-xs rounded-full border border-slate-200 
                           transition-colors duration-200 shadow-sm"
              >
                <span role="img" aria-label="clip">📎</span>
                <span className="font-medium truncate max-w-[150px]">{file.file_name}</span>
              </button>
            ))}
          </div>
        )}

        {previewPdf && (
          <div className="pdf-modal fixed inset-0 z-50 bg-black/50 flex items-center justify-center p-10">
            <div className="bg-white w-full h-full flex flex-col rounded-xl overflow-hidden">
              <div className="p-4 flex justify-between items-center border-b">
                <span className="font-bold">📄 PDF プレビュー</span>
                <button onClick={() => setPreviewPdf(null)} className="text-2xl">×</button>
              </div>
              <iframe 
                src={`data:application/pdf;base64,${previewPdf}`} 
                className="w-full h-full border-none"
              />
            </div>
          </div>
        )}

      </div>
    </div>
  );
}
