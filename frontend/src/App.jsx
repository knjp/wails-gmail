import {useState, useEffect, useCallback, useMemo} from 'react';
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


function App() {
    const [activeTab, setActiveTab] = useState("All");
    const [selectedMsg, setSelectedMsg] = useState(null);
    const [query, setQuery] = useState("");
    const [summary, setSummary] = useState("")
    const [isSummarizing, setIsSummarizing] = useState(false);
    const { messages, loading, loadMore, updateOne, removeOne, setMessages } = useMessages(activeTab);
    const { fullBody, loadingBody, relatedMsgs, selectMessage} = useMessageBody();
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
                <ChannelSidebar
                    workspaces={workspaces}
                    activeGroup={activeGroup}
                    activeTab={activeTab}
                    onToggleGroup={handleToggleGroup}
                    onSelectTab={handleSelectTab}
                    query={query}
                    onQueryChange={handleQueryChange}
                    onSearch={handleSearch}
                    onOpenChannelsEditor={openChannelsEditor}
                    onOpenSettings={openSettings}
                />

                <div
                    className="mail-list-pane relative"
                    style={{ width: `${listWidth}px` }}
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

                    <input
                        type="range"
                        min={240}
                        max={600}
                        step={2}
                        value={listWidth}
                        onInput={(e) => setListWidth(Number(e.currentTarget.value))}
                        className="absolute top-0 right-0 h-full w-[10px] cursor-col-resize opacity-0"
                        style={{
                            writingMode: 'bt-lr',
                            WebkitAppearance: 'slider-horizontal'
                        }}
                        aria-label="メール一覧ペイン幅"
                    />
                    <div
                        className="absolute top-0 right-0 h-full w-[8px] bg-slate-300 hover:bg-slate-400 transition-colors pointer-events-none"
                        aria-hidden
                    />
                </div>
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
                    <RelatedPane relatedMsgs={relatedMsgs} onSelect={handleSelect} />
                )}
            </div>
        </div>
    );
}

export default App;