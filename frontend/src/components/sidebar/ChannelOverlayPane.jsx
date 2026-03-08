// src/components/sidebar/ChannelOverlayPane.jsx
import { useEffect, useRef, useState } from "react";

export default function ChannelOverlayPane({
  group,                // { group_name, channels, ... }
  onSelectChannel,      // (channel: string) => void
  onClose               // () => void
}) {
  const [open, setOpen] = useState(false);
  const closeBtnRef = useRef(null);

  useEffect(() => {
    setOpen(true);                 // マウント後にスライドイン
    closeBtnRef.current?.focus();  // A11y: 最初に閉じるへフォーカス
    const onKey = (e) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  if (!group) return null;

  return (
    <>
      {/* 背景のスクラム（クリックで閉じる） */}
      <div
        className="fixed inset-0 bg-black/30 z-[1100]"
        onClick={onClose}
        aria-hidden
      />
      {/* 左から被せるペイン */}
      <aside
        role="dialog"
        aria-modal="true"
        aria-labelledby="overlay-title"
        className={[
          "fixed left-0 top-0 h-screen w-[320px] z-[1201] bg-white",
          "shadow-2xl border-r border-slate-200",
          "transition-transform duration-200 ease-out",
          open ? "translate-x-0" : "-translate-x-full"
        ].join(" ")}
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

        <div className="p-2 overflow-y-auto h-[calc(100vh-3rem)]">
          {Array.isArray(group.channels) && group.channels.length > 0 ? (
            <ul className="flex flex-col">
              {group.channels.map((ch) => (
                <li key={ch}>
                  <button
                    className="w-full text-left px-3 py-2 rounded hover:bg-slate-100"
                    onClick={() => { onSelectChannel(ch); onClose(); }}
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
}
