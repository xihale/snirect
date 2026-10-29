// Static built-in routing rules. Manually maintained; loaded verbatim by
// LoadRules() into the running binary. No runtime fetching or codegen.
//
// AlterHostname/Hosts are synced from Cealing-Host releases (currently
// 1.1.4.55) with these adaptations: browser-only (#) and disabled (^) rules
// skipped, $ prefixes stripped, IPv6-only hosts dropped (alter_hostname
// kept), empty hosts (127.0.0.1) skipped, and t.me/telegram.me kept as the
// precise "*.t.me"+"t.me" split instead of upstream's "*t.me" suffix form
// (which would hijack e.g. about.me). CertVerify/IgnoreExpiry are
// snirect-specific, validated against live front certificates. The GitHub
// Hosts/AlterHostname pins for upload/uploads/collector/alive and the
// *github.io Pages front are snirect-specific too (validated against live
// edges; see the inline comments).
//
// Front IPs are snirect-specific where upstream's are dead. 1.1.4.55 moved
// Google off the retired 183.56.143.147 to the rotating hostname
// google.spacetime.cc.cd (→ 64.233.168.107 at sync time); snirect keeps its
// own verified GFE, and still ships Telegram's front IP rather than a host
// name because the Telegram DC takes every *.telegram.org Host on empty SNI.
// 1.1.4.55 also updated the dailymotion/upld/flickr pins and removed the
// pixiv/fanbox/ehgt/greasyfork families outright — those fronts no longer
// answer their Host headers, so the removals are mirrored here. Front IPs are
// re-probed when a host fails; the rest of the rule set stays verbatim from
// upstream.
//
// Two exceptions go past upstream's pin:
//   - graphql.api.dailymotion.com is CloudFront-only, so it keeps the Host but
//     rewrites SNI to the distribution name (CloudFront requires a matching
//     SNI; the real one is GFW-reset and an empty one fails the handshake).
//   - Reddit is dropped entirely: Fastly selects the service by SNI, so the
//     empty-SNI front answers 403 Blocked, and the real SNI is reset on every
//     peer. Leaving the rules in would only force reddit into snirect and away
//     from whatever else the rule export feeds.

package rules

