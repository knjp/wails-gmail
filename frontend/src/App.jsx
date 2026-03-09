import {useState, useEffect, useCallback, useMemo, useRef, useLayoutEffect} from 'react';
import './App.css';
import { api } from './api';
import MessageList from "./components/mailList/MessageList";
import { getDaysLeft } from "./utils/dates";
import EmailView from "./components/emailView/EmailView";
import { useAuth } from "./hooks/useAuth";
import { useWorkspaces } from "./hooks/useWorkspaces";
import ChannelSidebar from "./components/sidebar/ChannelSidebar";
import { useMessages } from "./hooks/useMessages";
import { useMessageBody } from "./hooks/useMessageBody";
import AuthModal from "./components/modals/AuthModal";
import JsonEditorModal from "./components/modals/JsonEditorModal";
import SettingsEditorModal from "./components/modals/SettingsEditorModal";
import RelatedPane from "./components/related/RelatedPane";
import ResizablePane from "./components/common/ResizablePane";
import ChannelOverlayPane from "./components/sidebar/ChannelOverlayPane";


function App() {
    const [activeTab, setActiveTab] = useState("All");
    const [selectedMsg, setSelectedMsg] = useState(null);
    const [query, setQuery] = useState("");
    const [summary, setSummary] = useState("")
    const [isSummarizing, setIsSummarizing] = useState(false);
    const { messages, loading, loadMore, updateOne, removeOne, setMessages } = useMessages(activeTab);
    const { fullBody, loadingBody, relatedMsgs, threadMsgs, selectMessage} = useMessageBody();
    const {
        myAddress,
        showAuthModal, setShowAuthModal,
        authURL,
        inputCode, setInputCode,
        completeAuth,
    } = useAuth();
    const [showJsonEditor, setShowJsonEditor] = useState(false);
    const [rawJson, setRawJson] = useState("");
    const [showSettingsEditor, setShowSettingsEditor] = useState(false);
    const [rawSettings, setRawSettings] = useState("");
    const [activeGroup, setActiveGroup] = useState(null); // 現在開いているグループ名
    const [showRelated, setShowRelated] = useState(false);
    const toggleRelated = useCallback(() => setShowRelated(v => !v), []);
    const [listWidth, setListWidth] = useState(350);
    const [useOverlay, setUseOverlay] = useState(true);
    const [overlayWorkspace, setOverlayWorkspace] = useState(null);
    const sidebarRef = useRef(null);
    const [sidebarRect, setSidebarRect] = useState({ left:0, top: 0 , height: 0});
    const [sidebarWidth, setSidebarWidth] = useState(240);

    const handleManualSummarize = useCallback(async () => {
        if (!selectedMsg) return;
        setIsSummarizing(true);
        const sum = await api.summarizeEmail(selectedMsg.id);
        setSummary(sum);
        setIsSummarizing(false);
    }, [selectedMsg]);

    const handleAISearch = async () => {
        console.log("AI Searching!! for:", query)
        try {
            const results = await api.getAISearchResults(query);
            console.log("Search Results:", results); // ここで中身を確認！

            if(results && results.length > 0){
                setMessages(results);
                setActiveTab("🔍 検索結果");
            } else {
                alert("該当するメールが見つかりませんでした。");
            }
        } catch (err) {
            console.error("検索失敗:", err);
        }
    };

    const handleSelect = useCallback (
        async (msg) => {
            setSelectedMsg(msg);
            setSummary("");
            await selectMessage(msg, { onMarkRead: (id) => updateOne(id, { is_read: 1})});
        },
        [selectMessage, updateOne]
    );

    const handleDelete = useCallback (async (msg) => {
        if (!window.confirm(`「${msg.subject}」をゴミ箱に移動しますか？`)) return;
        try {
            await api.trashMessage(msg.id);
            // 成功したら、現在のリストからそのメールを消す
            removeOne(msg.id);
            setSelectedMsg(null);
        } catch (err) {
            alert("削除に失敗しました: " + err);
        }
    }, []);

    const authReady = !showAuthModal && !!myAddress;
    const { workspaces, reloadFromJson } = useWorkspaces({
        enabled: authReady,
        onPickInitial: setActiveTab,
    })

    const handleManualImportance = useCallback(async (level) => {
        if (!selectedMsg) return;
        try {
            await api.setManualImportance(selectedMsg.id, level);
            setSelectedMsg({
                ...selectedMsg,
                importance: level
            });
            updateOne(selectedMsg.id, { importance: level });
            console.log(`✅ 重要度を ${level} に変更しました`);
        } catch (err) {
            console.error("重要度の更新に失敗:", err);
        }
    }, [selectedMsg]);

    // 🌟 編集開始ボタンの処理
    const handleOpenChannelsEditor = async () => {
        try {
            const text = await api.getChannelsRaw();
            setRawJson(text);
            setShowJsonEditor(true);
        } catch (err) {
            alert("設定の読み込みに失敗しました: " + err.message);
        }
    };
    
    // 🌟 保存ボタンの処理
    const handleSaveChannels = async () => {
        try {
            await api.saveChannelsRaw(rawJson);
            setShowJsonEditor(false);
            // 🌟 保存後にサイドバーを最新化する（以前作った関数）
            await reloadFromJson(); 
            alert("✅ 設定を保存し、反映しました！ New");
        } catch (err) {
            alert("❌ 保存失敗: " + err.message);
        }
    };

    const handleOpenSettings = async () => {
        try {
            const text = await api.getSettingsRaw(); // 🌟 api.js に追加したやつ
            setRawSettings(text);
            setShowSettingsEditor(true);
        } catch (err) {
            alert("設定の取得に失敗: " + err.message);
        }
    };

    const handleSaveSettings = async () => {
        try {
            await api.saveSettingsRaw(rawSettings); // 🌟 api.js に追加したやつ
            setShowSettingsEditor(false);
            alert("✅ 設定を更新しました。再起動なしで反映されます！");
        } catch (err) {
            alert("❌ 保存失敗: " + err.message);
        }
    };

    const handleToggleGroup = useCallback(
      (next) => setActiveGroup(next),
      [setActiveGroup]
    );
    const handleSelectTab = useCallback(
      (tab) => setActiveTab(tab),
      [setActiveTab]
    );
    const handleQueryChange = useCallback(
      (text) => setQuery(text),
      [setQuery]
    );

    const handleSearch = useCallback(() => handleAISearch(), [handleAISearch]);
    const openChannelsEditor = useCallback(() => handleOpenChannelsEditor(), [handleOpenChannelsEditor]);
    const openSettings = useCallback(() => handleOpenSettings(), [handleOpenSettings]);

    useLayoutEffect(() => {
        if (!sidebarRef.current) return;
        const el = sidebarRef.current;
        const measure = () => {
          const r = el.getBoundingClientRect();
          setSidebarRect({ left: r.left, top: r.top, width: r.width, height: r.height });
        };
        measure();

        const ro = new ResizeObserver(measure);
        ro.observe(el);

        window.addEventListener("resize", measure);
        window.addEventListener("scroll", measure, true); // 内部スクロールにも強めに

        return () => {
          ro.disconnect();
          window.removeEventListener("resize", measure);
          window.removeEventListener("scroll", measure, true);
        };
    }, []);

    const handleOpenWorkspace = useCallback((group) => setOverlayWorkspace(group), []);

    useEffect(() => {
        const handleMessage = (event) => {
            if (event.data.type === 'open_url'){
                console.log("外部ブラウザで開きます:", event.data.url);
                api.openExternal(event.data.url);
            }
        };
        window.addEventListener('message', handleMessage);
        return () => window.removeEventListener('message', handleMessage);
    }, []);

    const daysLeft = useMemo(
        () => (selectedMsg ? getDaysLeft(selectedMsg.deadline) : null),
            [selectedMsg]
        );
    const isDirect = selectedMsg?.recipient?.includes(myAddress);

    return (
        <div className="container">
            {showAuthModal && (
                <AuthModal
                    authURL={authURL}
                    inputCode={inputCode}
                    onChangeCode={setInputCode}
                    onOpenExternal={(url) => api.openExternal(url)}
                    onComplete={completeAuth}
                    onClose={() => setShowAuthModal(false)}
                />
            )}
            {showJsonEditor && (
                <JsonEditorModal
                    title="⚙️ チャンネル設定 (JSON 編集)"
                    value={rawJson}
                    onChange={setRawJson}
                    onSave={async () => {
                        await handleSaveChannels();
                        await reloadFromJson();
                    }}
                    onClose={ () => setShowJsonEditor(false)}
                />
            )}
            {showSettingsEditor && (
                <SettingsEditorModal
                    title="🔧 アプリ基本設定 (settings.json)"
                    value={rawSettings}
                    onChange={setRawSettings}
                    onSave={handleSaveSettings}
                    onClose={() => setShowSettingsEditor(false)}
                />
            )}

            <div className="main-layout">
                <ResizablePane
                    width={sidebarWidth}
                    min={180}
                    max={420}
                    onResize={setSidebarWidth}
                    persistKey="ui:sidebarPaneWidth"
                    className="eptive flex-none h-full"
                    handleClassName="absolute top-0 right-0 h-full w-[10px] bg-slate-300/70 hover:bg-slate-400 cursor-col-resize transition-colors"
                >
                <div ref={sidebarRef} className="h-full">
                <ChannelSidebar
                    workspaces={workspaces}
                    activeGroup={activeGroup}
                    activeTab={activeTab}
                    onOpenWorkspace={useOverlay? setOverlayWorkspace: undefined}
                    onToggleGroup={handleToggleGroup}
                    onSelectTab={handleSelectTab}
                    query={query}
                    onQueryChange={handleQueryChange}
                    onSearch={handleSearch}
                    onOpenChannelsEditor={openChannelsEditor}
                    onOpenSettings={openSettings}
                />
                </div>
                </ResizablePane>

                {overlayWorkspace && (
                    <ChannelOverlayPane
                        group={overlayWorkspace}
                        anchorLeft={sidebarRect.left}
                        anchorTop={sidebarRect.top}
                        anchorWidth={sidebarRect.width}
                        onSelectChannel={(ch) => setActiveTab(ch)}
                        onClose={() => setOverlayWorkspace(null)}
                        autoCloseOnSelect={false}
                        modal={false}
                        panelWidth={240}
                        panelClassName="w-[220px] sm:w-[260px]"
                    />
                )}

                <ResizablePane
                    width={listWidth}
                    min={240}
                    max={600}
                    onResize={setListWidth}
                    onResizeEnd={(w) => console.log("final width:", w)}
                    persistKey="ui:mailListPaneWidth"
                    className="mail-list-pane"
                    handleClassName="absolute top-0 right-0 h-full w-[10px] bg-slate-300/80 hover:bg-slate-400 cursor-col-resize transition-colors select-none"
                >
                    <div className="pane-header">{activeTab}</div>
                    <div className="list-container">
                        {/* 中央：メールリスト */}
                        {messages.length === 0 && <div className="info">メールがありません</div>}
                        <MessageList
                            messages={messages}
                            myAddress={myAddress}
                            selectedId={selectedMsg?.id}
                            onSelect={handleSelect}
                        />
                        {messages.length>0 && (
                            <button onClick={loadMore} disabled={loading} className="load-more">
                                {loading ? "読み込み中・・・" : "さらに500件読み込む"}
                            </button>
                        )}
                    </div>
                </ResizablePane>

                <div className="main-content">
                    <EmailView
                        selectedMsg={selectedMsg}
                        isDirect={isDirect}
                        summary={summary}
                        isSummarizing={isSummarizing}
                        onSummarize={handleManualSummarize}
                        onDelete={handleDelete}
                        onChangeImportance={handleManualImportance}
                        daysLeft={daysLeft}
                        fullBody={fullBody}
                        loadingBody={loadingBody}
                        onToggleRelated={toggleRelated}
                        showRelated={showRelated}
                    />
                </div>

                {showRelated && (
                    <RelatedPane threadMsgs={threadMsgs} aiMsgs={relatedMsgs} onSelect={handleSelect} />
                )}
            </div>
        </div>
    );
}

export default App;