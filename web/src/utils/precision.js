// 计算工具函数

/**
 * 计算所需USDT保证金
 * @param {number} quantity - 数量
 * @param {number} price - 价格
 * @param {number} leverage - 杠杆倍数
 * @returns {number} 所需USDT金额
 */
export const calculateUsdtAmount = (quantity, price, leverage = 1) => {
  if (!quantity || !price) return 0;
  return (quantity * price) / leverage;
};

/**
 * 计算盈亏用于显示
 * @param {number} quantity - 数量
 * @param {number} entryPrice - 开仓价格
 * @param {number} targetPrice - 目标价格
 * @param {string} side - 持仓方向 ('LONG' | 'SHORT')
 * @returns {number} 盈亏金额
 */
export const calculatePnl = (quantity, entryPrice, targetPrice, side) => {
  if (!quantity || !entryPrice || !targetPrice) return 0;
  
  const isLong = side === 'LONG';
  let profitPerCoin;
  
  if (isLong) {
    profitPerCoin = targetPrice - entryPrice;
  } else {
    profitPerCoin = entryPrice - targetPrice;
  }
  
  return quantity * profitPerCoin;
};