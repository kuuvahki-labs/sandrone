/*
 * Sandrone 节点名称规范化脚本
 *
 * 用途：作为 nodes-stage 的 script processor 使用，在不访问网络、不修改连接
 * 参数的前提下，完成信息节点过滤、连接去重、地区/线路/倍率识别、稳定排序、
 * 编号、协议标注和最终名称去重。
 *
 * 默认输出示例：
 *   🇭🇰 香港 01 IPLC 家宽 2× VLESS
 *
 * 处理顺序：提取元数据 -> 过滤 -> 连接去重 -> 稳定排序 -> 命名 -> 名称去重。
 * 最终名称重复时默认保留排序后的第一个节点，直接删除其余节点；不会添加随机后缀。
 *
 * 常用参数（通过 ProcessorSpec.params.args 或请求参数传入）：
 *   filter_info=true             过滤流量、到期、公告等信息节点
 *   include_regex=[]             至少匹配一个正则才保留，字符串或字符串数组
 *   exclude_regex=[]             匹配任一正则即删除，字符串或字符串数组
 *   multiplier_filter=all        all / normal / high
 *   unknown_region=keep          keep / drop
 *   dedupe_connection=true       删除连接配置完全相同的节点
 *   name_conflict=drop           drop / error
 *   sort=true                    按地区和提取出的元数据稳定排序
 *   region_style=zh              zh / code / en
 *   show_flag=true               是否显示国旗
 *   prefix=""                    固定名称前缀
 *   source_label=""              模板来源标签；省略时取 input.context.input_name
 *   separator=" "                名称分隔符，可设置为 " · "
 *   tag_separator="/"            同类线路或特征之间的分隔符
 *   index_width=2                序号宽度，范围 1 到 4
 *   always_index=true            单节点地区是否仍显示 01
 *   protocol_mode=main           none / main / detailed
 *   custom_tags={}               自定义关键词映射，如 {"GPT":"AI","NF":"Netflix"}
 *   template=...                 已识别地区的名称模板
 *   unknown_template=...         未识别地区的名称模板
 *
 * 默认 template：
 *   {prefix}{separator}{flag}{separator}{region}{separator}{index}
 *   {separator}{entry}{separator}{city}{separator}{line}{separator}{features}
 *   {separator}{multiplier}{separator}{protocol}
 *
 * 模板变量：prefix、flag、region、region_code、region_en、index、entry、city、
 * line、features、multiplier、protocol、protocol_base、security、transport、
 * flow、ip_stack、source、original、separator。
 *
 * 脚本运行在 Sandrone 的受控同步 JavaScript sandbox 中，不使用 require、文件系统、
 * 子进程、环境变量、DNS、IP/ASN 查询或任意网络访问。
 *
 * 设计参考（仅提炼数据驱动、组合处理和特征识别思路，本文件为独立实现）：
 *   https://github.com/XiaoM-OVO/Mihomo-Toolkit
 *   https://github.com/sub-store-org/Sub-Store
 *   https://github.com/FengNinger/substore_rename_rule
 *   https://gist.github.com/Tleon-H/336610b0973205ef633446c7438631d8
 */

var DEFAULT_TEMPLATE = "{prefix}{separator}{flag}{separator}{region}{separator}{index}{separator}{entry}{separator}{city}{separator}{line}{separator}{features}{separator}{multiplier}{separator}{protocol}";
var DEFAULT_UNKNOWN_TEMPLATE = "{prefix}{separator}{original}{separator}{protocol}";

var PROTOCOL_NAMES = {
    ss: "SS",
    ssr: "SSR",
    vmess: "VMess",
    vless: "VLESS",
    trojan: "Trojan",
    hysteria: "HY",
    hysteria2: "HY2",
    tuic: "TUIC",
    mieru: "Mieru",
    socks: "SOCKS",
    http: "HTTP",
    wireguard: "WG",
    snell: "Snell",
    anytls: "AnyTLS"
};

