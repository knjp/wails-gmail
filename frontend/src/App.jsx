import {useState, useEffect, useRef} from 'react';
import './App.css';
import { api } from './api';

function App() {
    const [messages, setMessages] = useState([]);
    const [tabs, setTabs] = useState([]);
    const [activeTab, setActiveTab] = useState("All");
    const [selectedMsg, setSelectedMsg] = useState(null);
    const [fullBody, setFullBody] = useState("");
    const [loadingBody, setLoadingBody] = useState(false);
    const [loading, setLoading] = useState(false);
    const [nextPageToken, setNextPageToken] = useState("");
    const [query, setQuery] = useState("");
    const [summary, setSummary] = useState("")
    //const [results, setResults] = useState([]);
    const [relatedMsgs, setRelatedMsgs] = useState([])
    const [isSummarizing, setIsSummarizing] = useState(false);
    const requestRef = useRef(0); // 🌟 リクエストの通し番号を記録する
    const [myAddress, setMyAddress] = useState("");
    const [showAuthModal, setShowAuthModal] = useState(false);
    const [authURL, setAuthURL] = useState("");
    const [inputCode, setInputCode] = useState("");


    const handleManualSummarize = async () => {
        setIsSummarizing(true);
        const sum = await api.summarizeEmail(selectedMsg.id);
        setSummary(sum);
        setIsSummarizing(false);
    };

    const handleLoadMore = async () => {
        setLoading(true);
        // Goを呼び出して、次のトークンを受け取る
        const token = await api.syncHistoricalMessages(nextPageToken);
        setNextPageToken(token);

        // 表示を更新
        //const data = await GetMessagesByChannel(activeTab);
        const data = await api.getMessages(activeTab);
        setMessages(data);
        setLoading(false);
    };

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

    const handleDelete = async (msg) => {
        if (!window.confirm(`「${msg.subject}」をゴミ箱に移動しますか？`)) return;
        try {
            await api.trashMessage(msg.id);
            // 成功したら、現在のリストからそのメールを消す（再読み込み不要の爆速UI）
            setMessages(prev => prev.filter(m => m.id !== msg.id));
            setSelectedMsg(null);
        } catch (err) {
            alert("削除に失敗しました: " + err);
        }
    };

    const getDaysLeft = (deadline) => {
        if (!deadline || deadline === "なし") return null;
        const today = new Date();
        const target = new Date(deadline);
        const diffTime = target - today;
        const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
        return diffDays;
    };

    const loadChannels = async (retryCount = 0) => {
        try {
            const res = await api.getChannels();
            if((!res || res.length === 0) && retryCount < 20){
                console.log("Channels are not ready! Retry ...");
                setTimeout(() => loadChannels(retryCount + 1), 5000);
                return;
            }
            if (res) setTabs(res.map(c => c.name));
        } catch(err) {
            console.error("Read Error:", err);
        }
    };

    // チャンネル再読み込み関数
    const handleReloadChannels = async () => {
        try {
            console.log("♻️ チャンネル設定を再読み込み中...");
            await api.loadChannelsFromJson(); // Go側の関数を呼ぶ
            await loadChannels();        // React側のステート（tabs）を更新
            alert("チャンネル設定を更新しました！");
        } catch (err) {
            console.error("リロード失敗:", err);
        }
    };

    const handleManualImportance = async (level) => {
        if (!selectedMsg) return;
    
        try {
            // 1. Go側の関数を呼び出してDBを更新
            // ※ Go側で a.SetManualImportance(id, level) を定義済みである前提です
            await api.setManualImportance(selectedMsg.id, level);
    
            // 2. 現在表示中のメール情報を更新（これでボタンの「active」色が変わります）
            setSelectedMsg({
                ...selectedMsg,
                importance: level
            });
    
            // 3. 左側のリスト（messages）の中の該当メールも更新して、バッジの色などを即座に変える
            setMessages(prev => prev.map(m => 
                m.id === selectedMsg.id ? { ...m, importance: level } : m
            ));
    
            console.log(`✅ 重要度を ${level} に変更しました`);
        } catch (err) {
            console.error("重要度の更新に失敗:", err);
        }
    };

    useEffect(() => {
        const handleMessage = (event) => {
            if (event.data.type === 'open_url') {
                console.log("外部ブラウザで開きます:", event.data.url);
                api.openExternal(event.data.url); // 直接Wailsのランタイムを呼ぶ
            }
        };
        window.addEventListener('message', handleMessage);

        const initApp = async () => {
            try {
                // 1. まず「設定（MyAddressなど）」を読み込む
                const cfg = await api.getConfig();
                setMyAddress(cfg.my_address);

                const channelList = await api.getChannels();
                setTabs(channelList);

                // 2. 🌟 認証が必要かチェックする 🌟
                // Go側の getClient 等を呼び出して token.json があるか確認
                const authURL = await api.getAuthURL(); 
                if (authURL) {
                    // URLが返ってきたら「認証が必要」なのでモーダルを出す
                    setAuthURL(authURL);
                    setShowAuthModal(true);
                } else {
                    // すでに認証済みなら、そのままメール取得などを開始
                    api.loadChannelsFromJson();
                }
            } catch (err) {
                console.error("初期化エラー:", err);
                // エラー時も api 経由で取得を試みる
                const url = await api.getAuthURL().catch(() => "");
                if(url){
                    setAuthURL(url);
                    setShowAuthModal(true);
                }
            }
        };
        initApp();

        return () => window.removeEventListener('message', handleMessage);
    }, []);


    useEffect(() => {
        const currentRequestId = ++requestRef.current; // このリクエストに番号を振る
    
        const loadData = async () => {
            // 1. まず現在のDBからデータを出す（爆速表示）
            const data = await api.getMessages(activeTab);
            
            // 🌟 チェック：もし別のタブが既にクリックされていたら、この結果は捨てる
            if (currentRequestId !== requestRef.current) return;
            setMessages(data || []);
    
            // 2. バックグラウンドで同期を実行
            try {
                await api.syncMessages();
                
                // 🌟 チェック：同期が終わった時、まだ同じタブにいるか？
                if (currentRequestId !== requestRef.current) return;
                
                const freshData = await api.getMessages(activeTab);
                setMessages(freshData || []);
            } catch (err) {
                console.error("同期エラー:", err);
            }
        };
    
        loadData();
    }, [activeTab]);

    useEffect(() => {
        // 🌟 モーダルが閉じられ（false）、かつ認証が完了しているはずの時
        if (!showAuthModal && myAddress) {
            console.log("🔓 認証完了！アプリを始動します...");
            const startApp = async () => {
                await loadChannels(); // チャンネル一覧を取得
            }
            startApp();
        }
    }, [showAuthModal]); // 🌟 showAuthModal の変化を監視

    const handleSelect = async (msg) => {
        if (loadingBody) return;
    
        setSelectedMsg(msg);
        setFullBody("読み込み中...");
        setRelatedMsgs([]);
        setSummary("");
        setLoadingBody(true);
    
        // --- 1. 【爆速】手元のスニペットで関連検索を即座に開始 ---
        // 要約を待たないので、クリックした瞬間に右ペインが埋まり始めます
        api.getAISearchResults(msg.snippet).then(related => {
            if (related) {
                setRelatedMsgs(related.filter(r => r.id !== msg.id));
            }
        }).catch(err => console.error("関連検索エラー:", err));
    
        try {
            // --- 2. 本文取得 ---
            const body = await a.getMessageBody(msg.id);
            setFullBody(body);
    
        } catch (err) {
            console.error("本文取得エラー:", err);
            setFullBody("エラーが発生しました。");
        } finally {
            setLoadingBody(false);
        }

        setMessages(prev => prev.map(m =>
            m.id === msg.id ? { ...m, is_read: 1 } : m
        ))
        api.markAsRead(msg.id);
        /*
        setTimeout(async () => {
            const data = await GetMessagesByChannel(activeTab);
            setMessages(data || []);
        }, 500);
        */
    };

    //
    // メッセージリストを日付順に整理
    //
    const renderMessageList = () => {
        let lastGroup = ""; // 直前のグループを記憶

        const now = new Date();
        const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();

        return messages.map((m) => {
            const msgDate = new Date(m.timestamp);
            const msgTime = msgDate.getTime();
            console.log(`[DEBUG] 件名: ${m.subject} / 未読フラグ: ${m.is_read} / 型: ${typeof m.is_read}`);

            let currentGroup = "";
            if (msgTime >= todayStart) {
                currentGroup = "今日";
            } else if (msgTime >= todayStart - (7 * 24 * 60 * 60 * 1000)) {
                currentGroup = "1週間以内";
            } else if (msgTime >= todayStart - (30 * 24 * 60 * 60 * 1000)) {
                currentGroup = "1ヶ月以内";
            } else {
                currentGroup = "それ以前";
            }
    
            const displayDate = msgDate.toLocaleString('ja-JP');
            // --- グループが変わった時だけセパレーターを出す ---
            const showSeparator = currentGroup !== lastGroup;
            lastGroup = currentGroup;

            const isDirect = m.recipient && m.recipient.includes(myAddress);
            const isML = m.recipient && !isDirect; // 自分宛でなければML（またはCC）とみなす

            return (
                <div key={m.id}>
                    {showSeparator && (
                        <div className="list-separator">{currentGroup}</div>
                    )}
                    <div
                        className={`mail-item
                            ${selectedMsg?.id === m.id ? 'selected' : ''}
                            ${m.is_read === 0 ? 'unread-item' : ''}
                            importance-${m.importance}`}
                        onClick={() => handleSelect(m)}
                    >
                        <div className="subject">
                            {/* 🌟 宛先バッジを追加 🌟 */}
                            {isDirect ? (
                                <span className="recipient-badge direct">TO ME</span>
                            ) : isML ? (
                                <span className="recipient-badge ml">ML</span>
                            ) : null}

                            {m.subject}
                            {m.importance >= 4 && (
                                <span className={`importance-badge level-${m.importance}`}>
                                    {m.importance === 5 ? "🔥 CRITICAL" : "⚡ IMPORTANT"}
                                </span>
                            )}
                        </div>
                        <div className='list-snippet'> {m.snippet} </div>
                        <div className="from">{m.from}</div>
                        <div className="mail-date">{displayDate}</div>
                    </div>
                </div>
            );
        });
    };

    const daysLeft = selectedMsg ? getDaysLeft(selectedMsg.deadline) : null;
    const isDirect = selectedMsg?.recipient?.includes(myAddress);


    return (
        <div className="container">
            {showAuthModal && (
                <div className="auth-overlay">
                    <div className="auth-card">
                         {authURL === "MISSING_CREDENTIALS" ? (
                            <div className="error-state">
                                <h2>📁 credentials.json が必要です</h2>
                                <p>Google Cloud Console で「デスクトップアプリ」用の認証情報を作成し、JSONをダウンロードしてください。</p>
                                <div className="action-buttons">
                                <button 
                                    className="console-link-btn" 
                                    onClick={() => api.openExternal("https://console.cloud.google.com")}
                                >
                                🌐 Google Cloud Console を開く
                                </button>
                                <button className="retry-btn" onClick={() => window.location.reload()}>
                                    🔄 ファイルを置いたので再読み込み
                                </button>
                                </div>
                                <p className="path-hint">保存先: <code>config/credentials.json</code></p>
                            </div>
                        ) : (
                            <div className="auth-steps">
                                <h2>🔑 Google ログイン</h2>
                                <p>アプリを使用するために認証が必要です。</p>
                                <button onClick={() => api.openExternal(authURL)}>ブラウザを開いて承認</button>
                                <input 
                                    placeholder="表示されたコードを入力" 
                                    value={inputCode} 
                                    onChange={e => setInputCode(e.target.value)} 
                                />
                                <button onClick={async () => {
                                    await api.completeAuth(inputCode);
                                    setShowAuthModal(false);
                                    window.location.reload(); // 🌟 再起動してメール取得開始
                                }}>認証を完了する</button>
                            </div>
                        )}
                    </div>
                </div>
            )}
            <div className="main-layout">

                {/* 左端：チャンネルリスト（旧タブバー） */}
                <div className="channel-sidebar">

                    {/* 検索エリア */}
                    <div className="search-bar">
                        <input 
                            type="text" 
                            placeholder="AIであいまい検索..." 
                            value={query}
                            onChange={(e) => setQuery(e.target.value)}
                            onKeyDown={(e) => e.key === 'Enter' && handleAISearch(e.target.value)}
                        />
                        <button onClick={handleAISearch}>検索</button>
                    </div>

                    <div className="sidebar-header">CHANNELS</div>
                        {/* 🌟 再読み込みボタン 🌟 */}
                    <button onClick={handleReloadChannels} className="reload-channels-btn">
                        🔄 設定を反映
                    </button>

                    {tabs.map(name => (
                        <div 
                            key={name} 
                            className={`channel-item ${activeTab === name ? 'active' : ''}`}
                            onClick={() => setActiveTab(name)}
                        >
                            # {name}
                        </div>
                    ))}
                </div>

                {/* 中央：メールリスト */}
                <div className="mail-list-pane">
                    <div className="pane-header">{activeTab}</div>
                    <div className="list-container">
                        {messages.length === 0 && <div className="info">メールがありません</div>}

                        { renderMessageList() }

                        {messages.length>0 && (
                            <button onClick={handleLoadMore} disabled={loading} className="load-more">
                                {loading ? "読み込み中・・・" : "さらに500件読み込む"}
                            </button>
                        )}
                    </div>
                </div>

                <div className="main-content">
                    {selectedMsg ? (
                        <div className={`email-view 
                            ${selectedMsg.is_read === 0 ? 'unread-view' : ''} 
                            ${!isDirect ? 'reference-view' : ''}`}>

                            {/* 1. ヘッダー：件名と基本情報 */}
                            <div className="email-header-top">
                                <div className="header-main">
                                    {!isDirect && <span className="ref-badge">参考情報</span>}
                                    <h2 className="detail-subject">{selectedMsg.subject}</h2>
                                    <div className="detail-meta">
                                        <div className="meta-row-meta">
                                            <span className="meta-label">From:</span>
                                            <span className="detail-from">{selectedMsg.from}</span>
                                        </div>
                                        <div className="meta-row">
                                            <span className="meta-label">To:</span>
                                            <span className="detail-to">{selectedMsg.recipient || "（宛先なし）"}</span>
                                        </div>
                                        <span className="detail-date">
                                            📅 {new Date(selectedMsg.timestamp).toLocaleString('ja-JP')}
                                        </span>
                                    </div>
                                </div>
                                

<div className="header-actions-container">
    {/* 上段：メインアクション */}
    <div className="main-actions">
        <button onClick={handleManualSummarize} disabled={isSummarizing} className="summary-btn">
            {isSummarizing ? "⌛ 要約中..." : "✨ AI要約"}
        </button>
        <button onClick={() => handleDelete(selectedMsg)} className="delete-btn">
            🗑️
        </button>
    </div>

    {/* 下段：重要度ピッカー */}
    <div className="importance-picker-row">
        <span className="picker-label">重要度</span>
        <div className="imp-button-group">
            {[1, 2, 3, 4, 5].map(num => (
                <button 
                    key={num}
                    className={`imp-num-btn ${selectedMsg.importance === num ? 'active' : ''}`}
                    onClick={() => handleManualImportance(num)}
                >
                    {num}
                </button>
            ))}
        </div>
    </div>
</div>

                            </div>

                            {/* 3. AI インフォメーション（期限と要約） */}
                            {(daysLeft !== null || summary) && (
                                <div className="ai-info-section">
                                    {daysLeft !== null && (
                                        <div className={`deadline-banner ${daysLeft < 0 ? 'overdue' : daysLeft <= 3 ? 'urgent' : ''}`}>
                                            <span className="icon">📅</span>
                                            <span className="text">
                                                {daysLeft < 0 ? `期限切れ (${Math.abs(daysLeft)}日経過)` : 
                                                 daysLeft === 0 ? "本日締切！" : 
                                                 `${selectedMsg.deadline} まであと ${daysLeft} 日`}
                                            </span>
                                        </div>
                                    )}
                                    {summary && <div className="ai-summary-content">{summary}</div>}
                                </div>
                            )}
                
                            {/* 4. 本文 */}
                            <div className="email-body-container">
                                <iframe
                                    key={selectedMsg.id}
                                    title="body"
                                    className="email-body-frame"
                                    srcDoc={fullBody} 
                                    sandbox="allow-popups allow-popups-to-escape-sandbox allow-scripts" // セキュリティとポップアップ許可
                                />
                            </div>
                        </div>
                    ) : <div className="empty-state">メールを選択してください</div>}
                </div>

                {/* 🌟 4つ目のペイン：関連コンテキスト 🌟 */}
                <div className="related-pane">
                    <div className="pane-header">🔗 関連・過去の経緯</div>
                    <div className="related-list-container">
                        {relatedMsgs.length === 0 && <div className="info">関連なし</div>}
                        {relatedMsgs.map(rm => (
                            <div key={rm.id} className="mail-item related-item" onClick={() => handleSelect(rm)}>
                                <div className="subject-small">{rm.subject}</div>
                                <div className='list-snippet'> {rm.snippet} </div>
                                <div className="from">{rm.from}</div>
                                <div className="mail-date">{/*displayDate*/}Time </div>
                                <div className="date-small">{new Date(rm.timestamp).toLocaleDateString()}</div>
                            </div>
                        ))}
                    </div>
                </div>

            </div>
        </div>
    );
}

export default App;
