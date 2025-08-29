#!/bin/bash

# Trading Assistant Release Script
# 用于创建新版本发布的脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 函数定义
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

# 检查工具是否存在
check_dependencies() {
    print_info "检查依赖工具..."
    
    local missing_deps=()
    
    if ! command -v git &> /dev/null; then
        missing_deps+=("git")
    fi
    
    if ! command -v go &> /dev/null; then
        missing_deps+=("go")
    fi
    
    if ! command -v node &> /dev/null; then
        missing_deps+=("node")
    fi
    
    if ! command -v npm &> /dev/null; then
        missing_deps+=("npm")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "缺少以下依赖工具: ${missing_deps[*]}"
        exit 1
    fi
    
    print_success "所有依赖工具已安装"
}

# 检查Git状态
check_git_status() {
    print_info "检查Git状态..."
    
    if [[ -n $(git status --porcelain) ]]; then
        print_error "工作目录不干净，请先提交或储存您的更改"
        git status
        exit 1
    fi
    
    local current_branch=$(git branch --show-current)
    if [[ "$current_branch" != "main" && "$current_branch" != "master" ]]; then
        print_warning "当前不在主分支 (main/master)，当前分支: $current_branch"
        read -p "是否继续? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    
    print_success "Git状态检查通过"
}

# 版本验证
validate_version() {
    local version=$1
    if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        print_error "版本格式错误，应该是 vX.Y.Z (例如: v1.2.3)"
        exit 1
    fi
    
    # 检查版本是否已存在
    if git tag | grep -q "^$version$"; then
        print_error "版本 $version 已存在"
        exit 1
    fi
    
    print_success "版本格式验证通过: $version"
}

# 运行测试
run_tests() {
    print_info "运行测试..."
    
    # Go测试
    if ! go test ./...; then
        print_error "Go测试失败"
        exit 1
    fi
    
    # 前端测试（如果存在）
    if [ -f "web/package.json" ]; then
        cd web
        if ! npm test -- --coverage --watchAll=false; then
            print_error "前端测试失败"
            exit 1
        fi
        cd ..
    fi
    
    print_success "所有测试通过"
}

# 构建项目
build_project() {
    print_info "构建项目..."
    
    # 清理之前的构建
    rm -rf dist/
    mkdir -p dist/
    
    # 使用Makefile构建
    if ! make package; then
        print_error "项目构建失败"
        exit 1
    fi
    
    print_success "项目构建完成"
}

# 创建发布说明
create_release_notes() {
    local version=$1
    local notes_file="release_notes_${version}.md"
    
    print_info "创建发布说明..."
    
    cat > "$notes_file" << EOF
# Trading Assistant ${version}

## 🚀 新功能

- 

## 🐛 修复问题

- 

## 📈 性能优化

- 

## 📝 文档更新

- 

## 🔄 其他更改

- 

## 📦 下载地址

请根据您的系统选择对应的版本：

- **Linux (x64)**: \`trading_assistant_linux_amd64.tar.gz\`
- **Linux (ARM64)**: \`trading_assistant_linux_arm64.tar.gz\`
- **macOS (Intel)**: \`trading_assistant_darwin_amd64.tar.gz\`  
- **macOS (Apple Silicon)**: \`trading_assistant_darwin_arm64.tar.gz\`
- **Windows (x64)**: \`trading_assistant_windows_amd64.zip\`

## 🔧 升级说明

1. 停止现有服务
2. 备份配置文件
3. 替换可执行文件
4. 检查配置兼容性
5. 重启服务

## ⚠️ 重要提示

- 请在正式环境使用前充分测试
- 建议先在测试网环境验证
- 注意备份重要配置和数据

完整更新日志请查看: [CHANGELOG.md](https://github.com/your-username/trading-assistant/blob/main/CHANGELOG.md)
EOF

    # 打开编辑器让用户编辑发布说明
    ${EDITOR:-nano} "$notes_file"
    
    print_success "发布说明已创建: $notes_file"
}

# 创建Git标签和推送
create_and_push_tag() {
    local version=$1
    local notes_file="release_notes_${version}.md"
    
    print_info "创建Git标签..."
    
    # 创建带注释的标签
    git tag -a "$version" -F "$notes_file"
    
    print_info "推送标签到远程仓库..."
    git push origin "$version"
    
    print_success "标签 $version 已创建并推送"
    
    # 清理临时文件
    rm -f "$notes_file"
}

# 主函数
main() {
    echo "🚀 Trading Assistant 发布脚本"
    echo "================================="
    
    # 获取版本参数
    if [ $# -eq 0 ]; then
        read -p "请输入版本号 (格式: vX.Y.Z): " VERSION
    else
        VERSION=$1
    fi
    
    # 验证和准备
    validate_version "$VERSION"
    check_dependencies
    check_git_status
    
    # 确认发布
    echo
    print_info "即将发布版本: $VERSION"
    print_warning "此操作将："
    echo "  - 运行所有测试"
    echo "  - 构建项目"  
    echo "  - 创建Git标签"
    echo "  - 推送到远程仓库"
    echo "  - 触发GitHub Actions自动构建和发布"
    echo
    read -p "是否继续? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "发布已取消"
        exit 0
    fi
    
    # 执行发布流程
    run_tests
    build_project
    create_release_notes "$VERSION"
    create_and_push_tag "$VERSION"
    
    echo
    print_success "🎉 版本 $VERSION 发布完成！"
    echo
    print_info "接下来的步骤："
    echo "1. GitHub Actions 将自动构建多平台二进制文件"
    echo "2. Docker 镜像将自动构建并推送到Docker Hub"
    echo "3. 在GitHub上检查Release页面确认发布成功"
    echo
    print_info "监控发布进度:"
    echo "https://github.com/your-username/trading-assistant/actions"
}

# 运行主函数
main "$@"
