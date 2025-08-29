# 贡献指南 (Contributing Guide)

感谢您对 Trading Assistant 项目的关注！我们欢迎任何形式的贡献，无论是错误报告、功能建议、代码提交还是文档改进。

## 🎯 贡献方式

### 1. 报告错误 (Bug Reports)
如果您发现了错误，请：
- 检查 [Issues](https://github.com/your-username/trading-assistant/issues) 确认问题未被报告
- 创建新的 Issue 并包含以下信息：
  - 详细的错误描述
  - 复现步骤
  - 预期行为 vs 实际行为
  - 环境信息（操作系统、Go版本、Node版本等）
  - 相关的日志输出

### 2. 功能建议 (Feature Requests)
- 在 [Discussions](https://github.com/your-username/trading-assistant/discussions) 中讨论新功能
- 描述功能的用途和预期收益
- 如果可能，提供设计思路或实现方案

### 3. 代码贡献 (Code Contributions)
我们欢迎以下类型的代码贡献：
- 🐛 修复错误
- ✨ 新功能开发
- 📈 性能优化
- 📚 文档改进
- 🧪 添加测试用例
- 🔧 代码重构

## 🛠️ 开发流程

### 前置要求
- Go 1.21+
- Node.js 16+
- Git
- Redis (用于测试)

### 1. Fork 和克隆
```bash
# 1. Fork 项目到你的 GitHub 账户
# 2. 克隆你的 fork
git clone https://github.com/your-username/trading-assistant.git
cd trading-assistant

# 3. 添加上游仓库
git remote add upstream https://github.com/original-owner/trading-assistant.git
```

### 2. 设置开发环境
```bash
# 安装依赖
make install-deps

# 创建配置文件
cp .env.example .env
# 编辑 .env 文件填入测试配置

# 启动开发环境
make dev
```

### 3. 创建功能分支
```bash
# 获取最新代码
git fetch upstream
git checkout main
git merge upstream/main

# 创建功能分支
git checkout -b feature/your-feature-name
```

### 4. 开发和测试
```bash
# 后端开发
go run main.go

# 前端开发
cd web && npm start

# 运行测试（如有）
go test ./...
cd web && npm test
```

### 5. 提交代码
```bash
# 添加变更
git add .

# 提交（请使用清晰的提交信息）
git commit -m "feat: add new trading strategy feature"

# 推送到你的 fork
git push origin feature/your-feature-name
```

### 6. 创建 Pull Request
- 在 GitHub 上创建 Pull Request
- 填写 PR 模板中的所有必要信息
- 链接相关的 Issues
- 等待代码审查

## 📝 代码规范

### Go 代码规范
- 遵循 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化代码
- 添加必要的注释，特别是公共函数和复杂逻辑
- 错误处理应当完整和优雅
- 使用有意义的变量和函数名

```go
// ✅ 好的示例
func (pm *PriceMonitor) checkSingleEstimate(estimate *models.PriceEstimate) error {
    if estimate == nil {
        return fmt.Errorf("estimate cannot be nil")
    }
    
    // 获取当前市场价格
    currentPrice, err := pm.getCurrentPrice(estimate.Symbol)
    if err != nil {
        return fmt.Errorf("failed to get current price: %w", err)
    }
    
    // 检查是否触发条件
    if pm.shouldTrigger(estimate, currentPrice) {
        return pm.executeOrder(estimate)
    }
    
    return nil
}

// ❌ 不好的示例
func (pm *PriceMonitor) check(e *models.PriceEstimate) {
    p, _ := pm.getPrice(e.Symbol) // 忽略错误
    if p > e.TargetPrice {
        pm.exec(e) // 函数名不清晰
    }
}
```

### JavaScript/React 代码规范
- 遵循 [Airbnb JavaScript Style Guide](https://github.com/airbnb/javascript)
- 使用 ES6+ 语法
- 组件使用函数式组件和 Hooks
- 添加适当的 PropTypes 或 TypeScript 类型
- 使用有意义的变量和函数名

```jsx
// ✅ 好的示例
const TradingPairCard = ({ symbol, price, onChange, isSelected }) => {
  const [loading, setLoading] = useState(false);
  
  const handleSelectionChange = useCallback(async (selected) => {
    setLoading(true);
    try {
      await onChange(symbol, selected);
      message.success(`${symbol} ${selected ? '已选择' : '已取消选择'}`);
    } catch (error) {
      message.error('操作失败');
    } finally {
      setLoading(false);
    }
  }, [symbol, onChange]);

  return (
    <Card title={symbol} loading={loading}>
      <Switch 
        checked={isSelected}
        onChange={handleSelectionChange}
        disabled={loading}
      />
    </Card>
  );
};
```

### 提交信息规范
使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**类型说明:**
- `feat`: 新功能
- `fix`: 错误修复
- `docs`: 文档更新
- `style`: 代码格式（不影响功能）
- `refactor`: 重构（既不是新功能也不是错误修复）
- `perf`: 性能优化
- `test`: 添加测试
- `chore`: 构建过程或辅助工具的变动

**示例:**
```
feat(api): add new price estimation endpoints

Add endpoints for creating and managing price estimations:
- POST /api/v1/estimates
- GET /api/v1/estimates
- DELETE /api/v1/estimates/:id

Closes #123
```

## 🧪 测试指南

### 后端测试
```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./core/

# 运行带覆盖率的测试
go test -cover ./...
```

### 前端测试
```bash
cd web

# 运行单元测试
npm test

# 运行覆盖率测试
npm test -- --coverage
```

### 集成测试
```bash
# 启动测试环境
make dev

# 运行 API 测试
curl -X POST http://localhost:8080/api/v1/coins/sync
```

## 📋 Pull Request 清单

在创建 PR 之前，请确认：

- [ ] 代码遵循项目的代码规范
- [ ] 所有测试通过
- [ ] 添加了必要的测试用例
- [ ] 更新了相关文档
- [ ] 提交信息遵循规范
- [ ] PR 标题清晰描述了变更内容
- [ ] 填写了 PR 模板中的所有必要信息

## 🎖️ 认可贡献者

我们使用 [All Contributors](https://github.com/all-contributors/all-contributors) 来认可所有贡献者。

贡献类型包括但不限于：
- 💻 代码
- 📖 文档
- 🐛 错误报告
- 💡 想法和建议
- 🤔 问答支持
- 🎨 设计
- 🔧 工具和基础设施

## 📞 获得帮助

如果您在贡献过程中遇到问题，可以通过以下方式获得帮助：

- 💬 [GitHub Discussions](https://github.com/your-username/trading-assistant/discussions)
- 📧 Email: your-email@example.com
- 💬 Telegram: [@your_username](https://t.me/your_username)

## 🌟 感谢

感谢所有为项目做出贡献的开发者！您的每一个贡献都让这个项目变得更好。

---

再次感谢您的贡献！让我们一起构建更好的 Trading Assistant！🚀