// 单一地区注册表：代码、中文名、英文名。常见城市和历史写法在别名表补充。
var REGION_ROWS = [
    ["HK", "香港", "Hong Kong"],
    ["MO", "澳门", "Macao"],
    ["TW", "台湾", "Taiwan"],
    ["JP", "日本", "Japan"],
    ["KR", "韩国", "South Korea"],
    ["SG", "新加坡", "Singapore"],
    ["US", "美国", "United States"],
    ["GB", "英国", "United Kingdom"],
    ["FR", "法国", "France"],
    ["DE", "德国", "Germany"],
    ["AU", "澳大利亚", "Australia"],
    ["AE", "阿联酋", "United Arab Emirates"],
    ["AF", "阿富汗", "Afghanistan"],
    ["AL", "阿尔巴尼亚", "Albania"],
    ["DZ", "阿尔及利亚", "Algeria"],
    ["AO", "安哥拉", "Angola"],
    ["AR", "阿根廷", "Argentina"],
    ["AM", "亚美尼亚", "Armenia"],
    ["AT", "奥地利", "Austria"],
    ["AZ", "阿塞拜疆", "Azerbaijan"],
    ["BH", "巴林", "Bahrain"],
    ["BD", "孟加拉国", "Bangladesh"],
    ["BY", "白俄罗斯", "Belarus"],
    ["BE", "比利时", "Belgium"],
    ["BZ", "伯利兹", "Belize"],
    ["BJ", "贝宁", "Benin"],
    ["BT", "不丹", "Bhutan"],
    ["BO", "玻利维亚", "Bolivia"],
    ["BA", "波斯尼亚和黑塞哥维那", "Bosnia & Herzegovina"],
    ["BW", "博茨瓦纳", "Botswana"],
    ["BR", "巴西", "Brazil"],
    ["VG", "英属维尔京群岛", "British Virgin Islands"],
    ["BN", "文莱", "Brunei"],
    ["BG", "保加利亚", "Bulgaria"],
    ["BF", "布基纳法索", "Burkina Faso"],
    ["BI", "布隆迪", "Burundi"],
    ["KH", "柬埔寨", "Cambodia"],
    ["CM", "喀麦隆", "Cameroon"],
    ["CA", "加拿大", "Canada"],
    ["CV", "佛得角", "Cape Verde"],
    ["KY", "开曼群岛", "Cayman Islands"],
    ["CF", "中非共和国", "Central African Republic"],
    ["TD", "乍得", "Chad"],
    ["CL", "智利", "Chile"],
    ["CO", "哥伦比亚", "Colombia"],
    ["KM", "科摩罗", "Comoros"],
    ["CG", "刚果（布）", "Congo - Brazzaville"],
    ["CD", "刚果（金）", "Congo - Kinshasa"],
    ["CR", "哥斯达黎加", "Costa Rica"],
    ["HR", "克罗地亚", "Croatia"],
    ["CY", "塞浦路斯", "Cyprus"],
    ["CZ", "捷克", "Czechia"],
    ["DK", "丹麦", "Denmark"],
    ["DJ", "吉布提", "Djibouti"],
    ["DO", "多米尼加共和国", "Dominican Republic"],
    ["EC", "厄瓜多尔", "Ecuador"],
    ["EG", "埃及", "Egypt"],
    ["SV", "萨尔瓦多", "El Salvador"],
    ["GQ", "赤道几内亚", "Equatorial Guinea"],
    ["ER", "厄立特里亚", "Eritrea"],
    ["EE", "爱沙尼亚", "Estonia"],
    ["ET", "埃塞俄比亚", "Ethiopia"],
    ["FJ", "斐济", "Fiji"],
    ["FI", "芬兰", "Finland"],
    ["GA", "加蓬", "Gabon"],
    ["GM", "冈比亚", "Gambia"],
    ["GE", "格鲁吉亚", "Georgia"],
    ["GH", "加纳", "Ghana"],
    ["GR", "希腊", "Greece"],
    ["GL", "格陵兰", "Greenland"],
    ["GT", "危地马拉", "Guatemala"],
    ["GN", "几内亚", "Guinea"],
    ["GY", "圭亚那", "Guyana"],
    ["HT", "海地", "Haiti"],
    ["HN", "洪都拉斯", "Honduras"],
    ["HU", "匈牙利", "Hungary"],
    ["IS", "冰岛", "Iceland"],
    ["IN", "印度", "India"],
    ["ID", "印度尼西亚", "Indonesia"],
    ["IR", "伊朗", "Iran"],
    ["IQ", "伊拉克", "Iraq"],
    ["IE", "爱尔兰", "Ireland"],
    ["IM", "马恩岛", "Isle of Man"],
    ["IL", "以色列", "Israel"],
    ["IT", "意大利", "Italy"],
    ["CI", "科特迪瓦", "Côte d’Ivoire"],
    ["JM", "牙买加", "Jamaica"],
    ["JO", "约旦", "Jordan"],
    ["KZ", "哈萨克斯坦", "Kazakhstan"],
    ["KE", "肯尼亚", "Kenya"],
    ["KW", "科威特", "Kuwait"],
    ["KG", "吉尔吉斯斯坦", "Kyrgyzstan"],
    ["LA", "老挝", "Laos"],
    ["LV", "拉脱维亚", "Latvia"],
    ["LB", "黎巴嫩", "Lebanon"],
    ["LS", "莱索托", "Lesotho"],
    ["LR", "利比里亚", "Liberia"],
    ["LY", "利比亚", "Libya"],
    ["LT", "立陶宛", "Lithuania"],
    ["LU", "卢森堡", "Luxembourg"],
    ["MK", "北马其顿", "North Macedonia"],
    ["MG", "马达加斯加", "Madagascar"],
    ["MW", "马拉维", "Malawi"],
    ["MY", "马来西亚", "Malaysia"],
    ["MV", "马尔代夫", "Maldives"],
    ["ML", "马里", "Mali"],
    ["MT", "马耳他", "Malta"],
    ["MR", "毛里塔尼亚", "Mauritania"],
    ["MU", "毛里求斯", "Mauritius"],
    ["MX", "墨西哥", "Mexico"],
    ["MD", "摩尔多瓦", "Moldova"],
    ["MC", "摩纳哥", "Monaco"],
    ["MN", "蒙古", "Mongolia"],
    ["ME", "黑山", "Montenegro"],
    ["MA", "摩洛哥", "Morocco"],
    ["MZ", "莫桑比克", "Mozambique"],
    ["MM", "缅甸", "Myanmar (Burma)"],
    ["NA", "纳米比亚", "Namibia"],
    ["NP", "尼泊尔", "Nepal"],
    ["NL", "荷兰", "Netherlands"],
    ["NZ", "新西兰", "New Zealand"],
    ["NI", "尼加拉瓜", "Nicaragua"],
    ["NE", "尼日尔", "Niger"],
    ["NG", "尼日利亚", "Nigeria"],
    ["KP", "朝鲜", "North Korea"],
    ["NO", "挪威", "Norway"],
    ["OM", "阿曼", "Oman"],
    ["PK", "巴基斯坦", "Pakistan"],
    ["PA", "巴拿马", "Panama"],
    ["PY", "巴拉圭", "Paraguay"],
    ["PE", "秘鲁", "Peru"],
    ["PH", "菲律宾", "Philippines"],
    ["PT", "葡萄牙", "Portugal"],
    ["PR", "波多黎各", "Puerto Rico"],
    ["QA", "卡塔尔", "Qatar"],
    ["RO", "罗马尼亚", "Romania"],
    ["RU", "俄罗斯", "Russia"],
    ["RW", "卢旺达", "Rwanda"],
    ["SM", "圣马力诺", "San Marino"],
    ["SA", "沙特阿拉伯", "Saudi Arabia"],
    ["SN", "塞内加尔", "Senegal"],
    ["RS", "塞尔维亚", "Serbia"],
    ["SL", "塞拉利昂", "Sierra Leone"],
    ["SK", "斯洛伐克", "Slovakia"],
    ["SI", "斯洛文尼亚", "Slovenia"],
    ["SO", "索马里", "Somalia"],
    ["ZA", "南非", "South Africa"],
    ["ES", "西班牙", "Spain"],
    ["LK", "斯里兰卡", "Sri Lanka"],
    ["SD", "苏丹", "Sudan"],
    ["SR", "苏里南", "Suriname"],
    ["SZ", "斯威士兰", "Eswatini"],
    ["SE", "瑞典", "Sweden"],
    ["CH", "瑞士", "Switzerland"],
    ["SY", "叙利亚", "Syria"],
    ["TJ", "塔吉克斯坦", "Tajikistan"],
    ["TZ", "坦桑尼亚", "Tanzania"],
    ["TH", "泰国", "Thailand"],
    ["TG", "多哥", "Togo"],
    ["TO", "汤加", "Tonga"],
    ["TT", "特立尼达和多巴哥", "Trinidad & Tobago"],
    ["TN", "突尼斯", "Tunisia"],
    ["TR", "土耳其", "Türkiye"],
    ["TM", "土库曼斯坦", "Turkmenistan"],
    ["VI", "美属维尔京群岛", "U.S. Virgin Islands"],
    ["UG", "乌干达", "Uganda"],
    ["UA", "乌克兰", "Ukraine"],
    ["UY", "乌拉圭", "Uruguay"],
    ["UZ", "乌兹别克斯坦", "Uzbekistan"],
    ["VE", "委内瑞拉", "Venezuela"],
    ["VN", "越南", "Vietnam"],
    ["YE", "也门", "Yemen"],
    ["ZM", "赞比亚", "Zambia"],
    ["ZW", "津巴布韦", "Zimbabwe"],
    ["AD", "安道尔", "Andorra"],
    ["RE", "留尼汪", "Réunion"],
    ["PL", "波兰", "Poland"],
    ["GU", "关岛", "Guam"],
    ["VA", "梵蒂冈", "Vatican City"],
    ["LI", "列支敦士登", "Liechtenstein"],
    ["CW", "库拉索", "Curaçao"],
    ["SC", "塞舌尔", "Seychelles"],
    ["AQ", "南极洲", "Antarctica"],
    ["GI", "直布罗陀", "Gibraltar"],
    ["CU", "古巴", "Cuba"],
    ["FO", "法罗群岛", "Faroe Islands"],
    ["AX", "奥兰群岛", "Åland Islands"],
    ["BM", "百慕大", "Bermuda"],
    ["TL", "东帝汶", "Timor-Leste"]
];

