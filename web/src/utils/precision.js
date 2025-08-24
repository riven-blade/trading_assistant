// 精度处理工具函数

/**
 * 验证价格是否符合tickSize要求
 * @param {number} price - 价格
 * @param {string} tickSize - 价格最小变动单位
 * @returns {boolean} 是否符合要求
 */
export const validatePrice = (price, tickSize) => {
  if (!price || !tickSize) return false;
  
  const tick = parseFloat(tickSize);
  if (tick <= 0) return false;
  
  // 使用整数运算避免浮点数精度问题
  const tickStr = tick.toString();
  const decimalPlaces = tickStr.includes('.') ? tickStr.split('.')[1].length : 0;
  const factor = Math.pow(10, decimalPlaces);
  
  const priceInt = Math.round(price * factor);
  const tickInt = Math.round(tick * factor);
  
  const remainder = priceInt % tickInt;
  return Math.abs(remainder) < 1e-6; // 放宽到微秒级别的容差
};

/**
 * 验证数量是否符合stepSize要求
 * @param {number} quantity - 数量
 * @param {string} stepSize - 数量最小变动单位
 * @returns {boolean} 是否符合要求
 */
export const validateQuantity = (quantity, stepSize) => {
  if (!quantity || !stepSize) return false;
  
  const step = parseFloat(stepSize);
  if (step <= 0) return false;
  
  // 使用整数运算避免浮点数精度问题
  const stepStr = step.toString();
  const decimalPlaces = stepStr.includes('.') ? stepStr.split('.')[1].length : 0;
  const factor = Math.pow(10, decimalPlaces);
  
  const quantityInt = Math.round(quantity * factor);
  const stepInt = Math.round(step * factor);
  
  const remainder = quantityInt % stepInt;
  return Math.abs(remainder) < 1e-6; // 放宽到微秒级别的容差
};

/**
 * 根据精度格式化价格
 * @param {number} price - 价格
 * @param {number} precision - 小数位数
 * @returns {string} 格式化后的价格
 */
export const formatPrice = (price, precision = 4) => {
  if (!price && price !== 0) return '0';
  return parseFloat(price).toFixed(precision);
};

/**
 * 根据精度格式化数量
 * @param {number} quantity - 数量
 * @param {number} precision - 小数位数
 * @returns {string} 格式化后的数量
 */
export const formatQuantity = (quantity, precision = 6) => {
  if (!quantity && quantity !== 0) return '0';
  return parseFloat(quantity).toFixed(precision);
};

/**
 * 将价格调整到符合tickSize的最近值
 * @param {number} price - 原始价格
 * @param {string} tickSize - 价格最小变动单位
 * @returns {number} 调整后的价格
 */
export const adjustPriceToTickSize = (price, tickSize) => {
  if (!price || !tickSize) return price;
  
  const tick = parseFloat(tickSize);
  if (tick <= 0) return price;
  
  // 使用字符串操作避免浮点数精度问题
  const tickStr = tick.toString();
  const decimalPlaces = tickStr.includes('.') ? tickStr.split('.')[1].length : 0;
  
  // 转换为整数计算
  const factor = Math.pow(10, decimalPlaces);
  const priceInt = Math.round(price * factor);
  const tickInt = Math.round(tick * factor);
  
  // 在整数域进行计算
  const roundedInt = Math.round(priceInt / tickInt) * tickInt;
  const result = roundedInt / factor;
  
  // 最终格式化
  return parseFloat(result.toFixed(decimalPlaces));
};

/**
 * 将数量调整到符合stepSize的最近值
 * @param {number} quantity - 原始数量
 * @param {string} stepSize - 数量最小变动单位
 * @returns {number} 调整后的数量
 */
export const adjustQuantityToStepSize = (quantity, stepSize) => {
  if (!quantity || !stepSize) return quantity;
  
  const step = parseFloat(stepSize);
  if (step <= 0) return quantity;
  
  // 使用字符串操作避免浮点数精度问题
  const stepStr = step.toString();
  const decimalPlaces = stepStr.includes('.') ? stepStr.split('.')[1].length : 0;
  
  // 转换为整数计算
  const factor = Math.pow(10, decimalPlaces);
  const quantityInt = Math.round(quantity * factor);
  const stepInt = Math.round(step * factor);
  
  // 在整数域进行计算
  const roundedInt = Math.round(quantityInt / stepInt) * stepInt;
  const result = roundedInt / factor;
  
  // 最终格式化
  return parseFloat(result.toFixed(decimalPlaces));
};

/**
 * 验证价格是否在允许范围内
 * @param {number} price - 价格
 * @param {string} minPrice - 最小价格
 * @param {string} maxPrice - 最大价格
 * @returns {boolean} 是否在范围内
 */
export const validatePriceRange = (price, minPrice, maxPrice) => {
  if (!price) return false;
  
  const min = parseFloat(minPrice || 0);
  const max = parseFloat(maxPrice || Infinity);
  
  return price >= min && price <= max;
};

