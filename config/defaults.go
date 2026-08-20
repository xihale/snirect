package config

// SampleConfigTOML is the default sample configuration in TOML format,
// written directly into Go code.
const SampleConfigTOML = `# Snirect Configuration File
# Restart the program for changes to take effect.
# If an option is commented out, the program will use its internal default value.

# [Certificate Verification Policy]
# Controls how Snirect verifies the remote server's certificate.
# Possible values:
#   false    - Disable hostname verification. High compatibility but susceptible to MITM attacks.
#   true     - (Default) Loose verification. Matches subdomains more flexibly (e.g., a.test.com matches b.test.com).
#   "strict" - Standard strict verification. Recommended for high security.
#   [ "domain1.com", "domain2.com" ] - List of domains to enable verification for.
#
# 证书域名校验策略
# 控制 Snirect 如何验证远程服务器的证书域名。
# 可选值:
#   false    - 不校验证书域名。兼容性最高，但存在中间人攻击风险。
#   true     - (默认) 宽松校验。允许子域名间的通配匹配。
#   "strict" - 标准严格校验。建议追求安全的用户开启。
#   [ "domain1.com", "domain2.com" ] - 仅对列表内的域名开启校验。
# check_hostname = true

# [System Proxy]
# Automatically set Snirect as the system proxy on startup.
#
# 自动设置系统代理
# 启动时自动将 Snirect 设置为系统代理。
# set_proxy = true

# [Root CA Installation]
# Policy for automatically importing and trusting the Snirect Root CA.
# Options:
#   "auto"   - (Default) Install only if the CA is not already trusted.
#   "always" - Force re-installation on every startup.
#   "never"  - Disable automatic installation completely.
#
# 根证书安装策略
# 是否自动将 Snirect 的根证书导入并信任到系统中。
# 可选值:
#   "auto"   - (默认) 仅在未检测到已信任的证书时尝试安装。
#   "always" - 每次启动都强制尝试重新安装。
#   "never"  - 禁用自动安装，需手动处理证书信任。
# ca_install = "auto"

# [IPv6 Support]
# Enable or disable IPv6 support for proxy connections.
#
# IPv6 支持
# 是否开启对 IPv6 的支持。
# ipv6 = false

# [EDNS Client Subnet (ECS)]
# Provides your network subnet info to DNS servers to get geographically closer IP results.
# Options:
#   "auto"   - Automatically generate based on your public IP.
#   "CIDR"   - Use a specific subnet (e.g., "218.102.0.0/24").
#   ""       - (Default) Disable ECS.
#
# EDNS 客户端子网 (ECS)
# 向 DNS 服务器提供您的子网信息，以便获取地理位置更近的解析结果。
# 可选值:
#   "auto"   - 根据您的公网 IP 自动生成。
#   "子网"   - 使用指定的 CIDR 子网 (例如 "218.102.0.0/24")。
#   ""       - (默认) 禁用 ECS 功能。
# ecs = ""

# [DNS Settings]
# Custom DNS resolution settings.
#
# DNS 设置
# 自定义域名解析配置。
#
[DNS]
# List of upstream DNS servers. Supports DoH, DoT, UDP, and TCP.
# 上游 DNS 服务器列表。支持 DoH、DoT、UDP、TCP 格式。
# nameserver = [
#     "https://dnschina1.soraharu.com/dns-query",
#     "tls://223.5.5.5"
# ]

# Bootstrap DNS servers used to resolve the hostnames of the nameservers above.
# 引导 DNS 服务器，用于解析上述加密 DNS 服务器自身的域名。
# bootstrap_dns = ["tls://223.5.5.5"]

# [DNS IP Preference]
# Controls how Snirect selects between IPv6 and IPv4 addresses when both are available.
# Requires ipv6 = true to function (IPv6 must be enabled).
#
# DNS IP 优选策略
# 控制 Snirect 在有 IPv6 和 IPv4 地址时如何选择。
# 需要先开启 ipv6 = true 才会生效。
[preference]
# Mode: standard, fastest, ipv6, ipv4
#   standard - (Default) Prefer IPv6 if available, otherwise first available
#   fastest  - Test connection latency to all IPs and use the fastest (cached)
#   ipv6     - Always prefer IPv6 when available (no testing)
#   ipv4     - Force IPv4 only
#
# 模式: standard (标准), fastest (最快), ipv6 (强制v6), ipv4 (强制v4)
# mode = "standard"

# Timeout for each connection test in milliseconds.
# Lower = faster fallback, higher = more accurate.
# 每个连接测试的超时时间（毫秒）。
# test_timeout_ms = 500

# Maximum number of IPs to test per query.
# Set to conserve resources when many IPs are available (e.g., CDNs).
#
# 每次查询最多测试的 IP 数量。
# 当可用 IP 很多时（如 CDN）可限制资源消耗。
# max_test_ips = 10

# Preference cache TTL in seconds.
# 0 = automatically use half of the DNS record's TTL.
# Shorter TTL = adapt to network changes faster.
#
# 优选结果缓存时间（秒）。
# 0 = 自动使用 DNS 记录 TTL 的一半。
# 较短的 TTL 能更快适应网络变化。
# cache_ttl = 300

# Maximum entries in preference cache (0 = unlimited).
# PrefCache is separate from DNS cache and usually much smaller.
#
# 优选缓存的最大条目数 (0 = 不限制)。
# 优选缓存独立于 DNS 缓存，通常小得多。
# cache_size = 5000

# [Timeout Settings]
# Timeouts in seconds.
#
# 超时设置 (秒)
[timeout]
# Timeout for establishing a connection to a remote server.
# 连接远程服务器的超时时间。
# dial = 30
# Timeout for DNS queries.
# DNS 查询超时时间。
# dns = 5

# [Resource Limits]
# Settings to control resource usage.
#
# 资源限制
[limit]
# Maximum number of concurrent connections (0 = unlimited).
# 最大并发连接数 (0 表示不限制)。
# max_connections = 0
# Maximum number of entries in the DNS cache.
# DNS 缓存的最大条目数。
# dns_cache_size = 10000

# [Logging]
# Configure how Snirect records its activity.
#
# 日志配置
[log]
# Log level: "DEBUG", "INFO", "WARN", "ERROR".
# 日志级别: "DEBUG", "INFO", "WARN", "ERROR"。
# loglevel = "INFO"

# 日志文件路径。
# 如果留空，将使用以下默认系统路径：
#   Linux:   ~/.local/state/snirect/snirect.log
#   macOS:   ~/Library/Logs/snirect/snirect.log
#   Windows: %LOCALAPPDATA%\snirect\Logs\snirect.log
# logfile = ""

# [Server Settings]
# Configuration for the Snirect proxy server itself.
#
# 本地服务器设置
[server]
# Address to bind the server to. 
# Use "127.0.0.1" for local access only, "0.0.0.0" to allow access from other devices in the LAN.
# 绑定地址。
# 使用 "127.0.0.1" 仅限本机访问；使用 "0.0.0.0" 允许局域网内的其他设备连接。
# address = "127.0.0.1"

# Port number for the proxy server (1-65535).
# 代理服务器端口号 (1-65535)。
# port = 7654

# Hostname used in the generated PAC file.
# If using in a LAN, set this to the IP address of this machine.
# PAC 文件中的代理主机名。
# 如果需要在局域网内共享，请将其设置为本机的局域网 IP。
# pac_host = "127.0.0.1"
`