var REGION_ALIASES = {
    HK: ["Hongkong", "HKG", "九龙", "九龍", "新界", "沙田", "荃湾", "荃灣", "葵涌"],
    MO: ["Macau", "MAC", "澳門"],
    TW: ["TWN", "TPE", "台灣", "臺灣", "台北", "臺北", "新北", "台中", "臺中", "台南", "臺南", "高雄", "彰化", "Taipei"],
    JP: ["JPN", "NRT", "HND", "KIX", "TYO", "OSA", "东京", "東京", "大阪", "大坂", "埼玉", "关西", "關西", "Tokyo", "Osaka", "Kansai"],
    KR: ["Korea", "KOR", "ICN", "首尔", "首爾", "春川", "仁川", "Seoul", "Chuncheon"],
    SG: ["SGP", "SIN", "狮城", "獅城"],
    US: ["USA", "LAX", "SFO", "SJC", "SEA", "NYC", "JFK", "EWR", "IAD", "ATL", "ORD", "MIA", "DFW", "洛杉矶", "洛杉磯", "圣何塞", "聖何塞", "硅谷", "矽谷", "西雅图", "西雅圖", "纽约", "紐約", "芝加哥", "达拉斯", "達拉斯", "迈阿密", "邁阿密", "波特兰", "波特蘭", "波士顿", "波士頓", "Los Angeles", "San Jose", "Silicon Valley"],
    GB: ["UK", "GBR", "LHR", "MAN", "Britain", "England", "伦敦", "倫敦", "曼彻斯特", "曼徹斯特", "London", "Manchester"],
    FR: ["FRA", "CDG", "MRS", "巴黎", "马赛", "馬賽", "Paris", "Marseille"],
    DE: ["DEU", "BER", "MUC", "法兰克福", "法蘭克福", "柏林", "慕尼黑", "Frankfurt", "Berlin", "Munich"],
    AU: ["AUS", "SYD", "MEL", "澳洲", "悉尼", "墨尔本", "墨爾本", "Sydney", "Melbourne"],
    AE: ["UAE", "DXB", "迪拜", "杜拜", "Dubai", "阿拉伯联合酋长国", "阿拉伯聯合大公國"],
    CA: ["CAN", "YVR", "YYZ", "YUL", "温哥华", "溫哥華", "多伦多", "多倫多", "蒙特利尔", "蒙特利爾", "Vancouver", "Toronto", "Montreal"],
    MY: ["MYS", "KUL", "马来", "馬來", "吉隆坡", "Kuala Lumpur"],
    TH: ["THA", "BKK", "泰國", "曼谷", "Bangkok"],
    VN: ["VNM", "HAN", "SGN", "河内", "河內", "胡志明", "Hanoi"],
    PH: ["PHL", "MNL", "菲律賓", "马尼拉", "馬尼拉", "Manila"],
    ID: ["IDN", "JKT", "CGK", "印尼", "印度尼西亞", "雅加达", "雅加達", "Jakarta"],
    IN: ["IND", "BOM", "DEL", "孟买", "孟買", "Mumbai"],
    NL: ["NLD", "AMS", "荷蘭", "阿姆斯特丹", "Holland", "Amsterdam"],
    CH: ["CHE", "ZRH", "苏黎世", "蘇黎世", "Zurich"],
    RU: ["RUS", "MOW", "SVO", "俄羅斯", "莫斯科", "Moscow"],
    TR: ["TUR", "IST", "Turkey", "伊斯坦布尔", "伊斯坦堡", "Istanbul"],
    NZ: ["NZL", "AKL", "紐西蘭", "新西蘭", "奥克兰", "奧克蘭", "Auckland"],
    ZA: ["ZAF", "JNB", "约翰内斯堡", "約翰尼斯堡", "Johannesburg"]
};

