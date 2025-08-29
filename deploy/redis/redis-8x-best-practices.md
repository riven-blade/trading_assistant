# Redis 8.x 配置最佳实践

## 🎯 配置文件规则

### ✅ 正确的配置格式

```bash
# 注释应该独立成行
save 900 1

# 多行注释示例
# 这是关于内存策略的说明
# 当内存达到上限时使用LRU算法
maxmemory-policy allkeys-lru

# 字符串值需要引号
appendfilename "appendonly.aof"
```

### ❌ 避免的错误格式

```bash
# 错误：行内注释（特别是中文）
save 900 1      # 900秒内至少1个key变化时保存

# 错误：配置值后直接跟注释
appendfsync everysec  # 每秒同步一次

# 错误：注释和配置在同一行
maxmemory-policy allkeys-lru # LRU策略
```

## 🔧 Redis 8.x 新特性配置

### 1. 增强的事件通知

```bash
# 启用键空间通知
notify-keyspace-events "Ex"

# 启用所有事件类型（包括新增的）
notify-keyspace-events "AKE$lshzxegtmu"
```

### 2. 性能优化配置

```bash
# IO 线程数（根据CPU核心数调整）
io-threads 4
io-threads-do-reads yes

# 内存碎片整理增强
active-defrag-max-scan-fields 1000

# 延迟监控
latency-monitor-threshold 100
```

### 3. 安全增强配置

```bash
# 禁用危险命令
enable-protected-configs no
enable-debug-command no
enable-module-command no

# 密码认证（建议启用）
requirepass your_strong_password_here
```

### 4. AOF 持久化增强

```bash
# AOF 时间戳（新特性）
aof-timestamp-enabled no

# 内存驱逐策略增强
maxmemory-eviction-tenacity 10
```

## 📊 监控和指标

### 内存使用监控

```bash
# 连接跟踪配置
tracking-table-max-keys 1000000

# 详细统计信息
info-replication-backlog-size 1mb
```

### 性能调优

```bash
# 慢查询日志
slowlog-log-slower-than 10000
slowlog-max-len 128

# 客户端超时
timeout 300
tcp-keepalive 300
```

## 🛡️ 生产环境建议

### 安全配置

```bash
# 绑定指定IP（生产环境不要使用 0.0.0.0）
bind 127.0.0.1

# 启用保护模式
protected-mode yes

# 设置强密码
requirepass "$(openssl rand -base64 32)"
```

### 持久化策略

```bash
# RDB + AOF 混合持久化
save 900 1
save 300 10
save 60 10000

appendonly yes
aof-use-rdb-preamble yes
```

### 内存管理

```bash
# 设置最大内存（根据服务器配置）
maxmemory 2gb
maxmemory-policy allkeys-lru

# 启用内存碎片整理
activedefrag yes
active-defrag-threshold-lower 10
```

## 🚨 常见错误和解决方案

### 配置文件解析错误

```bash
# 问题：Invalid save parameters
# 原因：行内注释导致
# 解决：将注释移到独立行

# 错误示例
save 900 1 # 注释

# 正确示例  
# 注释
save 900 1
```

### 字符编码问题

```bash
# 确保配置文件使用UTF-8编码
# 避免使用特殊字符在配置值中
# 字符串值使用双引号包围
```

### 版本兼容性

```bash
# Redis 8.x 废弃的配置项：
# - hash-max-ziplist-* (改为 hash-max-listpack-*)
# - zset-max-ziplist-* (改为 zset-max-listpack-*)

# 新的配置项：
hash-max-listpack-entries 512
hash-max-listpack-value 64
zset-max-listpack-entries 128
zset-max-listpack-value 64
```

## 🔍 配置验证

### 启动前验证

```bash
# 使用 redis-server 验证配置
redis-server /path/to/redis.conf --test-config

# Docker 环境验证
docker run --rm -v $(pwd)/redis.conf:/redis.conf redis:8.0-alpine redis-server /redis.conf --test-config
```

### 运行时检查

```bash
# 连接到Redis并检查配置
redis-cli CONFIG GET "*"

# 检查特定配置项
redis-cli CONFIG GET save
redis-cli CONFIG GET maxmemory-policy
```

## 📝 配置文件模板

详细的配置模板请参考：
- `redis-8.conf` - 生产环境优化配置
- `redis.conf` - 当前使用的配置文件
- `redis.conf.backup` - 原始配置备份

## 🔄 升级检查清单

- [ ] 备份现有配置和数据
- [ ] 检查配置文件语法
- [ ] 移除行内注释
- [ ] 更新废弃的配置项
- [ ] 测试新配置
- [ ] 监控性能指标
- [ ] 验证数据完整性

