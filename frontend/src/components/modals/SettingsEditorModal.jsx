// src/components/modals/JsonEditorModal.jsx
export default function JsonEditorModal({ title, value, onChange, onSave, onClose }) {
  return (
    <div className="auth-overlay">
      <div className="auth-card" style={{ width: '80%', height: '80%', maxWidth: '800px' }}>
        <h3>{title}</h3>
        <p style={{ fontSize: '0.8rem', color: '#666' }}>
          JSON形式で入力してください。保存時にバリデーションが行われます。
        </p>
        <textarea
          style={{
            width: '100%', height: '70%',
            fontFamily: 'monospace', fontSize: '14px',
            padding: '10px', border: '1px solid #ccc'
          }}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
        <div style={{ marginTop: 20, display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
          <button onClick={onClose}>キャンセル</button>
          <button onClick={onSave} className="summary-btn">💾 保存して反映</button>
        </div>
      </div>
    </div>
  );
}
