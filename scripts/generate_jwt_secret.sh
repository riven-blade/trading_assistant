#!/bin/bash

# JWT Secret 生成脚本
# 用于生成安全的JWT密钥

set -e

echo "🔐 JWT Secret 生成器"
echo "===================="

# 检查是否安装了openssl
if ! command -v openssl &> /dev/null; then
    echo "❌ 错误: 未找到 openssl 命令"
    echo "请安装 openssl:"
    echo "  macOS: brew install openssl"
    echo "  Ubuntu/Debian: sudo apt-get install openssl"
    echo "  CentOS/RHEL: sudo yum install openssl"
    exit 1
fi

# 生成不同长度的JWT Secret
echo ""
echo "📝 生成的JWT Secret密钥:"
echo ""

echo "🔹 32字节 (256位) - 推荐用于生产环境:"
JWT_SECRET_32=$(openssl rand -hex 32)
echo "JWT_SECRET=$JWT_SECRET_32"

echo ""
echo "🔹 64字节 (512位) - 超高安全性:"
JWT_SECRET_64=$(openssl rand -hex 64)
echo "JWT_SECRET=$JWT_SECRET_64"

echo ""
echo "🔹 Base64编码格式 (44字符):"
JWT_SECRET_BASE64=$(openssl rand -base64 32)
echo "JWT_SECRET=$JWT_SECRET_BASE64"

echo ""
echo "📋 使用说明:"
echo "1. 选择上面任意一个JWT_SECRET"
echo "2. 将其添加到 .env 文件中"
echo "3. 重启应用程序使配置生效"

echo ""
echo "⚠️  安全提示:"
echo "- 请妥善保管JWT Secret，不要泄露给他人"
echo "- 生产环境建议使用32字节或更长的密钥"
echo "- 定期更换JWT Secret以提高安全性"
echo "- 不要将JWT Secret提交到版本控制系统"

echo ""
echo "✅ JWT Secret 生成完成!"
