// src/components/sidebar/ChannelOverlayPane.jsx
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

export default function ChannelOverlayPane({
  group,
  anchorLeft = 0,
  anchorTop = 0,
  anchorWidth = 0,
  onSelectChannel,
  onClose,
  autoCloseOnSelect = true,
  panelWidth = 280,
  panelClassName = "",
  modal = true,
}) {
  const [open, setOpen] = useState(false);
  const closeBtnRef = useRef(null);

  useEffect(() => {
    if (!group) return;
    setOpen(true);
    closeBtnRef.current?.focus();
    const onKey = (e) => e.key === "Escape" && onClose?.();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [group, onClose]);

  if (!group) return null;

  // サイドバーの上に重ねる場合（右にせり出すなら anchorLeft + anchorWidth）
  const left = anchorLeft;
  const top = anchorTop;
  const zBase = 60;

  const overlay = (
    <>
      {/* 背景（modal のときのみ） */}
      {modal && (
        <div
          className="fixed inset-0 bg-black/30"
          style={{ zIndex: zBase }}
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      {/* パネル本体：ヘッダー固定 + 本文スクロール */}
      <div
        className={[
          "fixed shadow-xl rounded-md border border-slate-200 bg-white",
          "flex flex-col",                        // ← 追加
          "transition-transform duration-150",
          open ? "translate-y-0 opacity-100" : "-translate-y-1 opacity-0",
          panelClassName,
        ].join(" ")}
        style={{
          left,
          top,
          width: panelWidth,
          // 画面からはみ出さない上限。必要なら 72vh などに調整
          maxHeight: "calc(100vh - 24px)",        // ← 重要
          zIndex: zBase + 1,
        }}
        role="dialog"
        aria-modal={modal}
        aria-label={`${group.group_name} のチャネル`}
      >
        {/* ヘッダー（固定） */}
        <div className="flex items-center justify-between px-3 py-2 border-b shrink-0">
          <h3 className="text-sm font-semibold truncate">
            {group.group_name} のチャネル
          </h3>
          <button
            ref={closeBtnRef}
            onClick={onClose}
            className="inline-flex items-center justify-center w-8 h-8 rounded hover:bg-slate-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-500"
            aria-label="閉じる"
            title="閉じる"
          >
            ✕
          </button>
        </div>

        {/* 本文（ここだけ縦スクロール） */}
        <div
          className="flex-1 overflow-y-auto px-2 py-2"
          style={{ WebkitOverflowScrolling: "touch" }} // iOS 慣性
        >
          {Array.isArray(group.channels) && group.channels.length > 0 ? (
            group.channels.map((ch) => (
              <button
                key={ch}
                onClick={() => {
                  onSelectChannel?.(ch);
                  if (autoCloseOnSelect) onClose?.();
                }}
                className={[
                  "group w-full text-left",
                  "px-3 py-1.5 rounded-lg border text-sm",
                  "flex items-center gap-2",
                  "transition-all hover:-translate-y-0.5 hover:shadow-sm",
                  "bg-white border-slate-200 text-slate-800 hover:bg-slate-50",
                  "focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-500",
                  "dark:bg-slate-800 dark:border-slate-700 dark:text-slate-100 dark:hover:bg-slate-700/60",
                ].join(" ")}
              >
                <span
                  className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200"
                  aria-hidden="true"
                >
                  #
                </span>
                <span className="flex-1 min-w-0 truncate">{ch.split("<")[0]}</span>
                <span className="opacity-0 group-hover:opacity-100 transition-opacity text-slate-400">
                  ↗
                </span>
              </button>
            ))
          ) : (
            <div className="px-3 py-4 text-slate-500">(チャネルなし)</div>
          )}
        </div>
      </div>
    </>
  );

  return createPortal(overlay, document.body);
}