# 证书信任与客户端配置

根证书路径：

- Linux / macOS：`~/.config/snirect/certs/root.crt`
- Windows：`%APPDATA%\snirect\certs\root.crt`

`snirect cert install` 会将根 CA 安装到系统信任库。浏览器（Chrome / Edge / Safari）、curl、git、Go 程序默认读取系统证书库，安装后即可生效。

以下工具或客户端使用独立证书库，需单独处理。

## Steam

Steam 内置浏览器（`steamwebhelper`，基于 Chromium）有时不读取系统证书库（尤其在 Linux 下），访问商店或社区可能报错 `Invalid SSL Certificate`（错误码 `-130` / `-202`）。可在 Steam 游戏内浏览器中直接导入根证书：

1. 确保 `Steam` -> `设置` -> `游戏中` 已开启**在游戏中启用 Steam 叠加界面**。
2. 启动任意一款游戏，按 `Shift + Tab` 呼出叠加界面，点击底部的**网页浏览器**。
3. 在浏览器顶部地址栏输入并回车：
   ```text
   chrome://settings/certificates
   ```
   （若未跳转可试 `chrome://certificate-manager`）
4. 切到**授权机构**（Authorities）标签页，点击**导入**（Import）。
5. 文件类型选**所有文件**，选择 `~/.config/snirect/certs/root.crt`（Windows 为 `%APPDATA%\snirect\certs\root.crt`；Linux 文件选择器按 `Ctrl + H` 显示隐藏目录）。
6. 勾选**信任此证书以标识网站**，确定后彻底退出并重启 Steam。

## Firefox / Zen / LibreWolf

Firefox 系浏览器使用独立的 NSS 数据库（`cert9.db`）。先安装 `certutil`（Debian/Ubuntu 装 `libnss3-tools`，Arch/Fedora 装 `nss`，macOS `brew install nss`），关闭浏览器后执行：

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

`NODE_EXTRA_CA_CERTS` 与 `keytool` 为**追加**，直接填 `root.crt` 路径即可。

`PIP_CERT`、`REQUESTS_CA_BUNDLE`、conda `ssl_verify`、`SSL_CERT_FILE` 为**替换**：如果只指向 `root.crt`，未走 MITM 的普通 HTTPS 站点会校验失败。这里需要传入「系统根证书包 + `root.crt`」拼接后的文件：

- Linux：执行 `snirect cert install` 后，`/etc/ssl/certs/ca-certificates.crt` 已包含 snirect CA，直接指向该文件即可。
- macOS：将 `root.crt` 追加到 `/etc/ssl/cert.pem` 副本后指向它（Homebrew 的 curl / git 若报证书错误也同理）。
- Windows：用 `python -c "import certifi; print(certifi.where())"` 找到 `cacert.pem`，把 `root.crt` 内容追加进去。

## Android 用户证书限制

Android 7.0（targetSdk ≥ 24）起，应用默认只信任系统证书库，手动导入的用户证书对大多数 App 不生效。

- **证书固定（Pinning）**：遇到内置证书固定的应用，代理检测到握手拒绝后会自动将其加入排除列表（可在应用内「自动排除」页管理）。
- **要在普通 App 中解密流量**：
  - Root 设备：通过 Magisk 模块将用户证书移入系统证书区。
  - Xposed：使用 LSPosed + TrustMeAlready 等模块绕过证书校验。
  - 免 Root：用 LSPatch / NPatch 对目标 APK 重新打包注入。