var builtinRules = &Rules{
	AlterHostname: map[string]string{

		"*.buy.yahoo.com":         "buy.yahoo.com",
		"*.ggpht.com":             "g.cn",
		"*.googlevideo.com":       "",
		"*.ig.me":                 "",
		"*.media.tumblr.com":      "",
		"*.t.me":                  "",
		"*.w.wiki":                "",
		"*.yahoo.com":             "www.yahoo.com",
		"*amazon.co.jp":           "amazon.com",
		"*android.com":            "g.cn",
		"*api.mega.co.nz":         "",
		"*apkmirror.com":          "",
		"*archive.org":            "",
		"*archiveofourown.org":    "",
		"*audiomack.com":          "",
		"*bbc.co.uk":              "",
		"*bbc.com":                "",
		"*bbci.co.uk":             "",
		"*behance.net":            "behance.net",
		"*bilibili.tv":            "bilibili.tv",
		"*blogger.com":            "g.cn",
		"*character.ai":           "",
		"*claude.ai":              "claude.ai",
		"*dailymotion.com":        "",
		"*discord.com":            "",
		"*discord.gg":             "",
		"*discordapp.com":         "",
		"*discordapp.net":         "",
		"*disneyplus.com":         "disneyplus.com",
		"*dropbox.com":            "",
		"*duckduckgo.com":         "",
		"*e-hentai.org":           "",
		"*ehtracker.org":          "",
		"*ehwiki.org":             "",
		"*etsy.com":               "",
		"*exhentai.org":           "",
		"*eyny.com":               "",
		"*facebook.com":           "",
		"*fbcdn.net":              "",
		"*flickr.com":             "",
		"*gamer.com.tw":           "",
		"*gelbooru.com":           "",
		"*github.com":             "",
		"*github.io":              "",
		"*githubusercontent.com":  "",
		"*google.com":             "g.cn",
		"*google.com.hk":          "g.cn",
		"*gravatar.com":           "",
		"*gstatic.com":            "g.cn",
		"*hentaiverse.org":        "",
		"*huggingface.co":         "huggingface.cn",
		"*imgur.com":              "",
		"*instagr.am":             "",
		"*instagram.com":          "",
		"*lumalabs.ai":            "vercel.com",
		"*mediawiki.org":          "",
		"*mega.io":                "",
		"*mega.nz":                "",
		"*netflix.com":            "netflix.com",
		"*nicovideo.jp":           "",
		"*nyaa.si":                "ddos-guard.net",
		"*nyt.com":                "",
		"*nytimes.com":            "",
		"*ok.ru":                  "",
		"*okx.com":                "",
		"*onedrive.live.com":      "",
		"*patreon.com":            "",
		"*patreonusercontent.com": "",
		"*pinimg.com":             "",
		"*pinterest.com":          "",
		"*pixeldrain.com":         "pixeldra.in",
		"*pornhub.com":            "",
		"*proton.me":              "",
		"*pximg.net":              "",
		"*quora.com":              "fs.quoracdn.net",
		// Reddit has no usable front: Fastly selects the service by SNI, so the
		// empty-SNI path Cealing-Host pins (146.75.33.140) answers "403 Blocked"
		// with the CN=default.ssl.fastly.net leaf, while SNI www.reddit.com /
		// reddit.com / i.redd.it is reset by the GFW on any peer — 64.233.160.142
		// (GFE), 1.1.1.1 and 104.16.89.20 all drop the handshake too, probed
		// 2026-10-29. The rules are dropped rather than left failing so a downstream
		// proxy can handle reddit; see the removed Hosts/CertVerify entries.
		"*rfi.fr":                  "",
		"*rumble.com":              "",
		"*rutube.ru":               "",
		"*scratch.mit.edu":         "",
		"*spotify.com":             "spotify.com",
		"*startpage.com":           "",
		"*steamcommunity.com":      "",
		"*sukebei.nyaa.si":         "",
		"*telegram.org":            "",
		"*telesco.pe":              "",
		"*tg.dev":                  "",
		"*thetvdb.com":             "",
		"*tumblr.com":              "",
		"*twitch.tv":               "",
		"*v2ex.com":                "",
		"*vercel.app":              "vercel.com",
		"*vimeo.com":               "",
		"*whatsapp.com":            "",
		"*whatsapp.net":            "",
		"*wikibooks.org":           "",
		"*wikidata.org":            "",
		"*wikifunctions.org":       "",
		"*wikimedia.org":           "",
		"*wikinews.org":            "",
		"*wikipedia.org":           "",
		"*wikiquote.org":           "",
		"*wikisource.org":          "",
		"*wikiversity.org":         "",
		"*wikivoyage.org":          "",
		"*wiktionary.org":          "",
		"*xhamster.com":            "zh.xhamster.com",
		"*xhamster42.desi":         "zh.xhamster42.desi",
		"*xnxx.com":                "",
		"*xvideos.com":             "",
		"*xvideos.red":             "",
		"*youtu.be":                "g.cn",
		"*youtube-nocookie.com":    "g.cn",
		"*youtube.com":             "g.cn",
		"*z-lib.help":              "",
		"*z-library.sk":            "",
		"a5.behance.net":           "a5.behance.net",
		"account-api.proton.me":    "",
		"account.nicovideo.jp":     "",
		"account.proton.me":        "",
		"alive.github.com":         "",
		"collector.github.com":     "",
		"api.audiomack.com":        "",
		"api.github.com":           "",
		"ask.vrchat.com":           "",
		"assets.audiomack.com":     "",
		"assets.twitch.tv":         "",
		"auth.docker.io":           "",
		"blog.docker.io":           "",
		"business.whatsapp.com":    "",
		"calendar.proton.me":       "",
		"cdn-registry-1.docker.io": "",
		"cdn.jsdelivr.net":         "",
		"cdn1.cdn-telegram.org":    "",
		"cdn4.cdn-telegram.org":    "",
		"cdn5.cdn-telegram.org":    "",
		"codeload.github.com":      "",
		"community.github.com":     "",
		"consent.yahoo.com":        "consent.yahoo.com",

		"docker.io":             "",
		"docs.docker.io":        "",
		"docs-editor.proton.me": "",
		"docs-rts.proton.me":    "",
		"docs.github.com":       "",
		"docs.proton.me":        "",
		"drive.proton.me":       "",
		// 1.1.4.55: upstream splits these two Hugging Face subdomains off the
		// *huggingface.co → huggingface.cn front. discuss goes to the Discourse
		// host with SNI stripped (see Hosts/CertVerify).
		"discuss.huggingface.co":          "",
		"embed.nicovideo.jp":              "",
		"external-content.duckduckgo.com": "",
		"f-droid.org":                     "",
		"fdroid.org":                      "",
		"forum.f-droid.org":               "",
		"fra-storage.proton.me":           "",
		"gemini.google.com":               "",
		"get.docker.io":                   "",
		"gql.twitch.tv":                   "",
		// CloudFront picks the distribution by Host header but requires an SNI
		// that matches one of its certificates. graphql.api.dailymotion.com is
		// a CloudFront-only host, so stripping SNI fails (alert 40) and sending
		// its real SNI is reset by the GFW. d2is9o3959y6go.cloudfront.net is the
		// distribution's own name: the handshake completes under the
		// CN=*.cloudfront.net leaf while Host still routes to dailymotion.
		// Probed 2026-10-29 (400 from the graphql app on GET /, not a CDN 403).
		"graphql.api.dailymotion.com":          "d2is9o3959y6go.cloudfront.net",
		"guce.yahoo.com":                       "guce.yahoo.com",
		"hello.vrchat.com":                     "",
		"hub.docker.com":                       "",
		"i.ytimg.com":                          "g.cn",
		"identity.flickr.com":                  "",
		"ig.me":                                "",
		"images.prismic.io":                    "imgix.net",
		"index.docker.io":                      "",
		"irc-ws.chat.twitch.tv":                "",
		"live.nicovideo.jp":                    "",
		"live2.nicovideo.jp":                   "",
		"lumo.proton.me":                       "",
		"m.twitch.tv":                          "",
		"mail.proton.me":                       "",
		"meet.proton.me":                       "",
		"music.audiomack.com":                  "",
		"notary.docker.io":                     "",
		"nvapi.nicovideo.jp":                   "",
		"objects-origin.githubusercontent.com": "",
		"ok.ru":                                "",
		"open.spotify.com":                     "",
		"panels.twitch.tv":                     "",
		"pass.proton.me":                       "",
		"passport.twitch.tv":                   "",
		"polymarket.com":                       "vercel.com",
		"production.cloudflare.docker.com":     "",
		"registry-1.docker.io":                 "",
		"registry.docker.io":                   "",
		"registry.hub.docker.com":              "",
		"resources.github.com":                 "",
		"s.yimg.com":                           "www.yahoo.com",
		"services.github.com":                  "",
		"sheets.proton.me":                     "",
		"sp.nicovideo.jp":                      "",
		"sso.proton.me":                        "",
		"status.docker.io":                     "",
		// 1.1.4.55: the status page moved to Better Uptime, whose host serves
		// Host: status.huggingface.co only under the singpass SNI.
		"status.huggingface.co":  "status.singpass.gov.sg",
		"store.steampowered.com": "",
		"support.github.com":     "",
		"t.me":                   "",
		"telegram.me":            "",
		"upld.e-hentai.org":      "",
		"upload.github.com":      "",
		"uploads.github.com":     "",
		"upload.wikimedia.org":   "",
		"w.wiki":                 "",
		"wallet.proton.me":       "",
		"wktk.nicovideo.jp":      "",
		"www.docker.io":          "",
		"www.ecosia.org":         "www.ecosia.org",
		"www.f-droid.org":        "",
		"www.ok.ru":              "",
		"zrh-storage.proton.me":  "",
	},
	CertVerify: map[string]interface{}{
		"*.objects-origin.githubusercontent.com": []interface{}{"*.actions.githubusercontent.com", "actions.githubusercontent.com"},
		// Live leaf (SNI-stripped Proton hosts) presents *.proton.me / *.pr.tn —
		// the old protonmail.com allowlist rejected every proton.me subdomain.
		"*.proton.me":    []interface{}{"*.proton.me", "proton.me", "*.pr.tn", "pr.tn", "*.storage.proton.me", "*.pm.me", "*.protonmail.ch", "*.protonmail.com", "protonmail.com"},
		"*ggpht.com":     []interface{}{"*.google.cn", "google.cn"},
		"*google.com":    "g.cn",
		"*google.com.hk": "g.cn",
		"*gstatic.com":   "g.cn",
		"*youtube.com":   []interface{}{"*.google.cn", "google.cn"},
		"*ytimg.com":     []interface{}{"*.google.cn", "google.cn"},
		// android/blogger/youtu.be share the g.cn front with youtube — the old
		// healthdatanexus.ai allowlist is a dead front and rejects the real cert.
		"*android.com":   []interface{}{"*.google.cn", "google.cn", "g.cn", "*.g.cn"},
		"*audiomack.com": []interface{}{"*.cloudfront.net", "cloudfront.net"},
		"*bbc.co.uk":     []interface{}{"*.cdn.cyberarena.at", "cdn.cyberarena.at"},
		"*bbci.co.uk":    []interface{}{"*.cdn.cyberarena.at", "cdn.cyberarena.at"},
		// CDN hosts present their own *.cdn-telegram.org leaf (not *.telegram.org).
		"*cdn-telegram.org": []interface{}{"*.cdn-telegram.org", "cdn-telegram.org", "*.telegram.org", "telegram.org"},
		"*ig.me":            []interface{}{"*.instagram.com", "*.cdninstagram.com", "*.igsonar.com", "cdninstagram.com", "igsonar.com", "instagram.com"},
		"*instagr.am":       []interface{}{"*.instagram.com", "*.cdninstagram.com", "*.igsonar.com", "cdninstagram.com", "igsonar.com", "instagram.com"},
		"*mega.io":          []interface{}{"*.static.mega.co.nz", "static.mega.co.nz"},
		"*nicovideo.jp":     []interface{}{"*.cloudfront.net", "cloudfront.net"},
		"*ok.ru":            []interface{}{"*.ok.ru", "ok.ru"},
		"*t.me":             []interface{}{"*.telegram.org", "telegram.org"},
		// Live leaf at the Fastly IP is *.twitch.tv; assets.* still sits on
		// CloudFront — keep both so either front cert passes.
		"*twitch.tv":            []interface{}{"*.twitch.tv", "twitch.tv", "*.cloudfront.net", "cloudfront.net"},
		"*z-lib.help":           []interface{}{"*.go-to-library.sk", "go-to-library.sk"},
		"*z-library.sk":         []interface{}{"*.go-to-library.sk", "go-to-library.sk"},
		"ask.vrchat.com":        []interface{}{"*.discourse.org"},
		"hello.vrchat.com":      []interface{}{"*.squarespace.com", "squarespace.com"},
		"*blogger.com":          []interface{}{"*.google.cn", "google.cn", "g.cn", "*.g.cn"},
		"business.whatsapp.com": []interface{}{"*.facebook.com", "*.facebook.net", "*.fbcdn.net", "*.fbsbx.com", "*.m.facebook.com", "*.messenger.com", "*.xx.fbcdn.net", "*.xy.fbcdn.net", "*.xz.fbcdn.net", "facebook.com", "messenger.com"},
		"docs.github.com":       []interface{}{"*.githubassets.com", "githubassets.com"},
		// 1.1.4.55 Hugging Face splits: status is served by Better Uptime under
		// the singpass leaf; discuss is SNI-stripped at the Discourse host,
		// whose empty-SNI vhost answer depends on the resolved edge — live
		// probes saw both *.hosted-by-discourse.com and *.discourse.org
		// (2026-10-29), so both Discourse leaves are allowed.
		"status.huggingface.co":  []interface{}{"status.singpass.gov.sg"},
		"discuss.huggingface.co": []interface{}{"*.hosted-by-discourse.com", "hosted-by-discourse.com", "*.discourse.org", "discourse.org"},
		"gemini.google.com":      false,
		"gemini.gstatic.com":     "g.cn",
		// 1.1.4.55 moved identity.flickr.com to 34.192.66.226, whose empty-SNI
		// default vhost presents Flickr's own Amazon-issued leaf
		// (CN=flickr.com, SANs flickr.com / *.flickr.com / flic.kr) instead of
		// the old *.bridgify.com tenant cert.
		"identity.flickr.com": []interface{}{"flickr.com", "*.flickr.com", "flic.kr"},
		"ig.me":               []interface{}{"*.instagram.com", "*.cdninstagram.com", "*.igsonar.com", "cdninstagram.com", "igsonar.com", "instagram.com"},
		"instagr.am":          []interface{}{"*.instagram.com", "*.cdninstagram.com", "*.igsonar.com", "cdninstagram.com", "igsonar.com", "instagram.com"},
		"mega.io":             []interface{}{"*.static.mega.co.nz", "static.mega.co.nz"},
		"*scratch.mit.edu":    []interface{}{"d.sni-645-default.ssl.fastly.net"},
		"*sukebei.nyaa.si":    false,
		// These SNI-stripped fronts currently return empty-SAN / honeypot leaves
		// (CN=fallback.wgcz.net, please-go-away, bare IP). Disable verify so the
		// tunnel still opens; Hosts IPs may need a later refresh.
		"*okx.com":         false,
		"*spotify.com":     false,
		"open.spotify.com": false,
		"*xnxx.com":        false,
		"*xvideos.com":     false,
		"*xvideos.red":     false,
		// The CloudFront distribution name is only rewritten into the SNI; the
		// leaf it serves is Amazon's shared CN=*.cloudfront.net.
		"graphql.api.dailymotion.com": []interface{}{"*.cloudfront.net", "cloudfront.net"},
		// Cealing-Host 1.1.4.53 strips SNI on *.googlevideo.com, but Google's
		// empty-SNI default vhost answers "invalid2.invalid" (CN=No SNI
		// provided, probed 2026-10-28), so upstream's rule as-is would fail
		// verification on every video request. Disable verify to keep the
		// tunnel open, same class as the honeypot fronts above.
		"*.googlevideo.com":     false,
		"t.me":                  []interface{}{"*.telegram.org", "telegram.org"},
		"telegram.me":           []interface{}{"*.telegram.org", "telegram.org"},
		"*telesco.pe":           []interface{}{"*.telegram.org", "telegram.org"},
		"*tg.dev":               []interface{}{"*.telegram.org", "telegram.org"},
		"www.google.com":        "g.cn",
		"www.gstatic.com":       "g.cn",
		"*youtu.be":             []interface{}{"*.google.cn", "google.cn", "g.cn", "*.g.cn"},
		"*youtube-nocookie.com": []interface{}{"*.google.cn", "google.cn", "g.cn", "*.g.cn"},
	},
	Hosts: map[string]string{
		"*.ggpht.com":            "64.233.160.142",
		"*.ig.me":                "157.240.27.174",
		"*.media.tumblr.com":     "192.0.77.3",
		"*.t.me":                 "149.154.166.110",
		"*.w.wiki":               "185.15.59.224",
		"*amazon.co.jp":          "13.35.219.115",
		"*android.com":           "64.233.160.142",
		"*api.mega.co.nz":        "66.203.125.15",
		"*apkmirror.com":         "104.17.67.215",
		"*archive.org":           "207.241.237.2",
		"*archiveofourown.org":   "104.20.8.2",
		"*audiomack.com":         "3.167.200.113",
		"*bbc.co.uk":             "23.77.21.232",
		"*bbc.com":               "146.75.36.81",
		"*bbci.co.uk":            "23.77.21.232",
		"*blogger.com":           "64.233.160.142",
		"*character.ai":          "104.18.223.226",
		"*dailymotion.com":       "195.8.215.140",
		"*discord.com":           "162.159.136.232",
		"*discord.gg":            "162.159.130.234",
		"*discordapp.com":        "162.159.130.233",
		"*discordapp.net":        "162.159.130.232",
		"*dropbox.com":           "162.125.248.18",
		"*duckduckgo.com":        "20.43.161.105",
		"*e-hentai.org":          "172.66.132.196",
		"*ehtracker.org":         "5.79.104.115",
		"*ehwiki.org":            "178.162.151.58",
		"*etsy.com":              "151.101.193.224",
		"*exhentai.org":          "178.175.132.22",
		"*eyny.com":              "23.19.72.223",
		"*facebook.com":          "157.240.22.169",
		"*fbcdn.net":             "157.240.22.169",
		"*flickr.com":            "13.33.142.102",
		"*gamer.com.tw":          "104.16.223.104",
		"*gelbooru.com":          "108.181.143.72",
		"*github.com":            "20.27.177.113",
		"*github.io":             "185.199.108.153",
		"*githubusercontent.com": "185.199.108.133",
		// Upstream's 183.56.143.147 Google front is retired: 183.56.143.0/24 is
		// shared hosting now (443 refused on .147; the rest answer VMware /
		// Huawei / jointas certs — probed 2026-10-28 from both a CN VPS and an
		// unfiltered host). 1.1.4.55 replaced it with the rotating hostname
		// google.spacetime.cc.cd, which resolved to 64.233.168.107 and answers
		// the same g.cn SNI — so upstream's source is a GFE either way.
		// 64.233.160.142 (oi-in-f142.1e100.net) is the GFE snirect pins: it
		// answers SNI g.cn with the *.google.cn leaf (SANs include g.cn) and
		// serves the real content behind the original Host header: www.google.com
		// 302 / gstatic 404 (unpinned path) / youtube 200 / ytimg 200, stable
		// probes from a Tencent Cloud GZ VPS, re-checked 2026-10-29.
		"*google.com":             "64.233.160.142",
		"*google.com.hk":          "64.233.160.142",
		"*gravatar.com":           "192.0.80.240",
		"*gstatic.com":            "64.233.160.142",
		"*hentaiverse.org":        "178.162.151.56",
		"*huggingface.co":         "3.167.200.113",
		"*imgur.com":              "199.232.196.193",
		"*instagr.am":             "157.240.27.174",
		"*instagram.com":          "157.240.27.174",
		"*lumalabs.ai":            "76.76.21.21",
		"*mediawiki.org":          "185.15.59.224",
		"*mega.io":                "66.203.127.24",
		"*mega.nz":                "31.216.144.5",
		"*nicovideo.jp":           "3.167.200.113",
		"*nyaa.si":                "186.2.163.20",
		"*nyt.com":                "146.75.117.164",
		"*nytimes.com":            "146.75.117.164",
		"*ok.ru":                  "5.61.23.30",
		"*okx.com":                "8.212.101.92",
		"*onedrive.live.com":      "13.107.42.13",
		"*patreon.com":            "104.16.25.14",
		"*patreonusercontent.com": "104.18.70.106",
		"*pinimg.com":             "151.101.0.84",
		"*pinterest.com":          "151.101.0.84",
		"*pixeldrain.com":         "103.107.198.191",
		"*pornhub.com":            "66.254.114.40",
		"*proton.me":              "185.70.42.45",
		"*pximg.net":              "210.140.139.133",
		"*quora.com":              "162.159.152.17",
		"*rfi.fr":                 "118.214.247.61",
		"*rumble.com":             "205.220.231.24",
		"*rutube.ru":              "109.238.90.239",
		"*scratch.mit.edu":        "151.101.66.133",
		"*startpage.com":          "67.63.60.231",
		"*steamcommunity.com":     "184.50.187.66",
		"*sukebei.nyaa.si":        "198.251.89.38",
		// Upstream's 45.192.12.153 Telegram front is gone: it answers every SNI
		// with a Traefik default cert and 45.192.12.0/24 is shared hosting
		// (cdnjs/wikipedia/cloudflare vhosts — probed 2026-10-28). Telegram's
		// own DC IP serves all *.telegram.org hosts on empty SNI with the real
		// GoDaddy leaf (*.telegram.org, valid to 2027-03): api 302, getMe 404
		// JSON, web 200, t.me/telegram.me/tg.dev 302 — probed live from a
		// Tencent Cloud GZ VPS.
		"*telegram.org":         "149.154.166.110",
		"*telesco.pe":           "149.154.166.110",
		"*tg.dev":               "149.154.166.110",
		"*thetvdb.com":          "13.35.222.88",
		"*tumblr.com":           "192.0.77.40",
		"*twitch.tv":            "151.101.194.167",
		"*v2ex.com":             "172.66.137.6",
		"*vercel.app":           "64.29.17.193",
		"*vimeo.com":            "162.159.138.60",
		"*whatsapp.com":         "157.240.225.60",
		"*whatsapp.net":         "157.240.225.60",
		"*wikibooks.org":        "185.15.59.224",
		"*wikidata.org":         "185.15.59.224",
		"*wikifunctions.org":    "185.15.59.224",
		"*wikimedia.org":        "185.15.59.224",
		"*wikinews.org":         "185.15.59.224",
		"*wikipedia.org":        "185.15.59.224",
		"*wikiquote.org":        "185.15.59.224",
		"*wikisource.org":       "185.15.59.224",
		"*wikiversity.org":      "185.15.59.224",
		"*wikivoyage.org":       "185.15.59.224",
		"*wiktionary.org":       "185.15.59.224",
		"*xhamster.com":         "104.17.35.109",
		"*xhamster42.desi":      "104.17.35.109",
		"*xnxx.com":             "89.222.127.8",
		"*xvideos.com":          "89.222.127.8",
		"*xvideos.red":          "89.222.127.8",
		"*youtu.be":             "64.233.160.142",
		"*youtube-nocookie.com": "64.233.160.142",
		"*youtube.com":          "64.233.160.142",
		"*z-lib.help":           "176.123.7.105",
		"*z-library.sk":         "176.123.7.105",
		"a5.behance.net":        "13.35.185.106",
		"account-api.proton.me": "185.70.42.20",
		"account.nicovideo.jp":  "3.167.200.113",
		"account.proton.me":     "185.70.42.36",
		// snirect-specific: these live *.github.com vhosts bounce (301 →
		// github.com) on the empty-SNI default vhost of the *github.com web
		// edge. Pin each to an edge whose vhost tier serves it: the beacon/ws
		// pair at 140.82.114.26 (alive also answers collector's Host).
		"alive.github.com":      "140.82.114.26",
		"collector.github.com":  "140.82.114.26",
		"api.audiomack.com":     "3.167.200.113",
		"api.github.com":        "20.205.243.168",
		"ask.vrchat.com":        "216.66.8.43",
		"assets.audiomack.com":  "3.167.200.113",
		"assets.twitch.tv":      "3.167.200.113",
		"auth.docker.io":        "hub-origin.docker.com",
		"blog.docker.io":        "elb-io.us-east-1.aws.dckr.io",
		"business.whatsapp.com": "157.240.22.169",
		// snirect-specific: googleapis API hosts (glm-agy: Cloud Code Assist +
		// OAuth token) get poisoned DNS answers from mainland resolvers, and the
		// 120.253.253.0/24 mainland edge is cache-only (404s every non-static
		// path). 64.233.160.142 is a full GFE that is reachable from CN and
		// serves the real API backends on the original SNI with a Google Trust
		// Services leaf covering *.googleapis.com (validated live 2026-10-28
		// from a Tencent Cloud GZ VPS: token → 400 unsupported_grant_type).
		// Hosts-only pin on purpose: no SNI rewrite, TLS passes through, no
		// CertVerify needed.
		"cloudcode-pa.googleapis.com":       "64.233.160.142",
		"daily-cloudcode-pa.googleapis.com": "64.233.160.142",
		"oauth2.googleapis.com":             "64.233.160.142",
		"calendar.proton.me":                "185.70.42.39",
		"cdn-registry-1.docker.io":          "registry.ipv4.docker.com",
		"cdn.jsdelivr.net":                  "104.16.89.20",
		"cdn1.cdn-telegram.org":             "34.111.15.3",
		"cdn4.cdn-telegram.org":             "34.111.35.152",
		"cdn5.cdn-telegram.org":             "34.111.108.175",
		"codeload.github.com":               "20.205.243.165",
		"community.github.com":              "140.82.114.17",
		"consent.yahoo.com":                 "consent.yahoo.com",
		"docker.io":                         "elb-io.us-east-1.aws.dckr.io",
		"docs.docker.io":                    "elb-io.us-east-1.aws.dckr.io",
		"docs-editor.proton.me":             "185.70.42.70",
		"docs-rts.proton.me":                "185.70.42.71",
		"docs.github.com":                   "185.199.110.154",
		"docs.proton.me":                    "185.70.42.69",
		// 1.1.4.55 splits discuss off *huggingface.co: the Discourse host
		// answers Host: discuss.huggingface.co on empty SNI with a
		// *.hosted-by-discourse.com leaf (200, probed 2026-10-29).
		"discuss.huggingface.co":          "hellohellohello.hosted-by-discourse.com",
		"drive.proton.me":                 "185.70.42.40",
		"embed.nicovideo.jp":              "3.167.200.113",
		"external-content.duckduckgo.com": "20.43.160.189",
		"f-droid.org":                     "37.218.243.72",
		"fdroid.org":                      "37.218.243.72",
		"forum.f-droid.org":               "37.218.242.53",
		"fra-storage.proton.me":           "185.205.70.10",
		"gemini.google.com":               "47.102.115.14",
		"get.docker.io":                   "elb-io.us-east-1.aws.dckr.io",
		"gql.twitch.tv":                   "151.101.194.167",
		// Dial the distribution's own name instead of an edge IP: CloudFront
		// rotates all four 18.245.86.x edges per resolver answer, while the
		// d2is9o3959y6go.cloudfront.net hostname stays valid. Upstream's
		// 18.65.14.4 belongs to a different (dead) distribution.
		"graphql.api.dailymotion.com": "d2is9o3959y6go.cloudfront.net",
		"guce.yahoo.com":              "guce.yahoo.com",
		"hello.vrchat.com":            "198.185.159.145",
		"hub.docker.com":              "hub-origin.docker.com",
		"i.ytimg.com":                 "64.233.160.142",

		"identity.flickr.com": "34.192.66.226",
		"ig.me":               "157.240.27.174",
		"images.prismic.io":   "151.101.78.208",

		"index.docker.io":                      "registry.ipv4.docker.com",
		"irc-ws.chat.twitch.tv":                "3.167.200.113",
		"live.nicovideo.jp":                    "3.167.200.113",
		"live2.nicovideo.jp":                   "3.167.200.113",
		"lumo.proton.me":                       "185.70.42.51",
		"m.twitch.tv":                          "151.101.194.167",
		"mail.proton.me":                       "185.70.42.37",
		"meet.proton.me":                       "185.70.42.118",
		"music.audiomack.com":                  "3.167.200.113",
		"notary.docker.io":                     "elb-io.us-east-1.aws.dckr.io",
		"nvapi.nicovideo.jp":                   "3.167.200.113",
		"objects-origin.githubusercontent.com": "140.82.113.22",
		"ok.ru":                                "5.61.23.11",
		"panels.twitch.tv":                     "3.167.200.113",
		"pass.proton.me":                       "185.70.42.63",
		"passport.twitch.tv":                   "3.167.200.113",
		"polymarket.com":                       "64.239.109.1",
		"production.cloudflare.docker.com":     "104.16.100.215",
		"registry-1.docker.io":                 "registry.ipv4.docker.com",
		"registry.docker.io":                   "registry.ipv4.docker.com",
		"registry.hub.docker.com":              "elb-io.us-east-1.aws.dckr.io",
		"resources.github.com":                 "185.199.108.133",
		"services.github.com":                  "140.82.113.18",
		"sheets.proton.me":                     "185.70.42.69",
		"sp.nicovideo.jp":                      "3.167.200.113",
		"sso.proton.me":                        "185.70.42.20",
		"status.docker.io":                     "elb-io.us-east-1.aws.dckr.io",
		// 1.1.4.55 splits status off *huggingface.co: Better Uptime serves it
		// under the status.singpass.gov.sg SNI (200, probed 2026-10-29).
		"status.huggingface.co": "statuspage.betteruptime.com",
		// Upstream's 23.39.61.133 store front is dead: TCP opens but the empty-SNI
		// handshake aborts (remote error: tls: internal error, probed 2026-10-28).
		// 184.50.187.66 (the Valve front pinned for *steamcommunity.com) does
		// answer store on empty SNI, but it is Valve's China POP: it serves the
		// CN storefront — CNY prices and store.cdn.queniuqe.com assets — to every
		// client regardless of egress IP, so a non-CN user would silently get a
		// foreign storefront. 23.217.182.113 is an Akamai edge that geo-routes
		// per client IP: CN clients get CNY + queniuqe, other egress gets the
		// global storefront (verified 2026-10-28: prices and CDN hosts match the
		// unpinned direct fetch). Leaf is Let's Encrypt CN=store.steampowered.com
		// (valid to 2027-01); 200 on empty SNI from both a JP and a CN egress.
		"store.steampowered.com": "23.217.182.113",
		"support.github.com":     "185.199.108.133",
		"t.me":                   "149.154.166.110",
		"telegram.me":            "149.154.166.110",
		// 1.1.4.55 moved upld.e-hentai.org from the retired 89.149.221.236
		// (443 refused) to 85.17.247.144, whose empty-SNI vhost answers the
		// Host with e-hentai's LE leaf (SAN *.e-hentai.org) and a 301.
		// Probed live 2026-10-29 from a Tencent Cloud GZ VPS.
		"upld.e-hentai.org": "85.17.247.144",
		// snirect-specific: upload.github.com has no public A record (GitHub
		// removed it from DNS; only the 302 from github.com/git-receive-pack
		// references it). Falling through to the *github.com web edge gets a
		// 301 back to github.com — the same bounce codeload had. The git edge
		// (codeload's IP) serves Host: upload.github.com on empty SNI with a
		// *.github.com leaf, so pushes land there instead.
		"upload.github.com": "20.205.243.165",
		// Same bounce class as upload above, but for release-asset *uploads*
		// (gh release upload / web attachments): the API plane at api's edge
		// serves Host: uploads.github.com on empty SNI.
		"uploads.github.com":    "20.205.243.168",
		"upload.wikimedia.org":  "208.80.154.240",
		"w.wiki":                "185.15.59.224",
		"wallet.proton.me":      "194.0.147.12",
		"wktk.nicovideo.jp":     "3.167.200.113",
		"www.docker.io":         "elb-io.us-east-1.aws.dckr.io",
		"www.f-droid.org":       "37.218.243.72",
		"www.ok.ru":             "5.61.23.11",
		"zrh-storage.proton.me": "176.119.200.8",
	},
	// Front certs that are known to ship expired / short-lived LE leaves.
	// Without these, VerifyCert rejects the handshake even when the allowlist
	// would otherwise accept the domain names.
	IgnoreExpiry: map[string]bool{
		"*z-lib.help":           true,
		"*z-library.sk":         true,
		"*cdn-telegram.org":     true,
		"cdn1.cdn-telegram.org": true,
		"cdn4.cdn-telegram.org": true,
		"cdn5.cdn-telegram.org": true,
	},
	// Pre-sorted key indexes are NOT populated here. LoadRules() calls Init(),
	// which rebuilds them from the maps above via getSortedKeys — the maps are
	// the single source of truth. Hardcoding the slices here was a latent bug:
	// any map edit that wasn't mirrored into the slice became invisible to the
	// pattern-matching accessors (GetAlterHostname/GetCertVerify/GetHost).
}
