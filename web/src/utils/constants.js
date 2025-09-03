// 操作类型常量
export const ACTIONS = {
  open: {
    title: '开仓',
    priceLabel: '开仓价格',
    quantityLabel: '开仓数量',
    priceRange: { min: -100, max: 100 },
    priceBase: 'current', // 基于当前价格
    color: '#52c41a'
  },
  addition: {
    title: '加仓',
    priceLabel: '加仓价格',
    quantityLabel: '加仓数量',
    priceRange: { min: -100, max: 100 },
    priceBase: 'current', // 基于当前价格
    color: '#52c41a'
  },
  close: {
    title: '平仓',
    priceLabel: '平仓价格',
    quantityLabel: '平仓数量',
    priceRange: { min: -100, max: 100 },
    priceBase: 'entry', // 基于开仓价格
    color: '#722ed1'
  },
  take_profit: {
    title: '止盈',
    priceLabel: '止盈价格',
    quantityLabel: '止盈数量',
    priceRange: { min: -100, max: 100 },
    priceBase: 'current', // 基于当前价格
    color: '#1890ff'
  },
  stop_loss: {
    title: '止损',
    priceLabel: '止损价格',
    quantityLabel: '止损数量',
    priceRange: { min: -100, max: 100 },
    priceBase: 'current', // 基于当前价格
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
  'open': 'green',
  'addition': 'green',
  'close': 'purple',
  'take_profit': 'blue',
  'stop_loss': 'red'
};

// 操作类型文本映射
export const ACTION_TYPE_TEXT = {
  'open': '开仓',
  'addition': '加仓',
  'close': '平仓',
  'take_profit': '止盈',
  'stop_loss': '止损'
};

// 详细操作类型文本映射
export const DETAILED_ACTION_TYPE_TEXT = {
  // 开仓类型
  'open_long': '做多开仓',
  'open_short': '做空开仓',
  
  // 加仓类型
  'addition_long': '做多加仓',
  'addition_short': '做空加仓',
  
  // 平仓类型
  'close_long': '多头平仓',
  'close_short': '空头平仓',
  
  // 止盈类型
  'take_profit_long': '多头止盈',
  'take_profit_short': '空头止盈',
  
  // 止损类型
  'stop_loss_long': '多头止损',
  'stop_loss_short': '空头止损'
};

// 根据action_type和side获取详细操作类型文本
export const getDetailedActionText = (actionType, side) => {
  const sideKey = side === 'long' ? 'long' : 'short';
  const key = `${actionType}_${sideKey}`;
  return DETAILED_ACTION_TYPE_TEXT[key] || ACTION_TYPE_TEXT[actionType] || actionType;
};

// 默认配置
export const DEFAULT_CONFIG = {
  leverage: 3,
  marginMode: 'CROSS', // 默认全仓模式
  orderType: 'limit',
  refreshInterval: {
    price: 5000,      // 价格更新间隔
    account: 30000,   // 账户信息更新间隔
    position: 10000,  // 持仓更新间隔
    estimate: 30000   // 监听统计更新间隔
  }
};
