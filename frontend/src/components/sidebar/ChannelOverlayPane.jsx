// src/components/sidebar/ChannelOverlayPane.jsx
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

export default function ChannelOverlayPane({
  group,
  anchorLeft = 0,      // App.jsx から渡す：sidebar の left 座標（px）
  anchorTop = 0,       // 必要なら使う：上端オフセット（px）
  anchorWidth,         // 必要なら使う：sidebar 幅
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
    setOpen(true);                          // マウント後にスライドイン
    closeBtnRef.current?.focus();           // A11y: 初回フォーカス
    const onKey = (e) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  if (!group) return null;

  const overlay = (
    <>
      {/* 背景: modal=true のときだけ描画 */}
      { modal && (
        <div
          className="fixed inset-0 bg-black/30 z-[1100]"
          onClick={onClose}
          aria-hidden
        />
      )}
      {/* ← サイドバー座標にアンカーして“左から重ねる” */}
      <aside
        role={modal ? "dialog" : undefined }
        aria-modal={modal ? "true" : undefined}
        aria-labelledby="overlay-title"
        className={[
          "fixed z-[1201] bg-white shadow-2xl border-r border-slate-200",
          "transition-transform duration-200 ease-out",
          open ? "translate-x-0" : "-translate-x-full",
          panelClassName || ""
        ].join(" ")}
        style={{
          left: `${anchorLeft}px`,                 // ★ ここが肝：sidebar の left に重ねる
          top: `${anchorTop}px`,                   // 上に固定ヘッダがあれば有効化
          width: typeof panelWidth === "number" ? `${panelWidth}px` : panelWidth,
          height: `calc(100vh - ${anchorTop}px)`   // 上オフセット分を差し引いた全高
        }}
      >
        <header className="flex items-center justify-between px-4 h-12 border-b border-slate-200">
          <h2 id="overlay-title" className="text-sm font-semibold">
            {group.group_name} のチャネル
          </h2>
          <button
            ref={closeBtnRef}
            onClick={onClose}
            className="text-slate-600 hover:text-slate-800"
            aria-label="閉じる (Esc)"
          >
            ✕
          </button>
        </header>

        <div className="p-2 overflow-y-auto h-[calc(100vh-3rem-0px)]">
          {Array.isArray(group.channels) && group.channels.length > 0 ? (
            <ul className="flex flex-col">
              {group.channels.map((ch) => (
                <li key={ch}>
                  <button
                    className="w-full text-left px-3 py-2 rounded hover:bg-slate-100"
                    onClick={() => {
                      onSelectChannel(ch);
                      if (autoCloseOnSelect) onClose();
                    }}
                  >
                    <span className="text-slate-500 mr-1">#</span>
                    {ch.split("<")[0]}
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <div className="text-sm text-slate-500 px-3 py-4">（チャネルなし）</div>
          )}
        </div>
      </aside>
    </>
  );

  // ★ レイアウト外（<body> 直下）に描画：flex の影響を完全に断つ
  return createPortal(overlay, document.body);
}