# R3V

[English](README.md) | **繁體中文**

給創作專案用的版本管理與團隊協作工具：以 Ableton Live 為主，Unity、Unreal、Godot 專案測試中。

> **開發中（WIP）。** R3V 目前是 Windows 上的早期預覽版，已在 Ableton Live 12 上測試。1.0 之前可能還有粗糙之處與變動，重要的專案請自行另外備份。

## 為什麼用 R3V

**為創作者設計，不是為工程師。** 在 Live 裡照常按 Ctrl+S，R3V 會一軌一軌列出改了什麼。用一句話提交版本、回到任何一個版本、撤銷某個版本，或在分支上試新點子。隊友同時改同一首歌時，R3V 會一軌一軌合併，只有兩個人改到同一軌時才會問你。不需要懂 Git。

**取樣自動跟著走。** 硬碟上任何地方的取樣都會跟著版本保存，並在隊友的電腦上重新連結。大檔案（stems、影片）會切塊保存，小改動不必整個重傳；大檔案也會在你提交前先在背景上傳。

**開源，儲存空間屬於你。** R3V 免費，以 Apache-2.0 授權。團隊的作品放在你自己的 S3 相容儲存空間（Cloudflare R2、Amazon S3、MinIO…），不在我們的伺服器上。小團隊通常在 Cloudflare R2 的免費額度內，也不需要有一台電腦一直開著。

## 限制

- **目前只支援 Windows**（macOS 在計畫中）。Unity、Unreal、Godot 專案在測試期間放在 Nightly 頻道（設定 › 更新）。
- **不會同步外掛。** R3V 不會複製外掛，也無法收集外掛從專案外載入的取樣（例如 Kontakt 或 Serum 裡的）。如果隊友沒有同樣的外掛，分享前請先 freeze 那些音軌，或使用 Live 內建的裝置，它們能完整同步。
- **拿到 connection code 的人都有完整權限。** 這組代碼包含儲存空間的金鑰：持有的人可以讀取、修改、刪除團隊所有作品。請私下傳給信任的人。如果外流，請建立新的金鑰並傳送新的代碼。

## 開始使用

到 [Releases](../../releases) 頁面下載 `R3V-<version>-setup.exe` 並執行（Windows 10 21H2 以上或 Windows 11，不需要系統管理員權限）。安裝檔還沒有程式碼簽章：如果 Windows 顯示「Windows 已保護您的電腦」，請按 **其他資訊 → 仍要執行**。之後 R3V 會自動更新。

每個專案都放在團隊的儲存空間裡，硬碟壞了版本也還在。**一個人用？** 建立只有自己的團隊就好。

**建立團隊（一個人做）：** 選 **Create a team**，照步驟建立 Cloudflare R2 的 bucket 和金鑰（約 5 分鐘），或填入其他 S3 相容的儲存空間。R3V 確認後會給你一組 **connection code**，傳給隊友。

**加入團隊：** 選 **Join a team**，貼上收到的 connection code，然後下載團隊的專案或加入你自己的。

詳細步驟和日常使用請看 [團隊設定指南](docs/team-setup.md)。

## 更多

- [團隊設定指南](docs/team-setup.md)
- [命令列工具](docs/cli.md)
- [給 AI agent 的說明](docs/agents.md)（Claude Code、Codex、Cursor…）
- [建置與開發](docs/development.md)

歡迎在 [Issues](../../issues) 回報問題和提供意見。

## 怎麼開發的

R3V 是在維護者的主導與審查下，借助 AI 程式助手（Claude）開發的：每一個變更都由人閱讀、試用並決定。Commit 由維護者提交，不另外加上 AI 共同作者的標記；這段說明就是對所有 commit 的一次性聲明。

因為 R3V 保管的是大家的作品，所以靠檢查而不是靠信任：

- Live set 的合併與差異比對以 golden 檔案固定；儲存格式（內容 hash、切塊邊界、版本紀錄）有 golden 測試，不會以舊版讀不到的方式改變。
- 端對端測試用真實團隊對真實雲端儲存操作（提交、更新、衝突、中斷的上傳、還原）。
- 程式開著專案時不會改寫專案檔案，也不會刪掉隊友的作品：更新會保留你未提交的變更，儲存空間清理和還原只會處理沒有版本用到的東西或遺失的東西。

請參考 [建置與開發](docs/development.md)、[docs/design](docs/design) 裡的設計說明，以及 [UX 原則](docs/ux-principles.md)。

## 授權

[Apache-2.0](LICENSE)。請參考 [NOTICE](NOTICE)。
