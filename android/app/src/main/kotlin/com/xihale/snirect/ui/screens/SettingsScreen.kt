package com.xihale.snirect.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.Bolt
import androidx.compose.material.icons.outlined.BugReport
import androidx.compose.material.icons.outlined.Code
import androidx.compose.material.icons.outlined.Compress
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.FilterAlt
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Language
import androidx.compose.material.icons.outlined.PowerSettingsNew
import androidx.compose.material.icons.outlined.Route
import androidx.compose.material.icons.outlined.Router
import androidx.compose.material.icons.outlined.Schedule
import androidx.compose.material.icons.outlined.SystemUpdate
import androidx.compose.material.icons.outlined.Update
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.xihale.snirect.BuildConfig
import com.xihale.snirect.R
import com.xihale.snirect.data.repository.ConfigRepository
import com.xihale.snirect.ktlib.AppUpdate
import com.xihale.snirect.ui.components.AppScreenScaffold
import com.xihale.snirect.ui.components.SOURCE_URL
import com.xihale.snirect.ui.components.SettingsGroup
import com.xihale.snirect.ui.components.SettingsOptionRow
import com.xihale.snirect.ui.components.SettingsTile
import com.xihale.snirect.ui.components.UpdateCheckDialogs
import com.xihale.snirect.ui.components.checkForUpdate
import com.xihale.snirect.ui.components.openInBrowser
import com.xihale.snirect.ui.theme.SnirectSpacing
import com.xihale.snirect.util.AppLogger
import kotlinx.coroutines.launch

