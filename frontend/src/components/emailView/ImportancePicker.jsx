// src/components/emailView/ImportancePicker.jsx
import React from "react";

export default function ImportancePicker({ label, current, onChange }) {
  return (
    <div className="importance-picker-row flex items-center gap-2">
      <span className="picker-label font-bold text-slate-500">{label || "重要度"}</span>
      <div className="imp-button-group flex gap-1">
        {[1, 2, 3, 4, 5].map((num) => (
          <button
            key={num}
            className={`imp-num-btn px-3 py-1 rounded border
              ${
                current === num ? "bg-blue-500 text-white border-blue-600" : "bg-white text-slate-600 border-slate-300 hover:bg-slate-50"
                 } transition-colors text-xs`}
            onClick={() => onChange(num)}
          >
            {num}
          </button>
        ))}
      </div>
    </div>
  );
}
