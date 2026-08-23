package com.xihale.snirect.data.repository

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

val Context.dataStore: DataStore<Preferences> by preferencesDataStore(name = "settings")

class ConfigRepository(
    private val context: Context,
) {
    companion object {
        val KEY_NAMESERVERS = stringPreferencesKey("nameservers")
        val KEY_BOOTSTRAP_DNS = stringPreferencesKey("bootstrap_dns")
        val KEY_CHECK_HOSTNAME = booleanPreferencesKey("check_hostname")
        val KEY_MTU = intPreferencesKey("mtu")
        val KEY_LOG_LEVEL = stringPreferencesKey("log_level")
        val KEY_ACTIVATE_ON_STARTUP = booleanPreferencesKey("activate_on_startup")
        val KEY_ACTIVATE_ON_BOOT = booleanPreferencesKey("activate_on_boot")
        val KEY_SKIP_CERT_CHECK = booleanPreferencesKey("skip_cert_check")
        val KEY_LANGUAGE = stringPreferencesKey("language")
        val KEY_FILTER_MODE = intPreferencesKey("filter_mode") // 0: None, 1: Whitelist, 2: Blacklist
        val KEY_WHITELIST_PACKAGES = stringPreferencesKey("whitelist_packages")
        val KEY_BYPASS_LAN = booleanPreferencesKey("bypass_lan")
        val KEY_IPV6_MODE = intPreferencesKey("ipv6_mode")
        val KEY_AUTO_UPDATE_CHECK = booleanPreferencesKey("auto_update_check")
        val KEY_UPDATE_CHECK_INTERVAL = intPreferencesKey("update_check_interval")
        val KEY_LAST_UPDATE_CHECK = longPreferencesKey("last_update_check")
        // Legacy keys, read once to migrate into KEY_IPV6_MODE.
        val KEY_ENABLE_IPV6 = booleanPreferencesKey("enable_ipv6")
        val KEY_BLOCK_IPV6 = booleanPreferencesKey("block_ipv6")

        const val FILTER_MODE_NONE = 0
        const val FILTER_MODE_WHITELIST = 1
        const val FILTER_MODE_BLACKLIST = 2

        const val IPV6_MODE_ONLY_V4 = 0     // TUN has no v6 route; engine IPv6 off
        const val IPV6_MODE_EXCLUDE_V6 = 1  // v6 bypasses VPN; engine IPv6 off (default)
        const val IPV6_MODE_FULL_V6 = 2     // v6 through VPN; engine IPv6 on

        // Default upstream resolver.
        // Users can add/remove upstreams in the DNS screen.
        const val DEFAULT_NAMESERVERS = "https://dnschina1.soraharu.com/dns-query,tls://223.5.5.5"
        const val DEFAULT_BOOTSTRAP_DNS = "tls://223.5.5.5"
        const val LANGUAGE_SYSTEM = "system"

        // Auto update-check frequency (Settings → About).
        const val UPDATE_INTERVAL_EVERY_LAUNCH = 0
        const val UPDATE_INTERVAL_DAILY = 1
        const val UPDATE_INTERVAL_WEEKLY = 2

        /** Interval mode → minimum gap between two auto checks. */
        fun updateIntervalMs(mode: Int): Long = when (mode) {
            UPDATE_INTERVAL_EVERY_LAUNCH -> 0L
            UPDATE_INTERVAL_WEEKLY -> 7L * 24 * 60 * 60 * 1000
            else -> 24L * 60 * 60 * 1000
        }
    }

    val nameservers: Flow<List<String>> =
        context.dataStore.data.map { prefs ->
            (prefs[KEY_NAMESERVERS] ?: DEFAULT_NAMESERVERS).split(",").filter { it.isNotBlank() }
        }

    val bootstrapDns: Flow<List<String>> =
        context.dataStore.data.map { prefs ->
            (prefs[KEY_BOOTSTRAP_DNS] ?: DEFAULT_BOOTSTRAP_DNS).split(",").filter { it.isNotBlank() }
        }

    val checkHostname: Flow<Boolean> =
        context.dataStore.data.map { prefs ->
            prefs[KEY_CHECK_HOSTNAME] ?: true
        }

    val mtu: Flow<Int> = context.dataStore.data.map { it[KEY_MTU] ?: 1500 }
    val logLevel: Flow<String> = context.dataStore.data.map { it[KEY_LOG_LEVEL] ?: "info" }
    val activateOnStartup: Flow<Boolean> = context.dataStore.data.map { it[KEY_ACTIVATE_ON_STARTUP] ?: true }
    val activateOnBoot: Flow<Boolean> = context.dataStore.data.map { it[KEY_ACTIVATE_ON_BOOT] ?: false }
    val skipCertCheck: Flow<Boolean> = context.dataStore.data.map { it[KEY_SKIP_CERT_CHECK] ?: false }
    val language: Flow<String> = context.dataStore.data.map { it[KEY_LANGUAGE] ?: LANGUAGE_SYSTEM }
    val filterMode: Flow<Int> = context.dataStore.data.map { it[KEY_FILTER_MODE] ?: FILTER_MODE_NONE }
    val whitelistPackages: Flow<Set<String>> = context.dataStore.data.map {
        it[KEY_WHITELIST_PACKAGES]?.split(",")?.filter { p -> p.isNotBlank() }?.toSet() ?: emptySet()
    }
    val bypassLan: Flow<Boolean> = context.dataStore.data.map { it[KEY_BYPASS_LAN] ?: true }
    val autoUpdateCheck: Flow<Boolean> = context.dataStore.data.map { it[KEY_AUTO_UPDATE_CHECK] ?: true }
    val updateCheckInterval: Flow<Int> =
        context.dataStore.data.map { it[KEY_UPDATE_CHECK_INTERVAL] ?: UPDATE_INTERVAL_DAILY }
    val lastUpdateCheck: Flow<Long> = context.dataStore.data.map { it[KEY_LAST_UPDATE_CHECK] ?: 0L }

    val ipv6Mode: Flow<Int> = context.dataStore.data.map { prefs ->
        prefs[KEY_IPV6_MODE] ?: when {
            prefs[KEY_BLOCK_IPV6] == true -> IPV6_MODE_ONLY_V4
            prefs[KEY_ENABLE_IPV6] == true -> IPV6_MODE_FULL_V6
            else -> IPV6_MODE_EXCLUDE_V6
        }
    }

    suspend fun setNameservers(servers: List<String>) =
        context.dataStore.edit {
            it[KEY_NAMESERVERS] = servers.joinToString(",")
        }

    suspend fun setBootstrapDns(dns: List<String>) =
        context.dataStore.edit {
            it[KEY_BOOTSTRAP_DNS] = dns.joinToString(",")
        }

    suspend fun setCheckHostname(check: Boolean) = context.dataStore.edit { it[KEY_CHECK_HOSTNAME] = check }
    suspend fun setMtu(mtu: Int) = context.dataStore.edit { it[KEY_MTU] = mtu }
    suspend fun setLogLevel(level: String) = context.dataStore.edit { it[KEY_LOG_LEVEL] = level }
    suspend fun setActivateOnStartup(enable: Boolean) = context.dataStore.edit { it[KEY_ACTIVATE_ON_STARTUP] = enable }
    suspend fun setActivateOnBoot(enable: Boolean) = context.dataStore.edit { it[KEY_ACTIVATE_ON_BOOT] = enable }
    suspend fun setSkipCertCheck(skip: Boolean) = context.dataStore.edit { it[KEY_SKIP_CERT_CHECK] = skip }
    suspend fun setLanguage(lang: String) = context.dataStore.edit { it[KEY_LANGUAGE] = lang }
    suspend fun setFilterMode(mode: Int) = context.dataStore.edit { it[KEY_FILTER_MODE] = mode }
    suspend fun setWhitelistPackages(packages: Set<String>) = context.dataStore.edit {
        it[KEY_WHITELIST_PACKAGES] = packages.joinToString(",")
    }
    suspend fun setBypassLan(enable: Boolean) = context.dataStore.edit { it[KEY_BYPASS_LAN] = enable }
    suspend fun setAutoUpdateCheck(enable: Boolean) = context.dataStore.edit { it[KEY_AUTO_UPDATE_CHECK] = enable }
    suspend fun setUpdateCheckInterval(mode: Int) = context.dataStore.edit { it[KEY_UPDATE_CHECK_INTERVAL] = mode }
    suspend fun setLastUpdateCheck(epochMs: Long) = context.dataStore.edit { it[KEY_LAST_UPDATE_CHECK] = epochMs }

    suspend fun setIpv6Mode(mode: Int) = context.dataStore.edit { prefs ->
        prefs[KEY_IPV6_MODE] = mode
        prefs.remove(KEY_BLOCK_IPV6)
        prefs.remove(KEY_ENABLE_IPV6)
    }
}
