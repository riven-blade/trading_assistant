# Trading Assistant - 智能交易助手

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![React](https://img.shields.io/badge/React-18.2+-61DAFB.svg)](https://reactjs.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker](https://img.shields.io/badge/Docker-Supported-blue.svg)](https://www.docker.com)

一个基于 Go 和 React 开发的智能化加密货币交易助手，集成 Binance 期货交易、实时监控、价格预估和自动执行等功能。

![Trading Assistant Dashboard](https://via.placeholder.com/800x400/1890FF/FFFFFF?text=Trading+Assistant+Dashboard)

## ✨ 核心特性

### 🏪 **多交易所支持**
- **Binance 期货交易**：完整的 API 集成，支持开仓、平仓、调整保证金等操作
- **实时数据同步**：WebSocket 连接确保数据实时性
- **测试网支持**：可在测试环境中安全测试策略

### 📊 **智能监控系统**
- **实时价格监控**：WebSocket 监听选中交易对的价格变动
- **订单簿分析**：实时获取和分析市场深度数据
- **持仓跟踪**：自动监控账户持仓和盈亏情况
- **余额管理**：实时追踪账户余额变化

### 🤖 **智能交易执行**
- **价格预估系统**：设置目标价格，自动执行交易策略
- **多种触发条件**：支持到达价格、突破价格等多种触发方式
- **风险控制**：内置余额比例阈值，防止过度交易
- **双向持仓**：支持同时持有多空仓位

### 📱 **Telegram 集成**
- **实时通知**：交易执行、价格预警、系统状态实时推送
- **风险提醒**：余额不足、交易失败等风险事件即时通知
- **状态报告**：定期推送账户状态和交易总结

### 🌐 **现代化 Web 界面**
- **响应式设计**：支持桌面和移动设备
- **实时数据展示**：WebSocket 连接确保界面数据实时更新
- **K线图表**：集成专业级图表展示价格趋势
- **直观操作**：简洁的界面设计，操作便捷

### 🔐 **安全认证**
- **JWT 认证**：基于令牌的安全认证机制
- **角色权限**：支持不同用户角色和权限管理
- **API 保护**：所有敏感操作都需要身份验证

## 🏗️ 技术架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   React 前端    │    │   Go 后端服务   │    │  Binance API    │
│                 │    │                 │    │                 │
│ • Ant Design    │◄──►│ • Gin Framework │◄──►│ • WebSocket     │
│ • WebSocket     │    │ • WebSocket Hub │    │ • REST API      │
│ • Charts        │    │ • Price Monitor │    │ • Real-time     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  用户界面层     │    │   业务逻辑层    │    │   数据存储层    │
│                 │    │                 │    │                 │
│ • 持仓管理      │    │ • 订单执行器    │    │ • Redis Cache   │
│ • 交易对选择    │    │ • 价格监控器    │    │ • 实时数据      │
│ • K线图表       │    │ • 账户管理器    │    │ • 配置存储      │
│ • 余额监控      │    │ • 市场管理器    │    │ • 历史记录      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🚀 快速开始

### 环境要求

- **Go**: 1.21+ 
- **Node.js**: 16+
- **Redis**: 6.0+
- **Binance Account**: 需要 API Key 和 Secret

### 方式一：Docker 部署 (推荐)

```bash
# 1. 克隆项目
git clone https://github.com/your-username/trading-assistant.git
cd trading-assistant

# 2. 复制环境配置
cp .env.example .env

# 3. 编辑配置文件
vi .env

# 4. 一键部署
make docker-deploy
```

### 方式二：本地开发

```bash
# 1. 克隆项目
git clone https://github.com/your-username/trading-assistant.git
cd trading-assistant

# 2. 安装依赖
make install-deps

# 3. 配置环境变量
cp .env.example .env
# 编辑 .env 文件，填入你的配置

# 4. 启动开发环境
make dev
```

### 方式三：生产部署

```bash
# 1. 构建项目
make package

# 2. 启动生产服务
make start
```

## ⚙️ 配置说明

### 环境变量配置

创建 `.env` 文件并配置以下参数：

```bash
# =================
# Binance API 配置
# =================
BINANCE_API_KEY=your_binance_api_key_here
BINANCE_SECRET_KEY=your_binance_secret_key_here
BINANCE_TESTNET=false  # true: 测试网, false: 正式网

# =================
# 数据库配置
# =================
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# =================
# Telegram 通知
# =================
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
TELEGRAM_CHAT_ID=your_telegram_chat_id

# =================
# 服务配置
# =================
HTTP_PORT=8080
LOG_LEVEL=info  # debug, info, warn, error

# =================
# 认证配置
# =================
ADMIN_USERNAME=admin
ADMIN_PASSWORD=your_secure_password
JWT_SECRET=your_jwt_secret_key

# =================
# 交易配置
# =================
POSITION_MODE=both  # both: 双向持仓, single: 单向持仓

# =================
# 风险管理
# =================
BALANCE_RATIO_THRESHOLD=20.0  # 余额比例阈值 (%)
```

### Binance API 配置

1. 登录 [Binance](https://www.binance.com) 账户
2. 前往 **API 管理** → **创建 API**
3. 为 API 启用以下权限：
   - ✅ **现货与杠杆交易**
   - ✅ **期货交易** (必需)
   - ✅ **读取** (必需)
4. 设置 IP 白名单 (推荐)
5. 将 API Key 和 Secret 填入配置文件

### Telegram 通知配置

1. 与 [@BotFather](https://t.me/BotFather) 对话创建 Bot
2. 获取 Bot Token
3. 发送消息给你的 Bot，然后访问：
   ```
   https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates
   ```
4. 从响应中找到你的 Chat ID
5. 将 Token 和 Chat ID 填入配置文件

## 📖 使用指南

### 1. 系统初始化

首次启动后，进行以下初始化操作：

```bash
# 同步 Binance 交易对数据
curl -X POST http://localhost:8080/api/v1/coins/sync
```

### 2. 交易对管理

选择要监控的交易对：

```bash
# 选择 BTCUSDT 进行监控
curl -X POST http://localhost:8080/api/v1/coins/select \
  -H "Content-Type: application/json" \
  -d '{"symbol": "BTCUSDT", "is_selected": true}'
```

### 3. 价格预估设置

创建自动交易策略：

```bash
# 设置 BTC 价格达到 50000 时做多
curl -X POST http://localhost:8080/api/v1/estimates \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTCUSDT",
    "side": "long",
    "action_type": "open",
    "trigger_type": "reach",
    "target_price": 50000.0,
    "quantity": 0.001,
    "margin_mode": "cross",
    "created_by": "trader1"
  }'
```

### 4. 监控运行

系统启动后将自动执行：

- 🔄 **实时监控**：WebSocket 连接监听价格变动
- 🎯 **价格检查**：每秒检查价格是否触发预估条件
- ⚡ **自动执行**：达到条件时自动执行交易策略
- 📨 **即时通知**：通过 Telegram 推送执行结果

## 🔗 API 接口

### 认证接口

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "your_password"
}
```

### 交易对管理

```http
# 获取所有交易对
GET /api/v1/coins

# 获取已选择的交易对
GET /api/v1/coins/selected

# 同步 Binance 交易对
POST /api/v1/coins/sync
```

### 价格预估管理

```http
# 创建价格预估
POST /api/v1/estimates

# 获取所有价格预估
GET /api/v1/estimates/all

# 删除价格预估
DELETE /api/v1/estimates/:id
```

### 监控接口

```http
# 获取订单信息
GET /api/v1/monitor/orders

# 取消订单
POST /api/v1/monitor/orders/cancel

# 获取 K 线数据
GET /api/v1/klines?symbol=BTCUSDT&interval=1m&limit=100
```

## 🔧 开发指南

### 项目结构

```
trading_assistant/
├── 📁 apis/              # HTTP API 路由定义
├── 📁 controllers/       # API 控制器实现
├── 📁 core/              # 核心业务逻辑
│   ├── account_manager.go    # 账户管理器
│   ├── market_manager.go     # 市场管理器  
│   ├── monitor_core.go       # 价格监控器
│   └── order_executor.go     # 订单执行器
├── 📁 models/            # 数据模型定义
├── 📁 pkg/               # 公共包
│   ├── 📁 config/            # 配置管理
│   ├── 📁 exchanges/         # 交易所接口
│   ├── 📁 redis/             # Redis 客户端
│   ├── 📁 telegram/          # Telegram 客户端
│   └── 📁 websocket/         # WebSocket 管理
├── 📁 servers/           # HTTP 服务器
├── 📁 web/               # React 前端应用
│   ├── 📁 src/components/    # React 组件
│   ├── 📁 src/pages/         # 页面组件
│   ├── 📁 src/services/      # API 服务
│   └── 📁 src/utils/         # 工具函数
├── 📄 main.go            # 程序入口
├── 📄 Dockerfile         # Docker 构建文件
├── 📄 Makefile          # 构建脚本
└── 📄 docker-compose.yml # Docker Compose 配置
```

### 本地开发

```bash
# 启动后端开发服务 (带热重载)
go run main.go

# 启动前端开发服务
cd web && npm start

# 同时启动前后端
make dev
```

### 添加新交易所

1. 在 `pkg/exchanges/` 下创建新交易所目录
2. 实现 `ExchangeInterface` 接口：
```go
type ExchangeInterface interface {
    GetAccountBalance() (*AccountBalance, error)
    GetPositions() ([]*Position, error)
    PlaceOrder(*OrderRequest) (*OrderResponse, error)
    StartWebSocket() error
    StopWebSocket()
}
```
3. 在配置中添加相应参数
4. 在 `main.go` 中初始化新交易所客户端

### 扩展功能建议

- 📊 **技术指标**：添加 MA、RSI、MACD 等技术指标
- 📈 **策略回测**：历史数据回测功能
- 🤖 **AI 集成**：机器学习价格预测
- 📱 **移动应用**：React Native 移动端
- 🔔 **多渠道通知**：邮件、短信、Discord 等

## 📊 系统监控

### 健康检查

```bash
# 检查服务状态
curl http://localhost:8080/health

# 检查 WebSocket 连接
curl http://localhost:8080/api/v1/monitor/account
```

### 日志查看

```bash
# Docker 环境查看日志
docker logs trading-assistant-container -f

# 本地环境调整日志级别
export LOG_LEVEL=debug
```

### 性能监控

系统提供以下监控指标：

- ✅ **WebSocket 连接状态**
- ✅ **Redis 连接健康度**
- ✅ **Binance API 调用统计**
- ✅ **价格监控器运行状态**
- ✅ **订单执行成功率**

## ⚠️ 风险提示

> **重要警告**：本系统涉及真实资金交易，请务必注意以下风险：

- 🚨 **市场风险**：加密货币市场波动剧烈，可能导致重大损失
- 🚨 **技术风险**：系统故障、网络中断可能影响交易执行
- 🚨 **配置风险**：错误的参数配置可能导致意外交易
- 🚨 **安全风险**：API 密钥泄露可能导致资产损失

### 安全建议

1. **从小额开始**：首次使用时设置较小的交易金额
2. **测试网优先**：先在 Binance 测试网环境充分测试
3. **定期检查**：定期检查系统运行状态和交易记录
4. **备份配置**：定期备份重要配置和数据
5. **监控日志**：密切关注系统日志和异常信息
6. **风险控制**：设置合理的余额比例阈值和单笔限额

## 🤝 贡献指南

我们欢迎所有形式的贡献！请查看 [贡献指南](CONTRIBUTING.md) 了解详情。

### 如何贡献

1. 🍴 Fork 本项目
2. 🌿 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 💻 提交更改 (`git commit -m 'Add amazing feature'`)
4. 📤 推送到分支 (`git push origin feature/amazing-feature`)
5. 🔀 创建 Pull Request

### 开发规范

- **代码风格**：遵循 Go 和 JavaScript 官方代码规范
- **提交信息**：使用清晰的提交信息描述变更
- **测试覆盖**：为新功能编写相应的测试用例
- **文档更新**：更新相关文档和 API 说明

## 📄 许可证

本项目基于 [MIT License](LICENSE) 开源许可证发布。

---

## 🙏 致谢

感谢以下开源项目和社区：

- [Gin](https://github.com/gin-gonic/gin) - Go Web 框架
- [React](https://reactjs.org) - 用户界面库
- [Ant Design](https://ant.design) - React UI 库
- [Redis](https://redis.io) - 内存数据库
- [Binance API](https://binance-docs.github.io/apidocs/) - 交易所 API

---

## 📞 支持与反馈

如果您在使用过程中遇到问题或有改进建议，请通过以下方式联系：

- 🐛 [提交 Issue](https://github.com/your-username/trading-assistant/issues)
- 💬 [讨论区](https://github.com/your-username/trading-assistant/discussions)
- 📧 Email: your-email@example.com
- 💬 Telegram: [@your_username](https://t.me/your_username)

---

<div align="center">

**⭐ 如果这个项目对您有帮助，请给我们一个 Star！⭐**

Made with ❤️ by [Your Name](https://github.com/your-username)

</div>