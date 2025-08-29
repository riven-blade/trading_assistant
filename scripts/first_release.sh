#!/bin/bash

# Trading Assistant 首次发布脚本
# 用于创建项目的第一个release

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

main() {
    echo "🚀 Trading Assistant 首次发布"
    echo "================================="
    
    print_info "准备创建第一个发布版本..."
    
    # 检查是否已经有release
    if git tag | grep -q "^v"; then
        print_warning "项目已有版本标签，使用普通发布脚本:"
        echo "  ./scripts/release.sh"
        exit 1
    fi
    
    # 默认版本
    VERSION="v1.0.0"
    
    print_info "将创建首个版本: $VERSION"
    print_info "包含以下功能:"
    echo "  ✨ Binance期货交易集成"
    echo "  📊 实时WebSocket监控"  
    echo "  🤖 智能价格预估系统"
    echo "  🌐 现代化React Web界面"
    echo "  🔐 JWT认证系统"
    echo "  📱 Telegram通知集成"
    echo "  🐳 Docker部署支持"
    echo "  📦 多平台二进制发布"
    
    echo
    read -p "是否继续创建首个发布? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "发布已取消"
        exit 0
    fi
    
    # 调用发布脚本
    ./scripts/release.sh "$VERSION"
    
    echo
    print_success "🎉 首次发布完成!"
    echo
    print_info "后续发布请使用:"
    echo "  ./scripts/release.sh vX.Y.Z"
    echo
    print_info "版本发布规则:"
    echo "  - 主版本号: 重大功能变更或破坏性更改"
    echo "  - 次版本号: 新功能添加，向后兼容"  
    echo "  - 修订版本号: 问题修复，向后兼容"
}

# 检查脚本是否存在
if [ ! -f "./scripts/release.sh" ]; then
    print_error "发布脚本不存在: ./scripts/release.sh"
    exit 1
fi

# 确保发布脚本可执行
chmod +x ./scripts/release.sh

# 运行主函数
main "$@"