// DefaultPAC is the default PAC script template.
const DefaultPAC = `var domains = {
  "*audiomack.com": 1,
  "*google*": 1,
  "*google.com": 1,
  "*nicovideo.jp": 1,
  "*twitch.tv": 1,
  "android.com": 1,
  "apkmirror.com": 1,
  "archive.org": 1,
  "archiveofourown.org": 1,
  "audiomack.com": 1,
  "bamgrid.com": 1,
  "bbc.com": 1,
  "behance.net": 1,
  "bilibili.tv": 1,
  "blogger.com": 1,
  "cdn-telegram.org": 1,
  "character.ai": 1,
  "claude.ai": 1,
  "co.jp": 1,
  "co.nz": 1,
  "co.uk": 1,
  "com.tw": 1,
  "dailymotion.com": 1,
  "discord.com": 1,
  "discord.gg": 1,
  "discordapp.com": 1,
  "discordapp.net": 1,
  "disneyplus.com": 1,
  "docker.com": 1,
  "dropbox.com": 1,
  "duckduckgo.com": 1,
  "e-hentai.org": 1,
  "ecosia.org": 1,
  "ehgt.org": 1,
  "ehtracker.org": 1,
  "ehwiki.org": 1,
  "etsy.com": 1,
  "exhentai.org": 1,
  "eyny.com": 1,
  "f-droid.org": 1,
  "facebook.com": 1,
  "fanbox.cc": 1,
  "fbcdn.net": 1,
  "fdroid.org": 1,
  "flickr.com": 1,
  "gelbooru.com": 1,
  "ggpht.com": 1,
  "github.com": 1,
  "githubusercontent.com": 1,
  "google.com": 1,
  "googlevideo.com": 1,
  "gravatar.com": 1,
  "greasyfork.org": 1,
  "gstatic.com": 1,
  "hentaiverse.org": 1,
  "huggingface.co": 1,
  "ig.me": 1,
  "imgur.com": 1,
  "instagr.am": 1,
  "instagram.com": 1,
  "itch.io": 1,
  "jsdelivr.net": 1,
  "live.com": 1,
  "lumalabs.ai": 1,
  "mediawiki.org": 1,
  "mega.io": 1,
  "mega.nz": 1,
  "mit.edu": 1,
  "netflix.com": 1,
  "nicovideo.jp": 1,
  "nyaa.si": 1,
  "nyt.com": 1,
  "nytimes.com": 1,
  "ok.ru": 1,
  "okx.com": 1,
  "patreon.com": 1,
  "patreonusercontent.com": 1,
  "pinimg.com": 1,
  "pinterest.com": 1,
  "pixeldrain.com": 1,
  "pixiv.net": 1,
  "pornhub.com": 1,
  "prismic.io": 1,
  "proton.me": 1,
  "pximg.net": 1,
  "quora.com": 1,
  "redd.it": 1,
  "reddit.com": 1,
  "redditmedia.com": 1,
  "redditstatic.com": 1,
  "rfi.fr": 1,
  "rumble.com": 1,
  "rutube.ru": 1,
  "spotify.com": 1,
  "startpage.com": 1,
  "steamcommunity.com": 1,
  "steampowered.com": 1,
  "t.me": 1,
  "telegram.me": 1,
  "telegram.org": 1,
  "telesco.pe": 1,
  "tg.dev": 1,
  "thetvdb.com": 1,
  "tumblr.com": 1,
  "twitch.tv": 1,
  "v2ex.com": 1,
  "vercel.app": 1,
  "vimeo.com": 1,
  "vrchat.com": 1,
  "w.wiki": 1,
  "whatsapp.com": 1,
  "whatsapp.net": 1,
  "wikibooks.org": 1,
  "wikidata.org": 1,
  "wikifunctions.org": 1,
  "wikimedia.org": 1,
  "wikinews.org": 1,
  "wikipedia.org": 1,
  "wikiquote.org": 1,
  "wikisource.org": 1,
  "wikiversity.org": 1,
  "wikivoyage.org": 1,
  "wiktionary.org": 1,
  "xhamster.com": 1,
  "xhamster42.desi": 1,
  "xnxx.com": 1,
  "xvideos.com": 1,
  "yahoo.com": 1,
  "yimg.com": 1,
  "youtu.be": 1,
  "youtube-nocookie.com": 1,
  "youtube.com": 1,
  "ytimg.com": 1,
  "z-lib.help": 1,
  "z-library.sk": 1,
};

var shexps = {
  "*://*.bbc.co.uk/*": 1,
  "*://*.bbci.co.uk/*": 1,
  "*://*.japantimes.co.jp/*": 1,
  "*://search.yahoo.co.jp/*": 1,
  "*://*.cna.com.tw/*": 1,
  "*://media.discordapp.net/*": 1,
  "*://i.pximg.net/*": 1,
  "*://*.pixivsketch.net/*": 1,
  "*://*.githubassets.com/*": 1,
  "*://*.githubusercontent.com/*": 1,
  "*://*.redditstatic.com/*": 1
};

var proxy = "PROXY {{host}}:{{port}};";
var direct = 'DIRECT;';
var hasOwnProperty = Object.hasOwnProperty;

function shExpMatchs(str, shexps) {
    for (shexp in shexps) {
        if (shExpMatch(str, shexp)) return true;
    }
    return false;
}

function FindProxyForURL(url, host) {
    var suffix;
    var pos = host.lastIndexOf('.');
    pos = host.lastIndexOf('.', pos - 1);
    while(1) {
        if (pos <= 0) {
            if (hasOwnProperty.call(domains, host)) return proxy;
            if (shExpMatchs(url, shexps)) return proxy;
            return direct;
        }
        suffix = host.substring(pos + 1);
        if (hasOwnProperty.call(domains, suffix)) return proxy;
        pos = host.lastIndexOf('.', pos - 1);
    }
}
`
