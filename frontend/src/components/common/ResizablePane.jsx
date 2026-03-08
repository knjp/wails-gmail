// src/components/common/ResizablePane.jsx
import React, { useCallback, useEffect, useRef } from "react";

export default function ResizablePane({
  width,
  min = 240,
  max = 600,
  onResize,
  onResizeEnd,
  handlePosition = "right",
  className = "",
  handleClassName = "",
  persistKey,
  children,
}) {
  const startX = useRef(0);
  const startW = useRef(width);
  const lastW = useRef(width);          // ← 追加：最後に適用した幅を覚えておく
  const clamp = (v) => Math.max(min, Math.min(max, v));

  // 保存済み幅を復元
  useEffect(() => {
    if (!persistKey) return;
    const saved = localStorage.getItem(persistKey);
    if (saved != null) {
      const w = clamp(parseInt(saved, 10));
      if (!Number.isNaN(w)) {
        onResize(w);
        lastW.current = w;
        startW.current = w;
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [persistKey]);

  const onMove = useCallback((e) => {
    const delta = e.clientX - startX.current;
    const next =
      handlePosition === "right" ? startW.current + delta : startW.current - delta;
    const clamped = clamp(next);
    onResize(clamped);
    lastW.current = clamped;            // ← ここで最新値を保存
  }, [handlePosition, onResize]);

  const onUp = useCallback(() => {
    window.removeEventListener("mousemove", onMove);
    window.removeEventListener("mouseup", onUp);

    const finalW = clamp(lastW.current); // ← 直近値を使って確定（propのwidthは使わない）
    onResize(finalW);
    onResizeEnd?.(finalW);
    if (persistKey) localStorage.setItem(persistKey, String(finalW));

    startW.current = finalW;

    document.body.style.cursor = "";
    document.body.style.userSelect = "";
  }, [onMove, onResizeEnd, persistKey, onResize]);

  const onDown = useCallback((e) => {
    startX.current = e.clientX;
    startW.current = lastW.current;
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    e.preventDefault();
  }, [onMove, onUp]);

  const onKey = useCallback((e) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    const step = e.shiftKey ? 50 : 10;
    const delta = e.key === "ArrowRight" ? step : -step;
    const next =
      handlePosition === "right" ? lastW.current + delta : lastW.current - delta;
    const clamped = clamp(next);
    onResize(clamped);
    lastW.current = clamped;
    onResizeEnd?.(clamped);
    if (persistKey) localStorage.setItem(persistKey, String(clamped));
  }, [handlePosition, onResize, onResizeEnd, persistKey]);

  // 親は relative（ハンドル absolute の基準）
  const wrapperStyle = { width, position: "relative" };

  // ハンドルは右端・全高・前面
  const handleStyle = {
    position: "absolute",
    top: 0,
    bottom: 0,
    right: handlePosition === "right" ? 0 : undefined,
    left: handlePosition === "left" ? 0 : undefined,
    cursor: "col-resize",
    zIndex: 10,
  };

  return (
    <div className={className} style={{ width, position: "relative" }}>
      {children}
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="ペイン幅の調整"
        aria-valuemin={min}
        aria-valuemax={Number.isFinite(max) ? max : undefined}
        aria-valuenow={width}
        tabIndex={0}
        onKeyDown={onKey}
        onMouseDown={onDown}
        className={[handleClassName, "resizer-handle"].filter(Boolean).join(" ")}
        style={handleStyle}
        title="ドラッグ または ←/→（Shiftで±50）でサイズ変更"
      />
    </div>
  );
}