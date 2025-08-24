# 交易助手 (Trading Assistant)

一个基于Go语言开发的自动化交易助手，集成Binance期货交易、Redis存储、Telegram通知等功能。

## 功能特性

- 🏪 **多交易所支持**: 目前集成Binance期货交易
- 📊 **WebSocket监听**: 实时监听订单薄、持仓和余额变化
- 💰 **智能下单**: 根据预设价格自动执行交易
- 📱 **Telegram通知**: 实时推送交易信号、持仓变化和系统状态
- 🔄 **双向持仓**: 支持单向和双向持仓模式
- 🗄️ **Redis存储**: 使用Redis作为数据存储后端
- 🌐 **HTTP API**: 提供完整的REST API接口
- ⚡ **高性能**: 使用WebSocket而非轮询，响应更快、资源消耗更低
- 🔐 **JWT认证**: 基于JWT的用户认证系统，保护API安全

## 项目结构

```
trading_assistant/
├── apis/                 # HTTP API路由
├── controllers/          # API控制器
├── core/                # 核心业务逻辑
├── models/              # 数据模型
├── pkg/                 # 公共包
│   ├── config/          # 配置管理
│   ├── exchanges/       # 交易所API封装
│   ├── redis/           # Redis客户端
│   └── telegram/        # Telegram客户端
├── servers/             # HTTP服务器
├── web/                 # 前端文件
└── main.go              # 程序入口
```

## 安装部署

### 1. 环境要求

- Go 1.21+
- Redis 6.0+
- Binance API账户
- Telegram Bot (可选)

### 2. 克隆项目

```bash
git clone <repository-url>
cd trading_assistant
```

### 3. 安装依赖

```bash
go mod tidy
```

### 4. 配置环境变量

创建 `.env` 文件：

```bash
# Binance API配置
BINANCE_API_KEY=your_binance_api_key
BINANCE_SECRET_KEY=your_binance_secret_key
BINANCE_TESTNET=false

# Redis配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Telegram配置
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
TELEGRAM_CHAT_ID=your_telegram_chat_id

# 服务配置
HTTP_PORT=8080
LOG_LEVEL=info

# 交易配置
POSITION_MODE=both  # both: 双向持仓, single: 单向持仓
# 保证金模式现在支持每个订单单独配置
```

### 5. 启动服务

```bash
go run main.go
```

## API使用说明

### 币种管理

#### 获取所有币种
```bash
GET /api/v1/coins
```

#### 获取已筛选币种
```bash
GET /api/v1/coins/selected
```

#### 筛选币种
```bash
POST /api/v1/coins/select
{
  "symbol": "BTCUSDT",
  "is_selected": true
}
```

#### 同步交易所币种
```bash
POST /api/v1/coins/sync
```

### 价格预估管理

#### 创建价格预估
```bash
POST /api/v1/estimates
{
  "symbol": "BTCUSDT",
  "side": "long",
  "target_price": 50000.0,
  "quantity": 0.001,
  "margin_mode": "cross",
  "created_by": "user1"
}
```

#### 获取价格预估列表
```bash
GET /api/v1/estimates?symbol=BTCUSDT&status=pending
```

### 监控管理

#### 获取账户状态（包含WebSocket状态）
```bash
GET /api/v1/monitor/account
```

#### 获取持仓信息
```bash
GET /api/v1/monitor/positions
```

#### 获取余额信息
```bash
GET /api/v1/monitor/balances
```

#### 获取订单薄
```bash
GET /api/v1/monitor/orderbook/BTCUSDT
```

## 使用流程

### 1. 同步币种
首先从Binance同步期货交易对到系统：
```bash
curl -X POST http://localhost:8080/api/v1/coins/sync
```

### 2. 筛选币种
选择要监控的交易对：
```bash
curl -X POST http://localhost:8080/api/v1/coins/select \
  -H "Content-Type: application/json" \
  -d '{"symbol": "BTCUSDT", "is_selected": true}'
```

### 3. 设置价格预估
创建价格触发条件：
```bash
curl -X POST http://localhost:8080/api/v1/estimates \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTCUSDT",
    "side": "long",
    "target_price": 50000.0,
    "quantity": 0.001,
    "created_by": "trader1"
  }'
```

### 4. 监控运行
系统将自动：
- **WebSocket监听**：实时监听选中币种的订单薄
- **账户监控**：使用WebSocket实时监控持仓和余额变化
- **价格检查**：检查价格是否达到预估目标
- **保证金设置**：根据每个订单的配置自动设置保证金模式
- **自动交易**：达到目标时自动下单
- **即时通知**：通过Telegram发送实时通知

## 配置说明

### Binance API配置
1. 登录Binance账户
2. 创建API密钥
3. 启用期货交易权限
4. 设置IP白名单（推荐）

### Telegram配置
1. 创建Telegram Bot (@BotFather)
2. 获取Bot Token
3. 获取Chat ID
4. 配置到环境变量

### 持仓模式
- `both`: 双向持仓模式，可以同时持有多空仓位
- `single`: 单向持仓模式，只能持有一个方向的仓位

## 注意事项

⚠️ **风险提示**
- 本系统涉及真实资金交易，使用前请充分测试
- 建议先在测试网环境运行
- 设置合理的风险控制参数
- 定期检查系统运行状态

## 开发说明

### 添加新交易所
1. 在 `pkg/exchanges/` 目录下创建新的交易所客户端
2. 实现统一的接口规范
3. 在配置中添加相应的参数

### 扩展功能
- 风险管理模块
- 更多技术指标
- 策略回测功能
- Web前端界面

## 故障排除

### 常见问题

1. **Redis连接失败**
   - 检查Redis服务是否启动
   - 确认连接参数正确

2. **Binance API错误**
   - 验证API密钥和权限
   - 检查网络连接
   - 确认API限制未超出

3. **WebSocket连接问题**
   - 检查网络稳定性
   - 确认防火墙设置

## 许可证

本项目采用MIT许可证。详见 [LICENSE](LICENSE) 文件。
