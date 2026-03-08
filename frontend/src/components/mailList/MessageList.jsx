import React, { useMemo, useCallback } from "react";
import { groupLabelByDate } from "../../utils/dates";
import MessageItem from "./MessageItem";

export default function MessageList({
  messages,
  myAddress,
  selectedId,
  onSelect,
}) {
  // onSelect を安定化（MessageItem の memo を効かせる）
  const handleSelect = useCallback((m) => onSelect(m), [onSelect]);

  // 当日開始時刻はメモ化（レンダ毎に new Date 連発しない）
  const todayStart = useMemo(() => {
    const now = new Date();
    return new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  }, []);

  let lastGroup = "";
  return (
    <div role="list" aria-label="メール一覧">
      {messages.map((m) => {
        const group = groupLabelByDate(m.timestamp, todayStart);
        const showSep = group !== lastGroup;
        lastGroup = group;

        return (
          <React.Fragment key={m.id}>
            {showSep && <div className="list-separator">{group}</div>}
            <MessageItem
              m={m}
              isSelected={selectedId === m.id}
              myAddress={myAddress}
              onSelect={handleSelect}
            />
          </React.Fragment>
        );
      })}
    </div>
  );
}
