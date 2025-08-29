# 🚀 Trading Assistant 快速开始指南

本指南将帮助您快速部署和使用 Trading Assistant。

## 📋 准备工作

### 1. 环境检查
确保您的系统已安装：
- **Go 1.21+** (用于本地编译)
- **Node.js 16+** (用于前端开发)
- **Redis 6.0+** (数据存储)
- **Docker** (推荐部署方式)

### 2. Binance 账户准备
1. 注册并完成 [Binance](https://www.binance.com) 账户认证
2. 进入 **账户 → API管理**
3. 创建新的 API Key，启用以下权限：
   - ✅ 现货与杠杆交易
   - ✅ **期货交易** (必需)
   - ✅ 读取
4. 记录 API Key 和 Secret Key

## ⚡ 30秒快速部署

### 方式一：Docker 一键部署 (推荐)

```bash
# 1. 克隆项目
git clone https://github.com/your-username/trading-assistant.git
cd trading-assistant

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 文件，至少需要配置：
# - BINANCE_API_KEY
# - BINANCE_SECRET_KEY  
# - ADMIN_PASSWORD

# 3. 一键启动
docker-compose up -d

# 4. 访问应用
# 浏览器打开: http://localhost:8080
# 用户名: admin
# 密码: 您在 .env 中设置的密码
```

### 方式二：预编译二进制

```bash
# 1. 下载对应平台的二进制文件
# 从 GitHub Releases 页面下载最新版本

# 2. 解压
tar -xzf trading_assistant_linux_amd64.tar.gz  # Linux
# unzip trading_assistant_windows_amd64.zip    # Windows

# 3. 配置
cp .env.example .env
# 编辑 .env 配置文件

# 4. 启动 Redis
redis-server

# 5. 启动应用
./trading_assistant
```

### 方式三：源码编译

```bash
# 1. 克隆并安装依赖
git clone https://github.com/your-username/trading-assistant.git
cd trading-assistant
make install-deps

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 文件

# 3. 启动开发环境
make dev
```

## 🎯 首次使用流程

### 1. 登录系统
- 访问 http://localhost:8080
- 用户名：`admin`
- 密码：`.env` 文件中设置的密码

### 2. 同步交易对
```bash
# 使用API同步Binance期货交易对
curl -X POST http://localhost:8080/api/v1/coins/sync \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

或在Web界面的 **交易对** 页面点击 **同步交易对** 按钮。

### 3. 选择监控币种
在 **交易对** 页面：
- 选择您要监控的币种（如 BTCUSDT、ETHUSDT）
- 切换开关启用监控

### 4. 设置价格预估
在 **订单** 页面：
- 点击 **新增预估**
- 设置交易对、方向、目标价格、数量等参数
- 保存后系统将自动监控

### 5. 监控运行状态
- **持仓** 页面：查看当前持仓情况
- **余额** 页面：监控账户余额变化
- **K线** 页面：查看价格图表和技术指标

## 📊 核心功能使用

### 🎯 价格预估策略

**做多策略示例**：
```json
{
  "symbol": "BTCUSDT",
  "side": "long",
  "action_type": "open",
  "trigger_type": "reach", 
  "target_price": 50000.0,
  "quantity": 0.001,
  "margin_mode": "cross"
}
```

**做空策略示例**：
```json
{
  "symbol": "ETHUSDT", 
  "side": "short",
  "action_type": "open",
  "trigger_type": "break",
  "target_price": 3000.0,
  "quantity": 0.01,
  "margin_mode": "isolated"
}
```

### 🔔 Telegram 通知设置

1. 与 [@BotFather](https://t.me/BotFather) 对话创建 Bot
2. 获取 Bot Token
3. 向 Bot 发送消息，访问以下URL获取 Chat ID：
   ```
   https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates
   ```
4. 在 `.env` 中配置 Token 和 Chat ID

### 🛡️ 风险管理

**重要配置**：
```bash
# 余额比例阈值 - 当可用余额低于总余额的20%时停止开仓
BALANCE_RATIO_THRESHOLD=20.0

# 持仓模式 - both:双向持仓, single:单向持仓
POSITION_MODE=both
```

## 🔧 常用命令

```bash
# 构建项目
make package

# 开发模式
make dev

# Docker构建
make docker-build

# 查看帮助
make help

# 创建发布
./scripts/first_release.sh  # 首次发布
./scripts/release.sh v1.1.0 # 后续发布
```

## 📱 API 接口示例

### 认证获取Token
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your_password"}'
```

### 创建价格预估
```bash
curl -X POST http://localhost:8080/api/v1/estimates \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "symbol": "BTCUSDT",
    "side": "long", 
    "target_price": 50000.0,
    "quantity": 0.001
  }'
```

### 查询持仓
```bash
curl -X GET http://localhost:8080/api/v1/monitor/positions \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## ⚠️ 安全提示

1. **🚨 首次使用请设置 `BINANCE_TESTNET=true`**
2. **🔐 使用强密码保护管理员账户**
3. **🗝️ 妥善保管 API Key 和 Secret**
4. **💰 设置合理的余额比例阈值**
5. **📊 定期检查系统运行状态**

## 🆘 常见问题

### Q: 连接 Binance API 失败？
A: 检查API密钥权限，确保启用期货交易，检查网络连接和防火墙设置。

### Q: Redis 连接失败？
A: 确保 Redis 服务运行，检查连接参数，考虑使用 Docker 部署避免环境问题。

### Q: WebSocket 连接不稳定？
A: 检查网络稳定性，可能需要使用代理或VPN，确认防火墙允许WebSocket连接。

### Q: 忘记管理员密码？
A: 修改 `.env` 文件中的 `ADMIN_PASSWORD`，重启服务即可。

## 📞 获取帮助

- 📖 [详细文档](README.md)
- 🐛 [问题报告](https://github.com/your-username/trading-assistant/issues)
- 💬 [讨论区](https://github.com/your-username/trading-assistant/discussions)
- 📧 Email: your-email@example.com

---

**🌟 开始您的智能交易之旅吧！**
