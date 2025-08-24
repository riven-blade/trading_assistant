// 操作类型常量
export const ACTIONS = {
  add_position: {
    title: '加仓',
    priceLabel: '加仓价格',
    quantityLabel: '加仓数量',
    priceRange: { min: -30, max: 30 },
    priceBase: 'current', // 基于当前价格
    color: '#52c41a'
  },
  take_profit: {
    title: '止盈',
    priceLabel: '止盈价格',
    quantityLabel: '止盈数量',
    priceRange: { min: -50, max: 50 },
    priceBase: 'entry', // 基于开仓价格
    color: '#1890ff'
  },
  stop_loss: {
    title: '止损',
    priceLabel: '止损价格',
    quantityLabel: '止损数量',
    priceRange: { min: -50, max: 50 },
    priceBase: 'entry', // 基于开仓价格
    color: '#ff4d4f'
  }
};

// 订单状态映射
export const ORDER_STATUS_MAP = {
  'NEW': { color: 'blue', text: '新订单' },
  'PARTIALLY_FILLED': { color: 'orange', text: '部分成交' },
  'FILLED': { color: 'green', text: '完全成交' },
  'CANCELED': { color: 'gray', text: '已取消' },
  'REJECTED': { color: 'red', text: '已拒绝' },
  'EXPIRED': { color: 'red', text: '已过期' }
};

// 操作类型颜色映射
export const ACTION_TYPE_COLORS = {
  'add_position': 'green',
  'take_profit': 'blue',
  'stop_loss': 'red',
  'open_position': 'orange'
};

// 操作类型文本映射
export const ACTION_TYPE_TEXT = {
  'add_position': '加仓',
  'take_profit': '止盈',
  'stop_loss': '止损',
  'open_position': '开仓'
};

// 默认配置
export const DEFAULT_CONFIG = {
  leverage: 3,
  marginMode: 'isolated', // 默认逐仓模式，风险更可控
  orderType: 'limit',
  refreshInterval: {
    price: 5000,      // 价格更新间隔
    account: 30000,   // 账户信息更新间隔
    position: 10000,  // 持仓更新间隔
    estimate: 30000   // 监听统计更新间隔
  }
};