var ENTRY_TAGS = [
    {label: "深港", pattern: /深港/i}, {label: "沪港", pattern: /沪港|滬港/i},
    {label: "广港", pattern: /广港|廣港/i}, {label: "京港", pattern: /京港/i},
    {label: "杭港", pattern: /杭港/i}, {label: "深日", pattern: /深日/i},
    {label: "沪日", pattern: /沪日|滬日/i}, {label: "广日", pattern: /广日|廣日/i},
    {label: "深美", pattern: /深美/i}, {label: "沪美", pattern: /沪美|滬美/i},
    {label: "广美", pattern: /广美|廣美/i}
];

var CITY_TAGS = [
    {label: "洛杉矶", pattern: /洛杉矶|洛杉磯|Los\s*Angeles|\bLAX\b/i},
    {label: "圣何塞", pattern: /圣何塞|聖何塞|San\s*Jose|\bSJC\b/i},
    {label: "西雅图", pattern: /西雅图|西雅圖|Seattle|\bSEA\b/i},
    {label: "纽约", pattern: /纽约|紐約|New\s*York|\bNYC\b|\bJFK\b/i},
    {label: "东京", pattern: /东京|東京|Tokyo|\bNRT\b|\bHND\b/i},
    {label: "大阪", pattern: /大阪|大坂|Osaka|\bKIX\b/i},
    {label: "首尔", pattern: /首尔|首爾|Seoul|\bICN\b/i},
    {label: "伦敦", pattern: /伦敦|倫敦|London|\bLHR\b/i},
    {label: "法兰克福", pattern: /法兰克福|法蘭克福|Frankfurt/i},
    {label: "巴黎", pattern: /巴黎|Paris|\bCDG\b/i},
    {label: "阿姆斯特丹", pattern: /阿姆斯特丹|Amsterdam|\bAMS\b/i},
    {label: "悉尼", pattern: /悉尼|Sydney|\bSYD\b/i},
    {label: "墨尔本", pattern: /墨尔本|墨爾本|Melbourne|\bMEL\b/i},
    {label: "新加坡", pattern: /新加坡|Singapore|\bSIN\b/i}
];

var LINE_TAGS = [
    {label: "IEPL", pattern: /\bIEPL\b/i},
    {label: "IPLC", pattern: /\bIPLC\b/i},
    {label: "CN2 GIA", pattern: /\bCN2[\s_-]*GIA\b|\bGIA\b/i},
    {label: "CN2", pattern: /\bCN2\b/i, blockedBy: "CN2 GIA"},
    {label: "CMIN2", pattern: /\bCMIN2\b/i},
    {label: "CMI", pattern: /\bCMI\b/i, blockedBy: "CMIN2"},
    {label: "9929", pattern: /\b9929\b/i},
    {label: "4837", pattern: /\b4837\b/i},
    {label: "163", pattern: /(?:^|\D)163(?:\D|$)/i},
    {label: "BGP", pattern: /\bBGP\b/i},
    {label: "Anycast", pattern: /\bAnycast\b|任播/i},
    {label: "电信", pattern: /电信|電信|China\s*Telecom|\bCT\b/i},
    {label: "联通", pattern: /联通|聯通|China\s*Unicom|\bCU\b/i},
    {label: "移动", pattern: /移动|移動|China\s*Mobile|\bCMCC\b/i},
    {label: "专线", pattern: /专线|專線|Dedicated/i},
    {label: "中转", pattern: /中转|中轉|Transit/i},
    {label: "直连", pattern: /直连|直連|Direct/i}
];

var FEATURE_TAGS = [
    {label: "家宽", pattern: /家宽|家寬|住宅|Residential/i},
    {label: "商宽", pattern: /商宽|商寬|Business\s*Broadband/i},
    {label: "原生", pattern: /原生|Native/i},
    {label: "游戏", pattern: /游戏|遊戲|Gaming|\bGame\b/i},
    {label: "CF", pattern: /Cloudflare|\bCF\b/i},
    {label: "LB", pattern: /负载均衡|負載均衡|Load\s*Balance|\bLB\b/i},
    {label: "UDP", pattern: /\bUDP\b/i},
    {label: "GPT", pattern: /ChatGPT|OpenAI|\bGPT\b/i},
    {label: "NF", pattern: /Netflix|奈飞|奈飛|\bNF\b/i},
    {label: "Disney+", pattern: /Disney\+?/i},
    {label: "TikTok", pattern: /TikTok|抖音/i},
    {label: "Kern", pattern: /核心|\bKern\b/i},
    {label: "Edge", pattern: /边缘|邊緣|\bEdge\b/i},
    {label: "Pro", pattern: /高级|高級|\bPro\b/i},
    {label: "Std", pattern: /标准|標準|\bStd\b/i},
    {label: "Exp", pattern: /实验|實驗|\bExp\b/i},
    {label: "Biz", pattern: /\bBiz\b/i},
    {label: "Fam", pattern: /\bFam\b/i},
    {label: "Buy", pattern: /购物|購物|\bBuy\b/i},
    {label: "Zx", pattern: /\bZx\b/i}
];

