// src/components/related/RelatedPane.jsx
import React, { memo } from "react";

function RelatedPaneBase({ threadMsgs, aiMsgs, onSelect }) {
  const [mode, setMode] = React.useState('thread');
  const displayMsgs = mode === 'thread' ? threadMsgs : aiMsgs;

  return (
    <div className="related-pane">
      {/* 🌟 3. 現代的なタブ切り替えヘッダー */}
      <div className="pane-header flex gap-2 border-b">
        <button 
          className={`flex-1 p-2 ${mode === 'thread' ? 'bg-blue-100 font-bold border-b-2 border-blue-600' : 'opacity-50'}`}
          onClick={() => setMode('thread')}
        >
          🔗 スレッド
        </button>
        <button 
          className={`flex-1 p-2 ${mode === 'ai' ? 'bg-blue-100 font-bold border-b-2 border-blue-600' : 'opacity-50'}`}
          onClick={() => setMode('ai')}
        >
          関連検索 (AI)
        </button>
      </div>
      <div className="related-list-container">
        {(!displayMsgs || displayMsgs.length === 0) && (
          <div className="info p-4 text-center text-slate-400">該当なし</div>
        )}

        {displayMsgs?.map((rm) => (
          <div key={rm.id} className="mail-item related-item" onClick={() => onSelect(rm)}>
            <div className="subject-small font-semibold text-sm truncate">{rm.subject}</div>
            <div className="list-snippet text-xs text-slate-500 line-clamp-2">{rm.snippet}</div>
            <div className="from text-xs text-blue-600">{rm.from}</div>
            <div className="date-small text-[10px] text-right">
              {new Date(rm.timestamp).toLocaleDateString()}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// 余計な再レンダを避けるため memo 化（配列参照が変わらない間は再描画しない）
export default RelatedPaneBase;
//export default memo(RelatedPaneBase, (prev, next) => {
//  return (
//    prev.threadMsgs === next.threadMsgs &&
//    prev.aiMsgs === next.aiMsgs
//  );
//});
