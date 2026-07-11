import {
  type ConfigNamingLocale,
  type ConfigRegionID,
  configRegionName,
} from "./naming";

export interface AdaptiveGroupDefinition {
  excludeFilter?: string;
  filter: string;
  id: ConfigRegionID;
  legacyNames?: readonly string[];
  name: string;
}

export interface CanonicalGroupDefinition {
  excludeFilter?: string;
  filter: string;
  legacyNames?: readonly string[];
  name: string;
}

interface RegionDefinition {
  codes: readonly string[];
  excludeTerms?: readonly string[];
  id: ConfigRegionID;
  legacyNames: readonly string[];
  terms: readonly string[];
  weight: number;
}

const REGIONS: readonly RegionDefinition[] = [
  {
    id: "hk",
    legacyNames: ["香港节点"],
    weight: 10,
    terms: ["香港", "Hong Kong", "HongKong", "🇭🇰"],
    codes: ["HK", "HKG"],
  },
  {
    id: "tw",
    legacyNames: ["台湾节点"],
    weight: 20,
    terms: ["台湾", "臺灣", "台北", "Taipei", "Taiwan", "🇹🇼"],
    codes: ["TW", "TWN", "TPE"],
  },
  {
    id: "sg",
    legacyNames: ["新加坡节点"],
    weight: 30,
    terms: ["新加坡", "狮城", "Singapore", "🇸🇬"],
    codes: ["SG", "SGP", "SIN"],
  },
  {
    id: "jp",
    legacyNames: ["日本节点"],
    weight: 40,
    terms: ["日本", "东京", "東京", "大阪", "Japan", "Tokyo", "Osaka", "🇯🇵"],
    codes: ["JP", "JPN", "NRT", "HND", "KIX"],
  },
  {
    id: "kr",
    legacyNames: ["韩国节点"],
    weight: 45,
    terms: ["韩国", "韓國", "首尔", "首爾", "Korea", "Seoul", "🇰🇷"],
    codes: ["KR", "KOR", "ICN"],
  },
  {
    id: "us",
    legacyNames: ["美国节点"],
    weight: 50,
    terms: [
      "美国", "美國", "United States", "America", "洛杉矶", "洛杉磯", "纽约", "紐約",
      "西雅图", "西雅圖", "硅谷", "🇺🇸",
    ],
    codes: ["US", "USA", "LAX", "SFO", "SJC", "SEA", "NYC", "JFK", "EWR", "IAD", "ATL", "ORD", "MIA", "DFW"],
    excludeTerms: ["美属", "美屬", "亚美尼亚", "亞美尼亞", "圣多美", "聖多美"],
  },
  {
    id: "ca",
    legacyNames: ["加拿大节点"],
    weight: 55,
    terms: ["加拿大", "Canada", "多伦多", "多倫多", "温哥华", "溫哥華", "蒙特利尔", "蒙特利爾", "🇨🇦"],
    codes: ["CA", "CAN", "YYZ", "YVR", "YUL"],
  },
  {
    id: "uk",
    legacyNames: ["英国节点"],
    weight: 60,
    terms: ["英国", "英國", "United Kingdom", "Britain", "England", "伦敦", "倫敦", "Manchester", "🇬🇧"],
    codes: ["UK", "GB", "GBR", "LHR", "MAN"],
  },
  {
    id: "de",
    legacyNames: ["德国节点"],
    weight: 70,
    terms: ["德国", "德國", "Germany", "柏林", "法兰克福", "法蘭克福", "慕尼黑", "🇩🇪"],
    codes: ["DE", "DEU", "BER", "MUC"],
    excludeTerms: ["瓜德罗普", "瓜德羅普"],
  },
  {
    id: "fr",
    legacyNames: ["法国节点"],
    weight: 80,
    terms: ["法国", "法國", "France", "巴黎", "马赛", "馬賽", "🇫🇷"],
    codes: ["FR", "FRA", "CDG", "MRS"],
    excludeTerms: ["法属", "法屬", "布基纳法索", "布基納法索", "法罗", "法羅"],
  },
  {
    id: "mo",
    legacyNames: ["澳门节点"],
    weight: 90,
    terms: ["澳门", "澳門", "Macau", "Macao", "🇲🇴"],
    codes: ["MO", "MAC"],
  },
  {
    id: "au",
    legacyNames: ["澳大利亚节点"],
    weight: 100,
    terms: ["澳大利亚", "澳大利亞", "澳洲", "Australia", "悉尼", "Sydney", "🇦🇺"],
    codes: ["AU", "AUS", "SYD", "MEL"],
  },
  {
    id: "ru",
    legacyNames: ["俄罗斯节点"],
    weight: 110,
    terms: ["俄罗斯", "俄羅斯", "Russia", "莫斯科", "Moscow", "🇷🇺"],
    codes: ["RU", "RUS", "SVO"],
    excludeTerms: ["白俄罗斯", "白俄羅斯"],
  },
  {
    id: "th",
    legacyNames: ["泰国节点"],
    weight: 120,
    terms: ["泰国", "泰國", "Thailand", "曼谷", "Bangkok", "🇹🇭"],
    codes: ["TH", "THA", "BKK"],
  },
  {
    id: "in",
    legacyNames: ["印度节点"],
    weight: 130,
    terms: ["印度", "India", "孟买", "孟買", "Mumbai", "🇮🇳"],
    codes: ["IN", "IND", "BOM", "DEL"],
    excludeTerms: ["印度洋"],
  },
  {
    id: "my",
    legacyNames: ["马来西亚节点"],
    weight: 140,
    terms: ["马来西亚", "馬來西亞", "马来", "馬來", "Malaysia", "吉隆坡", "Kuala Lumpur", "🇲🇾"],
    codes: ["MY", "MYS", "KUL"],
  },
  {
    id: "ph",
    legacyNames: ["菲律宾节点"],
    weight: 150,
    terms: ["菲律宾", "菲律賓", "Philippines", "马尼拉", "馬尼拉", "Manila", "🇵🇭"],
    codes: ["PH", "PHL", "MNL"],
  },
  {
    id: "tr",
    legacyNames: ["土耳其节点"],
    weight: 160,
    terms: ["土耳其", "Turkey", "Türkiye", "伊斯坦布尔", "伊斯坦布爾", "Istanbul", "🇹🇷"],
    codes: ["TR", "TUR", "IST"],
  },
  {
    id: "ua",
    legacyNames: ["乌克兰节点"],
    weight: 170,
    terms: ["乌克兰", "烏克蘭", "Ukraine", "基辅", "基輔", "Kyiv", "Kiev", "🇺🇦"],
    codes: ["UA", "UKR", "KBP"],
  },
  {
    id: "fi",
    legacyNames: ["芬兰节点"],
    weight: 180,
    terms: ["芬兰", "芬蘭", "Finland", "赫尔辛基", "赫爾辛基", "Helsinki", "🇫🇮"],
    codes: ["FI", "FIN", "HEL"],
  },
  {
    id: "ar",
    legacyNames: ["阿根廷节点"],
    weight: 190,
    terms: ["阿根廷", "Argentina", "布宜诺斯艾利斯", "布宜諾斯艾利斯", "Buenos Aires", "🇦🇷"],
    codes: ["AR", "ARG", "EZE"],
  },
  {
    id: "eg",
    legacyNames: ["埃及节点"],
    weight: 200,
    terms: ["埃及", "Egypt", "开罗", "開羅", "Cairo", "🇪🇬"],
    codes: ["EG", "EGY", "CAI"],
  },
];