var INFORMATION_PATTERNS = [
    /(?:剩余|可用|已用|总计|套餐)\s*流量|流量\s*(?:剩余|重置|到期|已用)|remaining\s+(?:data|traffic)|data\s+(?:left|remaining)/i,
    /(?:到期|过期|有效期|失效)\s*(?:时间|日期)?|expire[sd]?\s*(?:at|on|date)?|expiration\s*(?:date)?/i,
    /(?:下次|距离)?\s*(?:流量)?\s*重置|reset\s*(?:date|traffic|data)/i,
    /(?:订阅|配置)\s*(?:更新|更新时间)|(?:更新|刷新)\s*(?:订阅|配置)|subscription\s*(?:update|updated|refresh)/i,
    /^(?:https?:\/\/|www\.)\S+$/i,
    /^(?:官网|官方网站|官方地址|订阅地址|订阅链接|客服|工单|邮箱|official\s+(?:site|website)|subscription\s+(?:url|link))(?:\b|\s|[:：])/i,
    /^(?:公告|通知|提示|节点说明|使用说明|维护通知|announcement|notice|instructions?)(?:\b|\s|[:：])/i,
    /(?:维护中|正在维护|under\s+maintenance|maintenance\s+notice)/i
];

var NON_CONNECTION_FIELDS = {
    id: true, name: true, tags: true, meta: true, lossy: true, warnings: true
};

var TEMPLATE_VARIABLES = {
    prefix: true, flag: true, region: true, region_code: true, region_en: true,
    index: true, entry: true, city: true, line: true, features: true,
    multiplier: true, protocol: true, protocol_base: true, security: true,
    transport: true, flow: true, ip_stack: true, source: true, original: true,
    separator: true
};

var REGIONS = buildRegions();

function main(input, api) {
    input = input || {};
    var options = readOptions(input.args || {}, input.context || {});
    var sourceNodes = Array.isArray(input.nodes) ? input.nodes : [];
    var items = [];
    var seenConnections = Object.create(null);
    var counts = {information: 0, filtered: 0, connection: 0, name: 0};

    for (var index = 0; index < sourceNodes.length; index += 1) {
        var node = sourceNodes[index];
        if (!node || typeof node !== "object") {
            continue;
        }
        var original = cleanText(node.name) || fallbackName(node);
        var metadata = extractMetadata(node, original, options);

        if (options.filterInfo && isInformationNode(original)) {
            counts.information += 1;
            continue;
        }
        if (options.unknownRegion === "drop" && !metadata.region) {
            counts.filtered += 1;
            continue;
        }
        if (options.multiplierFilter === "normal" && metadata.multiplier.value > 1) {
            counts.filtered += 1;
            continue;
        }
        if (options.multiplierFilter === "high" && metadata.multiplier.value <= 1) {
            counts.filtered += 1;
            continue;
        }
        if (!matchesFilters(metadata.searchText, options.includePatterns, options.excludePatterns)) {
            counts.filtered += 1;
            continue;
        }
        if (options.dedupeConnection) {
            var fingerprint = connectionFingerprint(node);
            if (seenConnections[fingerprint]) {
                counts.connection += 1;
                continue;
            }
            seenConnections[fingerprint] = true;
        }
        items.push({node: node, position: index, metadata: metadata});
    }

    if (options.sort) {
        items.sort(compareItems);
    }

    var regionCounts = countRegions(items);
    var regionIndexes = Object.create(null);
    var seenNames = Object.create(null);
    var output = [];

    for (var outputIndex = 0; outputIndex < items.length; outputIndex += 1) {
        var item = items[outputIndex];
        var values = templateValues(item, options, regionCounts, regionIndexes);
        var template = item.metadata.region ? options.template : options.unknownTemplate;
        var name = renderTemplate(template, values, options.separator);

        if (seenNames[name]) {
            counts.name += 1;
            if (options.nameConflict === "error") {
                throw new Error("最终节点名称重复: " + name);
            }
            continue;
        }
        seenNames[name] = true;
        item.node.name = name;
        output.push(item.node);
    }

    input.nodes = output;
    reportCounts(api, counts, sourceNodes.length, output.length);
    return input;
}

function readOptions(args, context) {
    return {
        filterInfo: readBoolean(args.filter_info, true, "filter_info"),
        includePatterns: readPatterns(args.include_regex, "include_regex"),
        excludePatterns: readPatterns(args.exclude_regex, "exclude_regex"),
        multiplierFilter: readEnum(args.multiplier_filter, "all", ["all", "normal", "high"], "multiplier_filter"),
        unknownRegion: readEnum(args.unknown_region, "keep", ["keep", "drop"], "unknown_region"),
        dedupeConnection: readBoolean(args.dedupe_connection, true, "dedupe_connection"),
        nameConflict: readEnum(args.name_conflict, "drop", ["drop", "error"], "name_conflict"),
        sort: readBoolean(args.sort, true, "sort"),
        regionStyle: readEnum(args.region_style, "zh", ["zh", "code", "en"], "region_style"),
        showFlag: readBoolean(args.show_flag, true, "show_flag"),
        prefix: cleanText(readString(args.prefix, "")),
        sourceLabel: cleanText(readString(args.source_label, context.input_name || "")),
        separator: readString(args.separator, " "),
        tagSeparator: readString(args.tag_separator, "/"),
        indexWidth: readInteger(args.index_width, 2, 1, 4, "index_width"),
        alwaysIndex: readBoolean(args.always_index, true, "always_index"),
        protocolMode: readEnum(args.protocol_mode, "main", ["none", "main", "detailed"], "protocol_mode"),
        customTags: readCustomTags(args.custom_tags),
        template: readTemplate(args.template, DEFAULT_TEMPLATE, "template"),
        unknownTemplate: readTemplate(args.unknown_template, DEFAULT_UNKNOWN_TEMPLATE, "unknown_template")
    };
}

function readBoolean(value, fallback, name) {
    if (value === undefined || value === null || value === "") return fallback;
    if (typeof value === "boolean") return value;
    if (typeof value === "number" && (value === 0 || value === 1)) return value === 1;
    if (typeof value === "string") {
        var normalized = value.trim().toLowerCase();
        if (normalized === "true" || normalized === "1" || normalized === "yes" || normalized === "on") return true;
        if (normalized === "false" || normalized === "0" || normalized === "no" || normalized === "off") return false;
    }
    throw new Error(name + " 必须是布尔值");
}

