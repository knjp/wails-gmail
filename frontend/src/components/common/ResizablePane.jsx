// src/components/common/ResizablePane.jsx
import React, { useCallback, useEffect, useRef } from "react";

export default function ResizablePane({
  width,
  min = 200,
  max = Number.POSITIVE_INFINITY,
  onResize,
  onResizeEnd,
  handlePosition = "right",
  className = "",
  handleClassName = "",
  persistKey,
  children,
}) {
  const startXRef = useRef(0);
  const startWRef = useRef(width);

  const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

  useEffect(() => {
    if (!persistKey) return;
    const saved = localStorage.getItem(persistKey);
    if (saved) {
      const w = clamp(parseInt(saved, 10), min, max);
      if (!Number.isNaN(w)) onResize(w);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [persistKey]);

  const onMouseMove = useCallback(
    (e) => {
      const delta = e.clientX - startXRef.current;
      const next =
        handlePosition === "right"
          ? startWRef.current + delta
          : startWRef.current - delta;
      onResize(clamp(next, min, max));
    },
    [handlePosition, min, max, onResize]
  );

  const onMouseUp = useCallback(() => {
    window.removeEventListener("mousemove", onMouseMove);
    window.removeEventListener("mouseup", onMouseUp);
    const finalW = clamp(width, min, max);
    onResize(finalW);
    onResizeEnd?.(finalW);
    if (persistKey) localStorage.setItem(persistKey, String(finalW));
    document.body.style.cursor = "";
    document.body.style.userSelect = "";
  }, [onMouseMove, onResizeEnd, width, min, max, onResize, persistKey]);

  const onMouseDown = useCallback(
    (e) => {
      startXRef.current = e.clientX;
      startWRef.current = width;
      window.addEventListener("mousemove", onMouseMove);
      window.addEventListener("mouseup", onMouseUp);
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
      e.preventDefault();
    },
    [onMouseMove, onMouseUp, width]
  );

  const onKeyDown = useCallback(
    (e) => {
      if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
      const step = e.shiftKey ? 50 : 10;
      const delta = e.key === "ArrowRight" ? step : -step;
      const next =
        handlePosition === "right" ? width + delta : width - delta;
      const clamped = clamp(next, min, max);
      onResize(clamped);
      onResizeEnd?.(clamped);
      if (persistKey) localStorage.setItem(persistKey, String(clamped));
    },
    [width, handlePosition, min, max, onResize, onResizeEnd, persistKey]
  );

  // 親は必ず relative（ハンドル absolute の基準）
  const wrapperStyle = { width, position: "relative" };

  // ハンドルは absolute で右端／左端、全高、前面
  const handleStyle = {
    position: "absolute",
    top: 0,
    bottom: 0,
    right: handlePosition === "right" ? 0 : undefined,
    left: handlePosition === "left" ? 0 : undefined,
    cursor: "col-resize",
    zIndex: 10,
    // 幅はクラス側で指定（未指定のときのみフェールセーフ）
    // width: 8, ← 基本はコメントアウトして class で上書き
  };

  return (
    <div className={className} style={wrapperStyle}>
      <div style={{ width: "100%", height: "100%" }}>{children}</div>

      <div
        role="separator"
        aria-orientation="vertical"
        aria-valuemin={min}
        aria-valuemax={Number.isFinite(max) ? max : undefined}
        aria-valuenow={width}
        tabIndex={0}
        onKeyDown={onKeyDown}
        onMouseDown={onMouseDown}
        className={[handleClassName, "mail-list-resizer"].filter(Boolean).join(" ")}
        title="ドラッグ または ←/→（Shiftで加速）でサイズ変更"
        style={handleStyle}
      />
      {/* ← 連結を確実に */}
    </div>
  );
}