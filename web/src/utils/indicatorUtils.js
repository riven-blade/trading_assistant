import { getSupportedIndicators } from 'klinecharts';

// 指标分类配置 - 基于KLineCharts支持的所有内置指标
const INDICATOR_CATEGORIES = {
  // 趋势指标 - 显示在主图
  trend: {
    name: '趋势分析',
    type: 'overlay', // 主图叠加
    icon: '📈',
    color: '#52c41a',
    indicators: [
      'MA', 'EMA', 'SMA', 'BBI', 'BOLL', 'SAR'
    ]
  },
  
  // 动量指标 - 显示在副图
  momentum: {
    name: '动量振荡',
    type: 'oscillator', // 副图显示
    icon: '⚡',
    color: '#1890ff', 
    indicators: [
      'MACD', 'RSI', 'KDJ', 'CCI', 'WR', 'ROC', 'PSY', 'MTM'
    ]
  },
  
  // 成交量指标 - 显示在副图
  volume: {
    name: '成交量分析',
    type: 'oscillator',
    icon: '📊',
    color: '#fa8c16',
    indicators: [
      'VOL', 'OBV', 'PVT', 'VR', 'EMV'
    ]
  },
  
  // 波动性指标 - 显示在副图
  volatility: {
    name: '波动性分析', 
    type: 'oscillator',
    icon: '🌊',
    color: '#722ed1',
    indicators: [
      'BIAS', 'BRAR', 'CR', 'DMI'
    ]
  }
};

// 指标默认参数配置 - 基于KLineCharts文档的默认参数
const INDICATOR_DEFAULT_PARAMS = {
  // 趋势指标
  'MA': [], // 使用默认参数 [5, 10, 30, 60]
  'EMA': [], // 使用默认参数 [6, 12, 20]
  'SMA': [], // 使用默认参数 [12, 2]
  'BBI': [], // 使用默认参数 [3, 6, 12, 24]
  'BOLL': [], // 使用默认参数 [20, 2]
  'SAR': [], // 使用默认参数 [2, 2, 20]
  
  // 动量指标
  'MACD': [], // 使用默认参数 [12, 26, 9]
  'RSI': [], // 使用默认参数 [6, 12, 24]
  'KDJ': [], // 使用默认参数 [9, 3, 3]
  'CCI': [], // 使用默认参数 [13]
  'WR': [], // 使用默认参数 [6, 10, 14]
  'ROC': [], // 使用默认参数 [12, 6]
  'PSY': [], // 使用默认参数 [12, 6]
  'MTM': [], // 使用默认参数 [6, 10]
  
  // 成交量指标
  'VOL': [], // 使用默认参数 [5, 10, 20]
  'OBV': [], // 使用默认参数 [30]
  'PVT': [], // 无参数
  'VR': [], // 使用默认参数 [24, 30]
  'EMV': [], // 使用默认参数 [14, 9]
  
  // 波动性指标
  'BIAS': [], // 使用默认参数 [6, 12, 24]
  'BRAR': [], // 使用默认参数 [26]
  'CR': [], // 使用默认参数 [26, 10, 20, 40, 60]
  'DMI': [] // 使用默认参数 [14, 6]
};

// 指标显示名称映射 - 基于KLineCharts所有内置指标
const INDICATOR_DISPLAY_NAMES = {
  // 趋势指标
  'MA': '移动平均线',
  'EMA': '指数移动平均',
  'SMA': '简单移动平均',
  'BBI': '多空指数',
  'BOLL': '布林带',
  'SAR': '抛物线指标',
  
  // 动量指标
  'MACD': 'MACD指标',
  'RSI': '相对强弱指标',
  'KDJ': '随机指标',
  'CCI': '顺势指标',
  'WR': '威廉指标',
  'ROC': '变动率指标',
  'PSY': '心理线',
  'MTM': '动量指标',
  
  // 成交量指标
  'VOL': '成交量',
  'OBV': '能量潮',
  'PVT': '价格与成交量趋势',
  'VR': '成交量比率',
  'EMV': '简易波动指标',
  
  // 波动性指标
  'BIAS': '乖离率',
  'BRAR': '买卖意愿指标',
  'CR': '能量指标',
  'DMI': '动向指数'
};



// 获取所有可用指标并分类
export const getAvailableIndicatorsByCategory = () => {
  const supportedIndicators = getSupportedIndicators();
  
  const categorizedIndicators = {};
  
  Object.entries(INDICATOR_CATEGORIES).forEach(([categoryKey, category]) => {
    categorizedIndicators[categoryKey] = {
      ...category,
      indicators: category.indicators
        .filter(name => supportedIndicators.includes(name))
        .map(name => ({
          key: name,
          name: INDICATOR_DISPLAY_NAMES[name] || name,
          displayName: `${INDICATOR_DISPLAY_NAMES[name] || name} (${name})`,
          params: INDICATOR_DEFAULT_PARAMS[name] || [],
          category: categoryKey,
          type: category.type
        }))
    };
  });
  
  // 添加未分类的指标
  const categorizedNames = Object.values(INDICATOR_CATEGORIES)
    .flatMap(cat => cat.indicators);
  
  const uncategorized = supportedIndicators
    .filter(name => !categorizedNames.includes(name))
    .map(name => ({
      key: name,
      name: INDICATOR_DISPLAY_NAMES[name] || name,
      displayName: `${INDICATOR_DISPLAY_NAMES[name] || name} (${name})`,
      params: INDICATOR_DEFAULT_PARAMS[name] || [],
      category: 'other',
      type: 'oscillator' // 默认为副图
    }));
    
  if (uncategorized.length > 0) {
    categorizedIndicators.other = {
      name: '其他指标',
      type: 'oscillator',
      icon: '📋',
      color: '#8c8c8c',
      indicators: uncategorized
    };
  }
  
  return categorizedIndicators;
};

// 获取所有可用指标的平铺数组（用于IndicatorPanel）
export const getAvailableIndicators = () => {
  const categorized = getAvailableIndicatorsByCategory();
  const allIndicators = [];
  
  Object.values(categorized).forEach(category => {
    category.indicators.forEach(indicator => {
      allIndicators.push({
        ...indicator,
        categoryInfo: {
          name: category.name,
          type: category.type,
          color: category.color,
          icon: category.icon,
          category: indicator.category
        }
      });
    });
  });
  
  return allIndicators;
};



// 获取指标配置信息
export const getIndicatorInfo = (indicatorKey) => {
  const allIndicators = getAvailableIndicators();
  
  return allIndicators.find(indicator => indicator.key === indicatorKey) || null;
};

const indicatorUtils = {
  getAvailableIndicators,
  getAvailableIndicatorsByCategory,
  getIndicatorInfo
};

export default indicatorUtils;