function readEnum(value, fallback, allowed, name) {
    if (value === undefined || value === null || value === "") return fallback;
    var normalized = String(value).trim().toLowerCase();
    if (allowed.indexOf(normalized) === -1) {
        throw new Error(name + " 必须是 " + allowed.join(" / "));
    }
    return normalized;
}

function readInteger(value, fallback, minimum, maximum, name) {
    if (value === undefined || value === null || value === "") return fallback;
    var parsed = Number(value);
    if (!isFinite(parsed) || Math.floor(parsed) !== parsed || parsed < minimum || parsed > maximum) {
        throw new Error(name + " 必须是 " + minimum + " 到 " + maximum + " 的整数");
    }
    return parsed;
}

function readString(value, fallback) {
    return value === undefined || value === null ? fallback : String(value);
}

function readPatterns(value, name) {
    if (value === undefined || value === null || value === "") return [];
    var values = value;
    if (typeof value === "string" && value.trim().charAt(0) === "[") {
        try { values = JSON.parse(value); } catch (error) { throw new Error(name + " 的 JSON 数组无效: " + error.message); }
    }
    if (!Array.isArray(values)) values = [values];
    var patterns = [];
    for (var index = 0; index < values.length; index += 1) {
        if (typeof values[index] !== "string" || values[index] === "") {
            throw new Error(name + " 必须是非空字符串或字符串数组");
        }
        try { patterns.push(new RegExp(values[index], "i")); }
        catch (error) { throw new Error(name + " 包含无效正则: " + error.message); }
    }
    return patterns;
}

function readCustomTags(value) {
    if (value === undefined || value === null || value === "") return [];
    var object = value;
    if (typeof value === "string") {
        try { object = JSON.parse(value); } catch (error) { throw new Error("custom_tags 的 JSON 对象无效: " + error.message); }
    }
    if (!object || typeof object !== "object" || Array.isArray(object)) {
        throw new Error("custom_tags 必须是关键词到标签的对象");
    }
    var tags = [];
    var keys = Object.keys(object);
    for (var index = 0; index < keys.length; index += 1) {
        var key = keys[index];
        if (!key) throw new Error("custom_tags 不能包含空关键词");
        tags.push({match: key, label: cleanText(object[key]) || key});
    }
    return tags;
}

function readTemplate(value, fallback, name) {
    var template = value === undefined || value === null || value === "" ? fallback : String(value);
    template.replace(/\{([^{}]+)\}/g, function (_, key) {
        if (!TEMPLATE_VARIABLES[key]) throw new Error(name + " 包含未知变量: " + key);
        return _;
    });
    return template;
}

function buildRegions() {
    var regions = [];
    for (var index = 0; index < REGION_ROWS.length; index += 1) {
        var row = REGION_ROWS[index];
        var flag = countryFlag(row[0]);
        var aliases = [row[0], row[1], row[2], flag];
        if (REGION_ALIASES[row[0]]) aliases = aliases.concat(REGION_ALIASES[row[0]]);
        regions.push({code: row[0], zh: row[1], en: row[2], flag: flag, aliases: aliases, order: index});
    }
    return regions;
}

function countryFlag(code) {
    var first = 0xDDE6 + code.charCodeAt(0) - 65;
    var second = 0xDDE6 + code.charCodeAt(1) - 65;
    return String.fromCharCode(0xD83C, first, 0xD83C, second);
}

function extractMetadata(node, original, options) {
    var region = detectRegion(original);
    var lines = detectTags(original, LINE_TAGS);
    var features = detectTags(original, FEATURE_TAGS);
    appendUnique(features, detectCustomTags(original, options.customTags));
    var multiplier = detectMultiplier(original);
    var protocol = protocolMetadata(node);
    var entry = detectFirst(original, ENTRY_TAGS);
    var city = detectFirst(original, CITY_TAGS);
    var ipStack = detectIPStack(node.server);
    var searchParts = [original, region && region.zh, region && region.code, region && region.en,
        entry, city, lines.join(" "), features.join(" "), multiplier.label,
        protocol.base, protocol.security, protocol.transport, protocol.flow, ipStack];
    return {
        original: original,
        region: region,
        entry: entry,
        city: city,
        lines: lines,
        features: features,
        multiplier: multiplier,
        protocol: protocol,
        ipStack: ipStack,
        searchText: searchParts.filter(Boolean).join(" ")
    };
}

function detectRegion(name) {
    for (var regionIndex = 0; regionIndex < REGIONS.length; regionIndex += 1) {
        var region = REGIONS[regionIndex];
        var aliases = region.aliases.slice().sort(function (left, right) { return right.length - left.length; });
        for (var aliasIndex = 0; aliasIndex < aliases.length; aliasIndex += 1) {
            if (containsAlias(name, aliases[aliasIndex])) return region;
        }
    }
    return null;
}

function containsAlias(name, alias) {
    if (!alias) return false;
    if (/^[A-Za-z0-9]{2,4}$/.test(alias)) {
        return new RegExp("(^|[^A-Za-z0-9])" + escapeRegExp(alias) + "($|[^A-Za-z0-9])", "i").test(name);
    }
    return String(name).toLocaleLowerCase().indexOf(String(alias).toLocaleLowerCase()) !== -1;
}

function detectFirst(name, definitions) {
    for (var index = 0; index < definitions.length; index += 1) {
        if (definitions[index].pattern.test(name)) return definitions[index].label;
    }
    return "";
}

function detectTags(name, definitions) {
    var labels = [];
    for (var index = 0; index < definitions.length; index += 1) {
        var definition = definitions[index];
        if (definition.blockedBy && labels.indexOf(definition.blockedBy) !== -1) continue;
        if (definition.pattern.test(name) && labels.indexOf(definition.label) === -1) labels.push(definition.label);
    }
    return labels;
}