/**
 * 验证数量是否在允许范围内
 * @param {number} quantity - 数量
 * @param {string} minQty - 最小数量
 * @param {string} maxQty - 最大数量
 * @returns {boolean} 是否在范围内
 */
export const validateQuantityRange = (quantity, minQty, maxQty) => {
  if (!quantity) return false;
  
  const min = parseFloat(minQty || 0);
  const max = parseFloat(maxQty || Infinity);
  
  return quantity >= min && quantity <= max;
};

/**
 * 获取InputNumber组件的step值
 * @param {string} stepSize - 最小变动单位
 * @returns {number} step值
 */
export const getInputStep = (stepSize) => {
  if (!stepSize) return 0.01;
  return parseFloat(stepSize);
};

/**
 * 获取InputNumber组件的精度设置
 * @param {number} precision - 精度位数
 * @returns {number} 精度值
 */
export const getInputPrecision = (precision) => {
  if (typeof precision !== 'number' || precision < 0) return 4;
  return precision;
};

/**
 * 完整的价格验证（包括精度、步长、范围）
 * @param {number} price - 价格
 * @param {object} coinInfo - 币种信息
 * @returns {object} 验证结果 {valid: boolean, error: string}
 */
export const validatePriceComplete = (price, coinInfo) => {
  if (!coinInfo) {
    return { valid: false, error: '缺少币种精度信息' };
  }

  // 验证价格范围
  if (!validatePriceRange(price, coinInfo.min_price, coinInfo.max_price)) {
    return { 
      valid: false, 
      error: `价格必须在 ${coinInfo.min_price} - ${coinInfo.max_price} 之间` 
    };
  }

  // 验证价格精度
  if (!validatePrice(price, coinInfo.tick_size)) {
    return { 
      valid: false, 
      error: `价格必须是 ${coinInfo.tick_size} 的整数倍` 
    };
  }

  return { valid: true, error: '' };
};

/**
 * 完整的数量验证（包括精度、步长、范围）
 * @param {number} quantity - 数量
 * @param {object} coinInfo - 币种信息
 * @returns {object} 验证结果 {valid: boolean, error: string}
 */
export const validateQuantityComplete = (quantity, coinInfo) => {
  if (!coinInfo) {
    return { valid: false, error: '缺少币种精度信息' };
  }

  // 验证数量范围
  if (!validateQuantityRange(quantity, coinInfo.min_qty, coinInfo.max_qty)) {
    return { 
      valid: false, 
      error: `数量必须在 ${coinInfo.min_qty} - ${coinInfo.max_qty} 之间` 
    };
  }

  // 验证数量精度
  if (!validateQuantity(quantity, coinInfo.step_size)) {
    return { 
      valid: false, 
      error: `数量必须是 ${coinInfo.step_size} 的整数倍` 
    };
  }

  return { valid: true, error: '' };
};

/**
 * 自动修正价格和数量到符合要求的值
 * @param {number} price - 原始价格
 * @param {number} quantity - 原始数量
 * @param {object} coinInfo - 币种信息
 * @returns {object} 修正后的值 {price: number, quantity: number}
 */
export const autoFixPriceAndQuantity = (price, quantity, coinInfo) => {
  if (!coinInfo) {
    return { price, quantity };
  }

  let fixedPrice = price;
  let fixedQuantity = quantity;

  // 修正价格
  if (price) {
    fixedPrice = adjustPriceToTickSize(price, coinInfo.tick_size);
    
    // 确保在范围内
    const minPrice = parseFloat(coinInfo.min_price || 0);
    const maxPrice = parseFloat(coinInfo.max_price || Infinity);
    fixedPrice = Math.max(minPrice, Math.min(maxPrice, fixedPrice));
  }

  // 修正数量
  if (quantity) {
    fixedQuantity = adjustQuantityToStepSize(quantity, coinInfo.step_size);
    
    // 确保在范围内
    const minQty = parseFloat(coinInfo.min_qty || 0);
    const maxQty = parseFloat(coinInfo.max_qty || Infinity);
    fixedQuantity = Math.max(minQty, Math.min(maxQty, fixedQuantity));
  }

  return { 
    price: fixedPrice, 
    quantity: fixedQuantity 
  };
};

/**
 * 获取价格的小数位数
 * @param {string} tickSize - 价格最小变动单位
 * @returns {number} 小数位数
 */
export const getPriceDecimalPlaces = (tickSize) => {
  if (!tickSize) return 4;
  
  const tickStr = tickSize.toString();
  if (tickStr.includes('.')) {
    return tickStr.split('.')[1].length;
  }
  return 0;
};

/**
 * 获取数量的小数位数
 * @param {string} stepSize - 数量最小变动单位
 * @returns {number} 小数位数
 */
export const getQuantityDecimalPlaces = (stepSize) => {
  if (!stepSize) return 6;
  
  const stepStr = stepSize.toString();
  if (stepStr.includes('.')) {
    return stepStr.split('.')[1].length;
  }
  return 0;
};
