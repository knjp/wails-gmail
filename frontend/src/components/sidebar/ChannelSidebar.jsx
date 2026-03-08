// src/components/sidebar/ChannelSidebar.jsx
import React, { useCallback, memo } from "react";

function SearchBar({ query, onQueryChange, onSearch }) {
  const handleKeyDown = useCallback(
    (e) => { if (e.key === "Enter") onSearch(); },
    [onSearch]
  );
  return (
    <div className="search-bar">
      <input
        type="text"
        placeholder="AIであいまい検索..."
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        onKeyDown={handleKeyDown}
      />
      <button onClick={onSearch}>検索</button>
    </div>
  );
}

const WorkspaceGroup = memo(function WorkspaceGroup({
  group,
  isExpanded,
  activeTab,
  onToggleGroup,
  onSelectTab,
}) {
  return (
    <div className="sidebar-group flex flex-col gap-1">
      <div
        className={`cursor-pointer text-xl font-bold p-2 rounded transition-all duration-200 flex items-center gap-2 ${
          activeTab === group.group_name
            ? "bg-blue-600 text-white shadow-lg scale-[1.02]"
            : "hover:bg-slate-200 text-slate-700"
        }`}
        onClick={() => {
          onToggleGroup(isExpanded ? null : group.group_name);
          onSelectTab(group.group_name);
        }}
      >
        <span className="text-sm opacity-70">{isExpanded ? "▼" : "▶"}</span>
        {group.group_name}
      </div>

      {isExpanded && group.type === "auto_group" && (
        <div className="group-items">
          {group.channels?.map((channel) => (
            <div
              key={channel}
              className={`channel-item ${activeTab === channel ? "active" : ""}`}
              onClick={() => onSelectTab(channel)}
            >
              <span className="hash">#</span> {channel.split("<")[0]}
            </div>
          ))}
          {(!group.channels || group.channels.length === 0) && (
            <div className="channel-item-empty">(該当者なし)</div>
          )}
        </div>
      )}
    </div>
  );
});

function ChannelSidebarBase({
  workspaces,
  activeGroup,
  activeTab,
  onToggleGroup,
  onSelectTab,
  query,
  onQueryChange,
  onSearch,
  onOpenChannelsEditor,
  onOpenSettings,
}) {
  return (
    <div className="channel-sidebar">
      <SearchBar query={query} onQueryChange={onQueryChange} onSearch={onSearch} />

      <div className="sidebar-header"><h3>🚀 Workspaces</h3></div>

      <div className="channel-list p-4 flex flex-col gap-4">
        {workspaces?.map((group) => (
          <WorkspaceGroup
            key={group.group_name}
            group={group}
            isExpanded={activeGroup === group.group_name}
            activeTab={activeTab}
            onToggleGroup={onToggleGroup}
            onSelectTab={onSelectTab}
          />
        ))}
      </div>

      <div className="sidebar-footer">
        <button className="settings-btn" onClick={onOpenChannelsEditor}>⚙️ チャンネル設定</button>
        <button className="settings-btn" onClick={onOpenSettings} style={{ marginTop: 10 }}>🔧 アプリ基本設定</button>
      </div>
    </div>
  );
}

// どの props 変化で再レンダするかを明示（必要十分な比較）
function propsAreEqual(prev, next) {
  // 文字列/プリミティブ
  if (prev.activeGroup !== next.activeGroup) return false;
  if (prev.activeTab !== next.activeTab) return false;
  if (prev.query !== next.query) return false;

  // workspaces は API 取得後に“新しい配列”になることが多い。
  // 大規模比較は避け、参照同一性で十分（変われば再レンダOK）。
  if (prev.workspaces !== next.workspaces) return false;

  // ハンドラは親で useCallback 化しておく前提
  if (prev.onToggleGroup !== next.onToggleGroup) return false;
  if (prev.onSelectTab !== next.onSelectTab) return false;
  if (prev.onQueryChange !== next.onQueryChange) return false;
  if (prev.onSearch !== next.onSearch) return false;
  if (prev.onOpenChannelsEditor !== next.onOpenChannelsEditor) return false;
  if (prev.onOpenSettings !== next.onOpenSettings) return false;

  return true;
}

export default memo(ChannelSidebarBase, propsAreEqual);