function detectCustomTags(name, definitions) {
    var labels = [];
    for (var index = 0; index < definitions.length; index += 1) {
        if (name.indexOf(definitions[index].match) !== -1 && labels.indexOf(definitions[index].label) === -1) {
            labels.push(definitions[index].label);
        }
    }
    return labels;
}

function appendUnique(target, values) {
    for (var index = 0; index < values.length; index += 1) {
        if (target.indexOf(values[index]) === -1) target.push(values[index]);
    }
}

function detectMultiplier(name) {
    var value = 1;
    var match = String(name).match(/(?:倍率|rate)\s*[:：]?\s*(\d+(?:\.\d+)?)\s*(?:[xX×倍])?/i);
    if (!match) match = String(name).match(/(?:^|[^\d.])[xX×]\s*(\d+(?:\.\d+)?)(?:$|[^\d])/i);
    if (!match) match = String(name).match(/(?:^|[^\d.])(\d+(?:\.\d+)?)\s*(?:[xX×倍])(?:$|[^\d])/i);
    if (match) value = Number(match[1]);
    if (!match) {
        var superscript = String(name).match(/ˣ([⁰¹²³⁴⁵⁶⁷⁸⁹]+)/);
        if (superscript) value = Number(fromSuperscript(superscript[1]));
    }
    if (!isFinite(value) || value <= 0) value = 1;
    return {value: value, label: Math.abs(value - 1) < 0.0000001 ? "" : formatNumber(value) + "×"};
}

function fromSuperscript(value) {
    var digits = {"⁰": "0", "¹": "1", "²": "2", "³": "3", "⁴": "4", "⁵": "5", "⁶": "6", "⁷": "7", "⁸": "8", "⁹": "9"};
    var output = "";
    for (var index = 0; index < value.length; index += 1) output += digits[value.charAt(index)] || "";
    return output;
}

function formatNumber(value) {
    return String(Math.round(value * 1000) / 1000).replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1");
}

function protocolMetadata(node) {
    var rawType = node && node.type !== undefined && node.type !== null ? String(node.type).toLowerCase() : "";
    var base = PROTOCOL_NAMES[rawType] || (rawType ? rawType.toUpperCase() : "UNKNOWN");
    var tls = node && node.tls && typeof node.tls === "object" ? node.tls : null;
    var reality = tls && tls.reality && typeof tls.reality === "object" ? tls.reality : null;
    var security = "";
    if (reality && (reality.enabled || reality.public_key || reality.short_id)) security = "Reality";
    else if (tls && tls.enabled) security = "TLS";
    else if (node && node.snell && node.snell.shadow_tls) security = "ShadowTLS";

    var rawTransport = node && node.transport && node.transport.type ? String(node.transport.type).toLowerCase() : "";
    var transports = {
        ws: "WS", websocket: "WS", grpc: "gRPC", xhttp: "XHTTP", httpupgrade: "HTTPUpgrade",
        "http-upgrade": "HTTPUpgrade", h2: "H2", http: "HTTP", quic: "QUIC", kcp: "mKCP"
    };
    var transport = transports[rawTransport] || (rawTransport && rawTransport !== "tcp" ? rawTransport.toUpperCase() : "");
    var flow = node && String(node.flow || "").trim().toLowerCase() === "xtls-rprx-vision" ? "Vision" : "";
    var details = [base, security, transport, flow].filter(Boolean);
    return {base: base, security: security, transport: transport, flow: flow, detailed: details.join(" ")};
}

function detectIPStack(server) {
    var value = cleanText(server);
    if (!value) return "";
    if (value.indexOf(":") !== -1) return "IPv6";
    if (/^\d{1,3}(?:\.\d{1,3}){3}$/.test(value)) return "IPv4";
    return "";
}

function isInformationNode(name) {
    for (var index = 0; index < INFORMATION_PATTERNS.length; index += 1) {
        if (INFORMATION_PATTERNS[index].test(name)) return true;
    }
    return false;
}

function matchesFilters(searchText, includes, excludes) {
    var includeMatch = includes.length === 0;
    for (var includeIndex = 0; includeIndex < includes.length; includeIndex += 1) {
        if (includes[includeIndex].test(searchText)) { includeMatch = true; break; }
    }
    if (!includeMatch) return false;
    for (var excludeIndex = 0; excludeIndex < excludes.length; excludeIndex += 1) {
        if (excludes[excludeIndex].test(searchText)) return false;
    }
    return true;
}

function connectionFingerprint(node) {
    var connection = {};
    var keys = Object.keys(node || {});
    for (var index = 0; index < keys.length; index += 1) {
        if (!NON_CONNECTION_FIELDS[keys[index]]) connection[keys[index]] = node[keys[index]];
    }
    return stableStringify(connection);
}

function stableStringify(value) {
    if (value === null) return "null";
    if (typeof value !== "object") return JSON.stringify(value);
    if (Array.isArray(value)) {
        var arrayParts = [];
        for (var arrayIndex = 0; arrayIndex < value.length; arrayIndex += 1) arrayParts.push(stableStringify(value[arrayIndex]));
        return "[" + arrayParts.join(",") + "]";
    }
    var keys = Object.keys(value).sort();
    var objectParts = [];
    for (var keyIndex = 0; keyIndex < keys.length; keyIndex += 1) {
        if (value[keys[keyIndex]] !== undefined) objectParts.push(JSON.stringify(keys[keyIndex]) + ":" + stableStringify(value[keys[keyIndex]]));
    }
    return "{" + objectParts.join(",") + "}";
}

