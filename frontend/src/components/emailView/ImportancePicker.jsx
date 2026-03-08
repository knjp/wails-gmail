// src/components/emailView/ImportancePicker.jsx
import React from "react";

export default function ImportancePicker({ current, onChange }) {
  return (
    <div className="importance-picker-row">
      <span className="picker-label">重要度</span>
      <div className="imp-button-group">
        {[1, 2, 3, 4, 5].map((num) => (
          <button
            key={num}
            className={`imp-num-btn ${current === num ? "active" : ""}`}
            onClick={() => onChange(num)}
          >
            {num}
          </button>
        ))}
      </div>
    </div>
  );
}