@Composable
fun SettingsScreen(
    navController: NavController,
    repository: ConfigRepository
) {
    val scope = rememberCoroutineScope()
    val context = LocalContext.current

    var mtu by remember { mutableStateOf("1500") }
    var ipv6Mode by remember { mutableIntStateOf(ConfigRepository.IPV6_MODE_EXCLUDE_V6) }
    var logLevel by remember { mutableStateOf("info") }
    var activateOnStartup by remember { mutableStateOf(true) }
    var activateOnBoot by remember { mutableStateOf(false) }
    var language by remember { mutableStateOf(ConfigRepository.LANGUAGE_SYSTEM) }
    var filterMode by remember { mutableIntStateOf(ConfigRepository.FILTER_MODE_NONE) }
    var whitelistPackages by remember { mutableStateOf(setOf<String>()) }
    var bypassLan by remember { mutableStateOf(true) }
    var nameservers by remember { mutableStateOf<List<String>>(emptyList()) }
    var autoUpdateCheck by remember { mutableStateOf(true) }
    var updateCheckInterval by remember { mutableIntStateOf(ConfigRepository.UPDATE_INTERVAL_DAILY) }
    var checkingUpdate by remember { mutableStateOf(false) }
    var updateResult by remember { mutableStateOf<AppUpdate?>(null) }
    var updateError by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) { repository.mtu.collect { mtu = it.toString() } }
    LaunchedEffect(Unit) { repository.ipv6Mode.collect { ipv6Mode = it } }
    LaunchedEffect(Unit) { repository.logLevel.collect { logLevel = it } }
    LaunchedEffect(Unit) { repository.activateOnStartup.collect { activateOnStartup = it } }
    LaunchedEffect(Unit) { repository.activateOnBoot.collect { activateOnBoot = it } }
    LaunchedEffect(Unit) { repository.language.collect { language = it } }
    LaunchedEffect(Unit) { repository.filterMode.collect { filterMode = it } }
    LaunchedEffect(Unit) { repository.whitelistPackages.collect { whitelistPackages = it } }
    LaunchedEffect(Unit) { repository.bypassLan.collect { bypassLan = it } }
    LaunchedEffect(Unit) { repository.nameservers.collect { nameservers = it } }
    LaunchedEffect(Unit) { repository.autoUpdateCheck.collect { autoUpdateCheck = it } }
    LaunchedEffect(Unit) { repository.updateCheckInterval.collect { updateCheckInterval = it } }

    var showMtuDialog by remember { mutableStateOf(false) }

    AppScreenScaffold(
        title = stringResource(R.string.settings_title),
        onBack = { navController.popBackStack() },
        backContentDescription = stringResource(R.string.action_back)
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = SnirectSpacing.marginMobile, vertical = SnirectSpacing.medium),
            verticalArrangement = Arrangement.spacedBy(SnirectSpacing.large)
        ) {
            // 1. GENERAL GROUP
            SettingsGroup(title = stringResource(R.string.group_general)) {
                SettingsOptionRow(
                    icon = Icons.Outlined.Language,
                    title = stringResource(R.string.setting_language),
                    options = listOf(
                        ConfigRepository.LANGUAGE_SYSTEM to stringResource(R.string.option_lang_system),
                        "en" to stringResource(R.string.lang_en),
                        "zh" to stringResource(R.string.lang_zh)
                    ),
                    selectedKey = language,
                    onSelect = { selected ->
                        language = selected
                        scope.launch { repository.setLanguage(selected) }
                    }
                )
            }

            // 2. NETWORK GROUP
            SettingsGroup(title = stringResource(R.string.group_network)) {
                SettingsOptionRow(
                    icon = Icons.Outlined.FilterAlt,
                    title = stringResource(R.string.setting_filter_mode),
                    options = listOf(
                        ConfigRepository.FILTER_MODE_NONE to stringResource(R.string.option_filter_none),
                        ConfigRepository.FILTER_MODE_WHITELIST to stringResource(R.string.option_filter_whitelist),
                        ConfigRepository.FILTER_MODE_BLACKLIST to stringResource(R.string.option_filter_blacklist)
                    ),
                    selectedKey = filterMode,
                    onSelect = { selected ->
                        filterMode = selected
                        scope.launch { repository.setFilterMode(selected) }
                    }
                )

                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                SettingsTile(
                    icon = Icons.Outlined.Apps,
                    title = stringResource(R.string.setting_whitelist_apps),
                    subtitle = stringResource(R.string.setting_whitelist_apps_count, whitelistPackages.size),
                    showChevron = true,
                    onClick = { navController.navigate("app_whitelist") }
                )

                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                SettingsOptionRow(
                    icon = Icons.Outlined.Route,
                    title = stringResource(R.string.setting_ipv6_control),
                    options = listOf(
                        ConfigRepository.IPV6_MODE_ONLY_V4 to stringResource(R.string.ipv6_only_v4),
                        ConfigRepository.IPV6_MODE_EXCLUDE_V6 to stringResource(R.string.ipv6_exclude_v6),
                        ConfigRepository.IPV6_MODE_FULL_V6 to stringResource(R.string.ipv6_full_v6)
                    ),
                    selectedKey = ipv6Mode,
                    onSelect = { selected ->
                        ipv6Mode = selected
                        scope.launch { repository.setIpv6Mode(selected) }
                    }
                )

                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                SettingsTile(
                    icon = Icons.Outlined.Router,
                    title = stringResource(R.string.setting_bypass_lan),
                    subtitle = stringResource(R.string.setting_bypass_lan_desc),
                    trailing = {
                        Switch(
                            checked = bypassLan,
                            onCheckedChange = {
                                bypassLan = it
                                scope.launch { repository.setBypassLan(it) }
                            }
                        )
                    }
                )
            }

            // 3. SECURITY & STARTUP GROUP
            SettingsGroup(title = stringResource(R.string.group_security_startup)) {
                SettingsTile(
                    icon = Icons.Outlined.PowerSettingsNew,
                    title = stringResource(R.string.setting_active_startup),
                    subtitle = stringResource(R.string.setting_active_startup_desc),
                    trailing = {
                        Switch(
                            checked = activateOnStartup,
                            onCheckedChange = {
                                activateOnStartup = it
                                scope.launch { repository.setActivateOnStartup(it) }
                            }
                        )
                    }
                )

                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                SettingsTile(
                    icon = Icons.Outlined.Bolt,
                    title = stringResource(R.string.setting_active_boot),
                    subtitle = stringResource(R.string.setting_active_boot_desc),
                    trailing = {
                        Switch(
                            checked = activateOnBoot,
                            onCheckedChange = {
                                activateOnBoot = it
                                scope.launch { repository.setActivateOnBoot(it) }
                            }
                        )
                    }
                )

                // High-risk settings live on their own screen — they are
                // rarely touched and visually loud (error colors).
                SettingsTile(
                    icon = Icons.Default.Warning,
                    iconTint = MaterialTheme.colorScheme.error,
                    title = stringResource(R.string.risk_settings_title),
                    subtitle = stringResource(R.string.risk_settings_desc),
                    showChevron = true,
                    onClick = { navController.navigate("risk_settings") }
                )
            }

            // 4. DNS CONFIGURATION GROUP
            SettingsGroup(title = stringResource(R.string.group_dns_configuration)) {
                val dnsSummary = if (nameservers.isEmpty()) {
                    stringResource(R.string.no_custom_nameservers)
                } else {
                    nameservers.joinToString(", ")
                }

                SettingsTile(
                    icon = Icons.Outlined.Dns,
                    title = stringResource(R.string.setting_upstream_servers),
                    subtitle = dnsSummary,
                    showChevron = true,
                    onClick = { navController.navigate("dns") }
                )
            }

            // 5. ADVANCED GROUP
            SettingsGroup(title = stringResource(R.string.group_advanced)) {
                SettingsTile(
                    icon = Icons.Outlined.Compress,
                    title = stringResource(R.string.setting_mtu),
                    subtitle = "$mtu Bytes",
                    trailing = {
                        Icon(
                            imageVector = Icons.Default.Edit,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    },
                    onClick = { showMtuDialog = true }
                )

                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                SettingsOptionRow(
                    icon = Icons.Outlined.BugReport,
                    title = stringResource(R.string.setting_log_level),
                    options = listOf(
                        "debug" to "DEBUG",
                        "info" to "INFO",
                        "warn" to "WARN",
                        "error" to "ERROR"
                    ),
                    selectedKey = logLevel.lowercase(),
                    onSelect = { selected ->
                        logLevel = selected
                        scope.launch { repository.setLogLevel(selected) }
                    }
                )
            }

            // 6. ABOUT GROUP — version, source, and update checks. The
            // manual check also refreshes lastUpdateCheck, keeping the
            // next silent auto-check one full interval away.
            SettingsGroup(title = stringResource(R.string.group_about)) {
                SettingsTile(
                    icon = Icons.Outlined.Info,
                    title = stringResource(R.string.about_version),
                    subtitle = BuildConfig.VERSION_NAME
                )

                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                SettingsTile(
                    icon = Icons.Outlined.Code,
                    title = stringResource(R.string.about_source_code),
                    subtitle = SOURCE_URL.removePrefix("https://"),
                    onClick = { openInBrowser(context, SOURCE_URL) }
                )

                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                SettingsTile(
                    icon = Icons.Outlined.Update,
                    title = stringResource(R.string.about_check_update),
                    subtitle = if (checkingUpdate) stringResource(R.string.update_checking) else null,
                    onClick = {
                        if (!checkingUpdate) {
                            checkingUpdate = true
                            updateResult = null
                            updateError = null
                            scope.launch {
                                try {
                                    val info = checkForUpdate(context)
                                    repository.setLastUpdateCheck(System.currentTimeMillis())
                                    updateResult = info
                                } catch (e: Exception) {
                                    AppLogger.e("check update failed", e)
                                    updateError = e.message ?: e.toString()
                                } finally {
                                    checkingUpdate = false
                                }
                            }
                        }
                    }
                )

                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                SettingsTile(
                    icon = Icons.Outlined.SystemUpdate,
                    title = stringResource(R.string.about_auto_check),
                    subtitle = stringResource(R.string.about_auto_check_desc),
                    trailing = {
                        Switch(
                            checked = autoUpdateCheck,
                            onCheckedChange = {
                                autoUpdateCheck = it
                                scope.launch { repository.setAutoUpdateCheck(it) }
                            }
                        )
                    }
                )

                // Frequency only matters while auto-check is on; the group's
                // animateContentSize slides the row in/out with the toggle.
                if (autoUpdateCheck) {
                    HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)

                    SettingsOptionRow(
                        icon = Icons.Outlined.Schedule,
                        title = stringResource(R.string.about_check_frequency),
                        options = listOf(
                            ConfigRepository.UPDATE_INTERVAL_EVERY_LAUNCH to stringResource(R.string.option_freq_every_launch),
                            ConfigRepository.UPDATE_INTERVAL_DAILY to stringResource(R.string.option_freq_daily),
                            ConfigRepository.UPDATE_INTERVAL_WEEKLY to stringResource(R.string.option_freq_weekly)
                        ),
                        selectedKey = updateCheckInterval,
                        onSelect = { selected ->
                            updateCheckInterval = selected
                            scope.launch { repository.setUpdateCheckInterval(selected) }
                        }
                    )
                }
            }
        }
    }

    UpdateCheckDialogs(
        updateResult = updateResult,
        updateError = updateError,
        onClearResult = { updateResult = null },
        onClearError = { updateError = null }
    )

    // MTU Edit Dialog
    if (showMtuDialog) {
        var tempMtu by remember { mutableStateOf(mtu) }
        AlertDialog(
            onDismissRequest = { showMtuDialog = false },
            title = { Text(stringResource(R.string.setting_mtu)) },
            text = {
                OutlinedTextField(
                    value = tempMtu,
                    onValueChange = { value ->
                        if (value.all { it.isDigit() }) {
                            tempMtu = value
                        }
                    },
                    label = { Text(stringResource(R.string.label_bytes)) },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number)
                )
            },
            confirmButton = {
                Button(
                    onClick = {
                        tempMtu.toIntOrNull()?.let { value ->
                            mtu = value.toString()
                            scope.launch { repository.setMtu(value) }
                        }
                        showMtuDialog = false
                    }
                ) {
                    Text(stringResource(R.string.action_save))
                }
            },
            dismissButton = {
                TextButton(onClick = { showMtuDialog = false }) {
                    Text(stringResource(R.string.action_cancel))
                }
            }
        )
    }
}
