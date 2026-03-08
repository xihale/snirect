# Clash / dae 配合实现 SNI 专属代理分流

Snirect 本质上是一个提供特定解密与 SNI 伪装能力的 **HTTP Proxy**。你可以通过主流代理工具（如 Clash 或 dae）的路由规则，将特定域名的流量**劫持**并转发给 Snirect 进行处理。

核心逻辑非常简单：**利用规则优先级的自上而下特性，将涵盖特定 SNI 域名的规则插在最前排，并指向由 Snirect 承接的 HTTP Proxy 出口（这取决于你对节点或策略组的命名，本例中统称 `sni` 分组）。**

以下是 Clash 和 dae 各自独立的配置示例。

## 代理出口定义（前提准备）

无论你主要使用哪个工具处理流量，都需要在配置中设立一个专门指向 Snirect 的出站 (Outbound) 或策略组，便于后续将特定域名路由至此。

**Clash 策略组示例 (`proxy-groups`)：**
```yaml
  - name: "SNI分组"
    type: select
    proxies:
      - "sni"         # 指向 Snirect 具体 HTTP Proxy 节点配置的名称
      - "自动选择"    # 其他备选策略（由用户自己根据现有节点配置）
      - "节点选择"
      - "DIRECT"
```

dae 的分组需要你自己弄一个 outbound

## dae 路由配置 (Routing)

在 dae 中，规则匹配严格自上而下。为确保拦截生效，需要将这一大段目标域名列表作为 `domain(...)` 规则塞进 `routing { ... }` 的**最顶部**：

```dae
routing {
    domain(
        amazon.co.jp, android.com, apkmirror.com, archive.org, archiveofourown.org,
        bbc.co.uk, bbc.com, bbci.co.uk, behance.net, bilibili.tv, blogger.com,
        cdn-telegram.org, character.ai, claude.ai, dailymotion.com, discord.com,
        discord.gg, discordapp.com, discordapp.net, disneyplus.com, dropbox.com,
        duckduckgo.com, e-hentai.org, ecosia.org, edge.bamgrid.com, ehgt.org,
        ehtracker.org, ehwiki.org, etsy.com, exhentai.org, eyny.com, f-droid.org,
        facebook.com, fanbox.cc, fbcdn.net, flickr.com, gamer.com.tw, gelbooru.com,
        ggpht.com, github.com, githubusercontent.com, google.com, google.com.hk,
        googleusercontent.com, gravatar.com, greasyfork.org,
        gstatic.com, hentaiverse.org, hub.docker.com, huggingface.co, ig.me,
        images.prismic.io, imgur.com, instagr.am, instagram.com, itch.io,
        jsdelivr.net, lumalabs.ai, media.tumblr.com, mediawiki.org, mega.co.nz,
        mega.io, mega.nz, netflix.com, nyaa.si, nyt.com, nytimes.com, ok.ru,
        okx.com, onedrive.live.com, patreon.com, patreonusercontent.com, pinimg.com,
        pinterest.com, pixeldrain.com, pixiv.net, pornhub.com, proton.me, pximg.net,
        quora.com, redd.it, reddit.com, redditmedia.com, redditstatic.com, rfi.fr,
        rumble.com, rutube.ru, scratch.mit.edu, startpage.com, steamcommunity.com,
        t.me, telegram.me, telegram.org, telesco.pe, tg.dev, thetvdb.com,
        tumblr.com, twitch.tv, v2ex.com, vercel.app, vimeo.com, vrchat.com, w.wiki,
        whatsapp.com, whatsapp.net, wikibooks.org, wikidata.org, wikifunctions.org,
        wikimedia.org, wikinews.org, wikipedia.org, wikiquote.org, wikisource.org,
        wikiversity.org, wikivoyage.org, wiktionary.org, xhamster.com,
        xhamster42.desi, xnxx.com, xvideos.com, yahoo.com, yimg.com, youtu.be,
        youtube-nocookie.com, z-lib.help, z-library.sk
    ) -> sni
}
```

## Clash 路由配置 (Rules)

由于 Clash 不支持单行列表格式，需要拆分成多条 `DOMAIN-SUFFIX` 规则。
同样，**切记必须要将这组规则插到 `rules:` 列表下的最顶部**，以防被后方的全量接管或其他 GEOIP 规则覆盖截胡：

