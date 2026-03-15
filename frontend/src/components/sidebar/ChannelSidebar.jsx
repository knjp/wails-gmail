// src/components/sidebar/ChannelSidebar.jsx
import React, { useCallback, memo } from "react";

function SearchBar({ query, onQueryChange, onSearch }) {
  const handleKeyDown = useCallback(
    (e) => { if (e.key === "Enter") onSearch(); },
    [onSearch]
  );
  return (
    <div className="p-2">
      <input
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        onKeyDown={handleKeyDown}
        className="w-full border rounded px-2 py-1"
        placeholder="検索..."
        aria-label="検索語"
      />
      <button onClick={onSearch} className="ml-2 px-2 py-1 border rounded">
        検索
      </button>
    </div>
  );
}

const WorkspaceGroup = memo(function WorkspaceGroup({
  group, isExpanded, activeTab, onToggleGroup, onSelectTab, onOpenWorkspace,
}) {
  const isActive = isExpanded || activeTab === group.id;
  const channelCount = Array.isArray(group.channels) ? group.channels.length : 0;
  const typeLabel = { auto_group: "自動グループ", fixed: "固定", default: "標準", recommend: "推奨"};

  const baseBtn =
    "w-full flex items-center gap-2 px-3 py-2 rounded-lg border " +
    "shadow-sm transition-all select-none " +
    "hover:-translate-y-0.5 hover:shadow-md " +
    "focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-500";

  const activeCls =
    "bg-sky-50 border-sky-300 text-sky-900 " +
    "dark:bg-sky-900/30 dark:border-sky-700 dark:text-sky-100";

  const inactiveCls =
    "bg-white border-slate-200 text-slate-800 hover:bg-slate-50 " +
    "dark:bg-slate-800 dark:border-slate-700 dark:text-slate-100 " +
    "dark:hover:bg-slate-700/60";

  return (
    <div className="px-2 py-1">
      <button
        className={baseBtn + " " + (isActive ? activeCls : inactiveCls)}
        onClick={() => {
          //onSelectTab(group.id);
          if (group.channels?.length > 0) {
            onSelectTab(group.id);
          } else {
            onSelectTab(group.id);
          }
          if (!onOpenWorkspace) {
            onToggleGroup(isExpanded ? null : group.id);
          }
        }}
      >
        🗂️
        <span className="flex-1 truncate">{group.group_name}</span>
        <span className="text-xs opacity-60">
          {typeLabel[group.type] ?? "その他"}
        </span>
            {/* ▶ボタン */}
        <button
          className="ml-2 px-2 py-1 text-sm rounded border bg-white hover:bg-slate-100"
          onClick={(e) => {
            e.stopPropagation(); 
            onOpenWorkspace?.(group); // ← これでオーバレイを出す
          }}
        >
            ▶
        </button>
      </button>

      {/* 折りたたみ配下（useOverlay=false のときだけ） */}
      {isExpanded && (group.type === "auto_group" || group.type === "recommend") && !onOpenWorkspace && (
        <div className="pl-11 pt-1 pb-2 flex flex-col gap-1.5">
          {group.channels?.map((channel) => {
            const isActiveCh = activeTab === channel;
            return (
              <button
                key={channel}
                onClick={() => onSelectTab(channel)}
                className={[
                  "group w-full text-left",
                  "px-3 py-1.5 rounded-lg border text-sm",
                  "flex items-center gap-2",
                  "transition-all hover:-translate-y-0.5 hover:shadow-sm",
                  "focus:outline-none focus-visible:ring-2 focus-visible:ring-sky-500",
                  isActiveCh
                    ? "bg-sky-50 border-sky-300 text-sky-900 dark:bg-sky-900/30 dark:border-sky-700 dark:text-sky-100"
                    : "bg-white border-slate-200 text-slate-800 hover:bg-slate-50 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-100 dark:hover:bg-slate-700/60",
                ].join(" ")}
              >
                <span
                  className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200"
                  aria-hidden="true"
                >
                  #
                </span>
                <span className="flex-1 min-w-0 truncate">{channel.split("<")[0]}</span>
                <span className="opacity-0 group-hover:opacity-100 transition-opacity text-slate-400">
                  ↗
                </span>
              </button>
            );
          })}
          {(!group.channels || group.channels.length === 0) && (
            <div className="px-2 py-1 text-slate-500">(該当者なし)</div>
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
  onOpenWorkspace,
  onToggleGroup,
  onSelectTab,
  query,
  onQueryChange,
  onSearch,
  onOpenChannelsEditor,
  onOpenSettings,
}) {
  return (
    /**
     * ★ 重要：縦の flex 3分割
     *  - ヘッダー（shrink-0）
     *  - 本文（flex-1 overflow-y-auto）：ここだけスクロール
     *  - フッター（sticky bottom-0 / もしくは mt-auto shrink-0）
     */
    <aside className="w-full h-full min-w-0 flex flex-col">
      {/* Header */}
      <div className="shrink-0">
        <h4 className="px-3 py-2 text-sm font-semibold">🚀 Workspaces</h4>
        <SearchBar query={query} onQueryChange={onQueryChange} onSearch={onSearch} />
      </div>

      {/* Body: scrollable */}
      <div className="flex-1 overflow-y-auto px-0.5">
        <div className="mt-2">
          {workspaces?.map((group) => (
            <WorkspaceGroup
              key={group.group_name}
              group={group}
              isExpanded={activeGroup === group.id}
              activeTab={activeTab}
              onToggleGroup={onToggleGroup}
              onSelectTab={onSelectTab}
              onOpenWorkspace={onOpenWorkspace}
            />
          ))}
        </div>
      </div>

      {/* Footer: 固定（sticky） */}
      <div className="sticky bottom-0 z-10 bg-white/85 backdrop-blur supports-[backdrop-filter]:bg-white/60 border-t border-slate-200 px-3 py-2 dark:bg-slate-800/85 dark:border-slate-700">
        <div className="flex gap-2">
          <button onClick={onOpenChannelsEditor} className="px-2 py-1 border rounded">
            ⚙️ チャンネル設定
          </button>
          <button onClick={onOpenSettings} className="px-2 py-1 border rounded">
            🔧 アプリ基本設定
          </button>
        </div>
      </div>
    </aside>
  );
}

// 比較関数は現状どおり
function propsAreEqual(prev, next) {
  if (prev.activeGroup !== next.activeGroup) return false;
  if (prev.activeTab !== next.activeTab) return false;
  if (prev.query !== next.query) return false;
  if (prev.workspaces !== next.workspaces) return false;
  if (prev.onToggleGroup !== next.onToggleGroup) return false;
  if (prev.onSelectTab !== next.onSelectTab) return false;
  if (prev.onQueryChange !== next.onQueryChange) return false;
  if (prev.onSearch !== next.onSearch) return false;
  if (prev.onOpenChannelsEditor !== next.onOpenChannelsEditor) return false;
  if (prev.onOpenSettings !== next.onOpenSettings) return false;
  if (prev.onOpenWorkspace !== next.onOpenWorkspace) return false;
  return true;
}

export default memo(ChannelSidebarBase, propsAreEqual);
``