function compareItems(left, right) {
    var leftMeta = left.metadata;
    var rightMeta = right.metadata;
    if (leftMeta.region && !rightMeta.region) return -1;
    if (!leftMeta.region && rightMeta.region) return 1;
    if (leftMeta.region && rightMeta.region && leftMeta.region.order !== rightMeta.region.order) return leftMeta.region.order - rightMeta.region.order;
    var comparisons = [
        compareText(leftMeta.entry, rightMeta.entry), compareText(leftMeta.city, rightMeta.city),
        compareText(leftMeta.lines.join("/"), rightMeta.lines.join("/")),
        leftMeta.multiplier.value - rightMeta.multiplier.value,
        compareText(leftMeta.protocol.detailed, rightMeta.protocol.detailed),
        compareText(leftMeta.original, rightMeta.original)
    ];
    for (var index = 0; index < comparisons.length; index += 1) if (comparisons[index] !== 0) return comparisons[index];
    return left.position - right.position;
}

function compareText(left, right) {
    if (left < right) return -1;
    if (left > right) return 1;
    return 0;
}

function countRegions(items) {
    var counts = Object.create(null);
    for (var index = 0; index < items.length; index += 1) {
        if (items[index].metadata.region) {
            var code = items[index].metadata.region.code;
            counts[code] = (counts[code] || 0) + 1;
        }
    }
    return counts;
}

function templateValues(item, options, regionCounts, regionIndexes) {
    var metadata = item.metadata;
    var region = metadata.region;
    var index = "";
    if (region) {
        regionIndexes[region.code] = (regionIndexes[region.code] || 0) + 1;
        if (options.alwaysIndex || regionCounts[region.code] > 1) index = padNumber(regionIndexes[region.code], options.indexWidth);
    }
    var regionName = "";
    if (region) {
        if (options.regionStyle === "code") regionName = region.code;
        else if (options.regionStyle === "en") regionName = region.en;
        else regionName = region.zh;
    }
    var protocol = "";
    if (options.protocolMode === "main") protocol = metadata.protocol.base;
    else if (options.protocolMode === "detailed") protocol = metadata.protocol.detailed;
    var original = stripGeneratedParts(metadata.original, options.prefix, metadata.protocol);
    return {
        prefix: options.prefix,
        flag: region && options.showFlag ? region.flag : "",
        region: regionName,
        region_code: region ? region.code : "",
        region_en: region ? region.en : "",
        index: index,
        entry: metadata.entry,
        city: metadata.city,
        line: metadata.lines.join(options.tagSeparator),
        features: metadata.features.join(options.tagSeparator),
        multiplier: metadata.multiplier.label,
        protocol: protocol,
        protocol_base: metadata.protocol.base,
        security: metadata.protocol.security,
        transport: metadata.protocol.transport,
        flow: metadata.protocol.flow,
        ip_stack: metadata.ipStack,
        source: options.sourceLabel,
        original: original,
        separator: options.separator
    };
}

function stripGeneratedParts(original, prefix, protocol) {
    var value = cleanText(original);
    if (prefix && value.indexOf(prefix) === 0) {
        value = cleanText(value.slice(prefix.length).replace(/^[\s|/·•—–_-]+/, ""));
    }
    var suffixes = [protocol.detailed, protocol.base];
    for (var index = 0; index < suffixes.length; index += 1) {
        if (!suffixes[index]) continue;
        var pattern = new RegExp("(?:[\\s|/·•—–_-]+)" + escapeRegExp(suffixes[index]) + "$", "i");
        if (pattern.test(value)) {
            value = cleanText(value.replace(pattern, ""));
            break;
        }
    }
    return value || original;
}

function renderTemplate(template, values, separator) {
    var chunks = template.split("{separator}");
    var rendered = [];
    for (var index = 0; index < chunks.length; index += 1) {
        var chunk = cleanChunk(replaceVariables(chunks[index], values));
        if (chunk && rendered.indexOf(chunk) === -1) rendered.push(chunk);
    }
    var name = chunks.length > 1 ? rendered.join(separator) : cleanRenderedName(rendered.join(""), separator);
    return cleanRenderedName(name, separator) || "未命名节点";
}

function replaceVariables(template, values) {
    return template.replace(/\{([^{}]+)\}/g, function (_, key) { return values[key] === undefined || values[key] === null ? "" : String(values[key]); });
}

function cleanChunk(value) {
    return cleanText(value).replace(/^[|/·•—–_-]+|[|/·•—–_-]+$/g, "").trim();
}

function cleanRenderedName(value, separator) {
    var name = cleanText(value);
    if (!separator) return name;
    var escaped = escapeRegExp(separator);
    return name.replace(new RegExp("(?:" + escaped + "){2,}", "g"), separator).replace(new RegExp("^(?:" + escaped + ")+|(?:" + escaped + ")+$", "g"), "").trim();
}

function cleanText(value) {
    if (value === undefined || value === null) return "";
    return String(value).replace(/[\u0000-\u001f\u007f]/g, " ").replace(/\s+/g, " ").trim();
}

function fallbackName(node) {
    var server = cleanText(node && node.server) || "未命名节点";
    return server + (node && node.port ? ":" + String(node.port) : "");
}

function padNumber(value, width) {
    var output = String(value);
    while (output.length < width) output = "0" + output;
    return output;
}

function escapeRegExp(value) {
    return String(value).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function reportCounts(api, counts, before, after) {
    if (!api) return;
    if (typeof api.warn === "function") {
        if (counts.information > 0) api.warn({code: "node_normalize_information_filtered", message: "filtered information nodes: " + counts.information});
        if (counts.filtered > 0) api.warn({code: "node_normalize_filtered", message: "filtered nodes by configured rules: " + counts.filtered});
        if (counts.connection > 0) api.warn({code: "node_normalize_connection_deduped", message: "dropped duplicate connections: " + counts.connection});
        if (counts.name > 0) api.warn({code: "node_normalize_name_deduped", message: "dropped duplicate final names: " + counts.name});
    }
    if (typeof api.log === "function") api.log("normalized", before, "nodes to", after, "nodes");
}