```yaml
rules:
  - DOMAIN-SUFFIX,amazon.co.jp,SNI分组
  - DOMAIN-SUFFIX,android.com,SNI分组
  - DOMAIN-SUFFIX,apkmirror.com,SNI分组
  - DOMAIN-SUFFIX,archive.org,SNI分组
  - DOMAIN-SUFFIX,archiveofourown.org,SNI分组
  - DOMAIN-SUFFIX,bbc.co.uk,SNI分组
  - DOMAIN-SUFFIX,bbc.com,SNI分组
  - DOMAIN-SUFFIX,bbci.co.uk,SNI分组
  - DOMAIN-SUFFIX,behance.net,SNI分组
  - DOMAIN-SUFFIX,bilibili.tv,SNI分组
  - DOMAIN-SUFFIX,blogger.com,SNI分组
  - DOMAIN-SUFFIX,cdn-telegram.org,SNI分组
  - DOMAIN-SUFFIX,character.ai,SNI分组
  - DOMAIN-SUFFIX,claude.ai,SNI分组
  - DOMAIN-SUFFIX,dailymotion.com,SNI分组
  - DOMAIN-SUFFIX,discord.com,SNI分组
  - DOMAIN-SUFFIX,discord.gg,SNI分组
  - DOMAIN-SUFFIX,discordapp.com,SNI分组
  - DOMAIN-SUFFIX,discordapp.net,SNI分组
  - DOMAIN-SUFFIX,disneyplus.com,SNI分组
  - DOMAIN-SUFFIX,dropbox.com,SNI分组
  - DOMAIN-SUFFIX,duckduckgo.com,SNI分组
  - DOMAIN-SUFFIX,e-hentai.org,SNI分组
  - DOMAIN-SUFFIX,ecosia.org,SNI分组
  - DOMAIN-SUFFIX,edge.bamgrid.com,SNI分组
  - DOMAIN-SUFFIX,ehgt.org,SNI分组
  - DOMAIN-SUFFIX,ehtracker.org,SNI分组
  - DOMAIN-SUFFIX,ehwiki.org,SNI分组
  - DOMAIN-SUFFIX,etsy.com,SNI分组
  - DOMAIN-SUFFIX,exhentai.org,SNI分组
  - DOMAIN-SUFFIX,eyny.com,SNI分组
  - DOMAIN-SUFFIX,f-droid.org,SNI分组
  - DOMAIN-SUFFIX,facebook.com,SNI分组
  - DOMAIN-SUFFIX,fanbox.cc,SNI分组
  - DOMAIN-SUFFIX,fbcdn.net,SNI分组
  - DOMAIN-SUFFIX,flickr.com,SNI分组
  - DOMAIN-SUFFIX,gamer.com.tw,SNI分组
  - DOMAIN-SUFFIX,gelbooru.com,SNI分组
  - DOMAIN-SUFFIX,ggpht.com,SNI分组
  - DOMAIN-SUFFIX,github.com,SNI分组
  - DOMAIN-SUFFIX,githubusercontent.com,SNI分组
  - DOMAIN-SUFFIX,google.com,SNI分组
  - DOMAIN-SUFFIX,google.com.hk,SNI分组
  - DOMAIN-SUFFIX,googleusercontent.com,SNI分组
  - DOMAIN-SUFFIX,gravatar.com,SNI分组
  - DOMAIN-SUFFIX,greasyfork.org,SNI分组
  - DOMAIN-SUFFIX,gstatic.com,SNI分组
  - DOMAIN-SUFFIX,hentaiverse.org,SNI分组
  - DOMAIN-SUFFIX,hub.docker.com,SNI分组
  - DOMAIN-SUFFIX,huggingface.co,SNI分组
  - DOMAIN-SUFFIX,ig.me,SNI分组
  - DOMAIN-SUFFIX,images.prismic.io,SNI分组
  - DOMAIN-SUFFIX,imgur.com,SNI分组
  - DOMAIN-SUFFIX,instagr.am,SNI分组
  - DOMAIN-SUFFIX,instagram.com,SNI分组
  - DOMAIN-SUFFIX,itch.io,SNI分组
  - DOMAIN-SUFFIX,jsdelivr.net,SNI分组
  - DOMAIN-SUFFIX,lumalabs.ai,SNI分组
  - DOMAIN-SUFFIX,media.tumblr.com,SNI分组
  - DOMAIN-SUFFIX,mediawiki.org,SNI分组
  - DOMAIN-SUFFIX,mega.co.nz,SNI分组
  - DOMAIN-SUFFIX,mega.io,SNI分组
  - DOMAIN-SUFFIX,mega.nz,SNI分组
  - DOMAIN-SUFFIX,netflix.com,SNI分组
  - DOMAIN-SUFFIX,nyaa.si,SNI分组
  - DOMAIN-SUFFIX,nyt.com,SNI分组
  - DOMAIN-SUFFIX,nytimes.com,SNI分组
  - DOMAIN-SUFFIX,ok.ru,SNI分组
  - DOMAIN-SUFFIX,okx.com,SNI分组
  - DOMAIN-SUFFIX,onedrive.live.com,SNI分组
  - DOMAIN-SUFFIX,patreon.com,SNI分组
  - DOMAIN-SUFFIX,patreonusercontent.com,SNI分组
  - DOMAIN-SUFFIX,pinimg.com,SNI分组
  - DOMAIN-SUFFIX,pinterest.com,SNI分组
  - DOMAIN-SUFFIX,pixeldrain.com,SNI分组
  - DOMAIN-SUFFIX,pixiv.net,SNI分组
  - DOMAIN-SUFFIX,pornhub.com,SNI分组
  - DOMAIN-SUFFIX,proton.me,SNI分组
  - DOMAIN-SUFFIX,pximg.net,SNI分组
  - DOMAIN-SUFFIX,quora.com,SNI分组
  - DOMAIN-SUFFIX,redd.it,SNI分组
  - DOMAIN-SUFFIX,reddit.com,SNI分组
  - DOMAIN-SUFFIX,redditmedia.com,SNI分组
  - DOMAIN-SUFFIX,redditstatic.com,SNI分组
  - DOMAIN-SUFFIX,rfi.fr,SNI分组
  - DOMAIN-SUFFIX,rumble.com,SNI分组
  - DOMAIN-SUFFIX,rutube.ru,SNI分组
  - DOMAIN-SUFFIX,scratch.mit.edu,SNI分组
  - DOMAIN-SUFFIX,startpage.com,SNI分组
  - DOMAIN-SUFFIX,steamcommunity.com,SNI分组
  - DOMAIN-SUFFIX,t.me,SNI分组
  - DOMAIN-SUFFIX,telegram.me,SNI分组
  - DOMAIN-SUFFIX,telegram.org,SNI分组
  - DOMAIN-SUFFIX,telesco.pe,SNI分组
  - DOMAIN-SUFFIX,tg.dev,SNI分组
  - DOMAIN-SUFFIX,thetvdb.com,SNI分组
  - DOMAIN-SUFFIX,tumblr.com,SNI分组
  - DOMAIN-SUFFIX,twitch.tv,SNI分组
  - DOMAIN-SUFFIX,v2ex.com,SNI分组
  - DOMAIN-SUFFIX,vercel.app,SNI分组
  - DOMAIN-SUFFIX,vimeo.com,SNI分组
  - DOMAIN-SUFFIX,vrchat.com,SNI分组
  - DOMAIN-SUFFIX,w.wiki,SNI分组
  - DOMAIN-SUFFIX,whatsapp.com,SNI分组
  - DOMAIN-SUFFIX,whatsapp.net,SNI分组
  - DOMAIN-SUFFIX,wikibooks.org,SNI分组
  - DOMAIN-SUFFIX,wikidata.org,SNI分组
  - DOMAIN-SUFFIX,wikifunctions.org,SNI分组
  - DOMAIN-SUFFIX,wikimedia.org,SNI分组
  - DOMAIN-SUFFIX,wikinews.org,SNI分组
  - DOMAIN-SUFFIX,wikipedia.org,SNI分组
  - DOMAIN-SUFFIX,wikiquote.org,SNI分组
  - DOMAIN-SUFFIX,wikisource.org,SNI分组
  - DOMAIN-SUFFIX,wikiversity.org,SNI分组
  - DOMAIN-SUFFIX,wikivoyage.org,SNI分组
  - DOMAIN-SUFFIX,wiktionary.org,SNI分组
  - DOMAIN-SUFFIX,xhamster.com,SNI分组
  - DOMAIN-SUFFIX,xhamster42.desi,SNI分组
  - DOMAIN-SUFFIX,xnxx.com,SNI分组
  - DOMAIN-SUFFIX,xvideos.com,SNI分组
  - DOMAIN-SUFFIX,yahoo.com,SNI分组
  - DOMAIN-SUFFIX,yimg.com,SNI分组
  - DOMAIN-SUFFIX,youtu.be,SNI分组
  - DOMAIN-SUFFIX,youtube-nocookie.com,SNI分组
  - DOMAIN-SUFFIX,z-lib.help,SNI分组
  - DOMAIN-SUFFIX,z-library.sk,SNI分组
```