export const ADAPTIVE_REGION_GROUPS: readonly AdaptiveGroupDefinition[] = [...REGIONS]
  .sort((left, right) => left.weight - right.weight)
  .map((region) => ({
    id: region.id,
    name: configRegionName(region.id, "en-US"),
    legacyNames: [configRegionName(region.id, "zh-CN"), ...region.legacyNames],
    filter: regionFilter(region.terms, region.codes),
    ...(region.excludeTerms?.length ? { excludeFilter: termFilter(region.excludeTerms) } : {}),
  }));

export const CANONICAL_ADAPTIVE_GROUP_DEFINITIONS: readonly CanonicalGroupDefinition[] = [
  ...ADAPTIVE_REGION_GROUPS.map((region) => ({
    name: region.name,
    legacyNames: region.legacyNames,
    filter: region.filter,
    ...(region.excludeFilter ? { excludeFilter: region.excludeFilter } : {}),
  })),
];

export function adaptiveRegionName(id: ConfigRegionID, locale: ConfigNamingLocale): string {
  return configRegionName(id, locale);
}

function regionFilter(terms: readonly string[], codes: readonly string[]): string {
  return `(?i)(?:${[
    ...terms.map(escapeRegex),
    ...codes.map((code) => `\\b${escapeRegex(code)}\\b`),
  ].join("|")})`;
}

function termFilter(terms: readonly string[]): string {
  return `(?i)(?:${terms.map(escapeRegex).join("|")})`;
}

function escapeRegex(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
