# 证书信任与客户端配置

根证书路径（可使用 `snirect cert status` 查看）：

- **Linux**：`~/.config/snirect/certs/root.crt`
- **macOS**：`~/Library/Application Support/snirect/certs/root.crt`
- **Windows**：`%APPDATA%\snirect\certs\root.crt`

执行 `snirect cert install` 会在缺少根 CA 时自动生成并安装到系统信任库（Linux 下若系统安装了 `certutil`，还会自动写入 Chrome/Chromium 的 `~/.pki/nssdb` 以及 Firefox NSS）。浏览器（Chrome / Edge / Safari）、`curl`、`git`、Go 程序默认信任系统证书库，安装后即可生效。

对于使用证书固定（Certificate Pinning）的客户端（如网银、部分桌面端软件），它们不会信任用户安装的根 CA，应通过桌面端 `pac` 文件或 Android 排除列表让其直连。以下使用独立证书库的工具需单独配置。

## Steam

Steam 内置浏览器（`steamwebhelper`）有时不读取系统证书库（尤其在 Linux 下），访问商店或社区可能报错 `Invalid SSL Certificate`（`-130` / `-202`）。可通过游戏内浏览器导入根证书：

1. 在 `Steam` → `设置` → `游戏中` 开启**在游戏中启用 Steam 叠加界面**。
2. 启动任意游戏，按 `Shift + Tab` 打开叠加界面，点击底部的**网页浏览器**。
3. 地址栏输入 `chrome://settings/certificates` 回车（若无效可试 `chrome://certificate-manager`）。
4. 切到**授权机构**（Authorities）标签页，点击**导入**（Import）。
5. 文件类型选**所有文件**，选中上述路径下的 `root.crt`（Linux 文件选择器可按 `Ctrl + H` 显示隐藏目录）。
6. 勾选**信任此证书以标识网站**，确定后彻底退出并重启 Steam。

## Firefox / Zen / Waterfox / LibreWolf / Floorp

Firefox 系浏览器使用独立的 NSS 数据库（`cert9.db`）。先安装 `certutil`（Debian/Ubuntu：`libnss3-tools`；Arch/Fedora：`nss`；macOS：`brew install nss`），关闭浏览器后执行：

```bash
snirect cert install --firefox
```

## 开发工具与语言运行时

| 工具 | 默认证书来源 | 配置方式 |
| :--- | :--- | :--- |
| uv | 内置 Mozilla 根集 | `UV_SYSTEM_CERTS=1`（或 `uv --system-certs`） |
| Node.js | 内置 Mozilla 根集 | `NODE_EXTRA_CA_CERTS=<root.crt>`；v22.15+ / v24+ 可设 `NODE_USE_SYSTEM_CA=1` |
| conda | 环境内 certifi | `conda config --set ssl_verify <拼合包>` 或 `REQUESTS_CA_BUNDLE=<拼合包>` |
| pip / requests | certifi（pip ≥ 24.2 且 Python ≥ 3.10 默认读系统库） | `PIP_CERT=<拼合包>` / `REQUESTS_CA_BUNDLE=<拼合包>` |
| JVM（Java / Gradle） | 自带 `cacerts` | `keytool -importcert -cacerts -alias snirect -file <root.crt>`（默认口令 `changeit`） |

- **追加型**（`NODE_EXTRA_CA_CERTS`、`keytool`）：直接传入 `root.crt`。
- **替换型**（`PIP_CERT`、`REQUESTS_CA_BUNDLE`、conda `ssl_verify`、`SSL_CERT_FILE`）：若只填 `root.crt`，未走 MITM 的普通 HTTPS 站点会校验失败，需传入「系统根证书包 + `root.crt`」拼接后的文件：
  - **Linux**：执行 `snirect cert install` 后，`/etc/ssl/certs/ca-certificates.crt` 已包含 Snirect CA，直接指向该文件即可。
  - **macOS**：将 `root.crt` 追加到 `/etc/ssl/cert.pem` 副本后指向该副本（Homebrew 的 `curl`/`git` 若报错同理）。
  - **Windows**：用 `python -c "import certifi; print(certifi.where())"` 找到 `cacert.pem`，将 `root.crt` 内容追加到副本或原文件。

## Android 用户证书限制

Android 7.0（`targetSdk` ≥ 24）起，应用默认只信任系统证书库，手动导入的用户证书对多数 App 不生效。

- **自动排除**：代理检测到客户端因证书固定拒绝握手后，会自动将该应用加入排除列表（可在应用内「自动排除」页管理）。
- **若需对普通 App 解密流量**：
  - Root 设备：通过 Magisk 模块将用户证书移入系统证书区。
  - Xposed：使用 LSPosed + TrustMeAlready 等模块绕过证书校验。
  - 免 Root：用 LSPatch / NPatch 对目标 APK 重新打包注入。
