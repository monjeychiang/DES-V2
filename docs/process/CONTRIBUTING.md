# 貢獻指南

感謝你對 DES Trading System V2.0 的興趣! 我們歡迎各種形式的貢獻。

## 🤝 如何貢獻

### 回報問題 (Bug Reports)

如果你發現了 bug,請建立一個 Issue 並包含:

- **清楚的標題** - 簡短描述問題
- **重現步驟** - 詳細說明如何重現問題
- **預期行為** - 你期望發生什麼
- **實際行為** - 實際發生了什麼
- **環境資訊** - OS、Go/Python/Node 版本等
- **相關日誌** - 如果有的話

### 功能請求 (Feature Requests)

如果你有新功能的想法:

1. 先檢查 Issues 是否已有類似建議
2. 建立新 Issue 並標記為 `enhancement`
3. 詳細說明:
   - 功能描述
   - 使用場景
   - 可能的實作方式
   - 是否願意協助實作

### 提交程式碼 (Pull Requests)

#### 開發流程

1. **Fork 專案**
   ```bash
   # 在 GitHub 上點擊 Fork 按鈕
   git clone https://github.com/your-username/DES-V2.git
   cd DES-V2
   ```

2. **建立分支**
   ```bash
   git checkout -b feature/your-feature-name
   # 或
   git checkout -b fix/bug-description
   ```

3. **開發與測試**
   - 遵循現有的程式碼風格
   - 新增必要的測試
   - 確保所有測試通過
   - 更新相關文件

4. **提交變更**
   ```bash
   git add .
   git commit -m "feat: add amazing feature"
   ```

5. **推送到你的 Fork**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **建立 Pull Request**
   - 在 GitHub 上建立 PR
   - 填寫 PR 模板
   - 等待 review

#### Commit 訊息規範

我們使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Type:**
- `feat`: 新功能
- `fix`: Bug 修復
- `docs`: 文件更新
- `style`: 程式碼格式調整 (不影響功能)
- `refactor`: 重構 (不是新功能也不是 bug 修復)
- `perf`: 效能優化
- `test`: 新增或修改測試
- `chore`: 建置工具或輔助工具的變動

**範例:**
```
feat(trading): add stop-loss order support

Implement stop-loss order functionality for risk management.
- Add StopLossOrder type
- Integrate with order executor
- Add unit tests

Closes #123
```

## 📝 程式碼規範

### Go

- 遵循 [Effective Go](https://golang.org/doc/effective_go.html)
- 使用 `gofmt` 格式化程式碼
- 使用 `golint` 檢查程式碼品質
- 函數和方法需要有註解
- 保持函數簡短且專注

```bash
# 格式化
go fmt ./...

# 檢查
go vet ./...
golint ./...

# 測試
go test ./...
```

### Python

- 遵循 [PEP 8](https://www.python.org/dev/peps/pep-0008/)
- 使用 `black` 格式化程式碼
- 使用 `pylint` 或 `flake8` 檢查
- 使用 type hints
- 撰寫 docstrings

```bash
# 格式化
black .

# 檢查
pylint **/*.py
flake8 .

# 測試
pytest
```

### JavaScript/React

- 遵循 ESLint 規則
- 使用函數式元件和 Hooks
- 保持元件小而專注
- 撰寫有意義的變數名稱

```bash
# 檢查
npm run lint

# 格式化
npm run format
```

## 🧪 測試

所有新功能和 bug 修復都應該包含測試:

- **Go**: 使用標準 `testing` 套件
- **Python**: 使用 `pytest`
- **Frontend**: 使用 `vitest` 或 `jest`

確保測試覆蓋率不降低。

## 📚 文件

如果你的變更影響到使用方式:

- 更新相關的 README
- 更新 API 文件
- 新增範例程式碼
- 更新 docs/roadmap/CHANGELOG.md

## 🔍 Code Review

所有 PR 都需要經過 review:

- 至少一位維護者的批准
- 所有 CI 檢查通過
- 沒有未解決的討論
- 符合專案的程式碼規範

## ⚖️ 授權

提交程式碼即表示你同意將你的貢獻以 MIT 授權釋出。

## 💬 需要幫助?

- 查看 [DEVELOPER_ONBOARDING.md](DEVELOPER_ONBOARDING.md)
- 查看 [QUICK_REFERENCE.md](../setup/QUICK_REFERENCE.md)
- 在 Issues 中提問
- 加入我們的討論區

## 🙏 感謝

感謝所有貢獻者讓 DES Trading System 變得更好!

---

再次感謝你的貢獻! 🎉
