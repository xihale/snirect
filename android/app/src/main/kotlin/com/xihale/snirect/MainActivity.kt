package com.xihale.snirect

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.widget.Toast
import androidx.activity.result.ActivityResultLauncher
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.appcompat.app.AppCompatDelegate
import androidx.activity.compose.setContent
import androidx.compose.animation.*
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.core.os.LocaleListCompat
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.viewModelScope
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.xihale.snirect.R
import com.xihale.snirect.data.model.LogEntry
import com.xihale.snirect.data.model.LogLevel
import com.xihale.snirect.data.repository.ConfigRepository
import com.xihale.snirect.service.SnirectVpnService
import com.xihale.snirect.service.VpnStatusManager
import com.xihale.snirect.ktlib.SnirectClient
import com.xihale.snirect.ui.screens.AppWhitelistScreen
import com.xihale.snirect.ui.screens.DnsScreen
import com.xihale.snirect.ui.screens.HelpScreen
import com.xihale.snirect.ui.screens.HomeScreen
import com.xihale.snirect.ui.screens.LogsScreen
import com.xihale.snirect.ui.screens.RiskSettingsScreen
import com.xihale.snirect.ui.screens.SettingsScreen
import com.xihale.snirect.ui.theme.SnirectMotion
import com.xihale.snirect.ui.theme.SnirectTheme
import com.xihale.snirect.util.AppLogger
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

/**
 * Bridges ViewModel permission requests to activity-scoped result launchers.
 * Implemented by [MainActivity] so the launchers live at activity level
 * (registered before onStart, valid for the whole activity lifetime) instead
 * of being handed down from composable composition.
 */
interface PermissionGateway {
    fun launchVpnConsent(intent: Intent)
}

class MainViewModelFactory(
    private val appContext: Context,
    private val repository: ConfigRepository,
    private val permissionGateway: PermissionGateway,
) : ViewModelProvider.Factory {
    override fun <T : ViewModel> create(modelClass: Class<T>): T {
        if (modelClass.isAssignableFrom(MainViewModel::class.java)) {
            @Suppress("UNCHECKED_CAST")
            return MainViewModel(appContext, repository, permissionGateway) as T
        }
        throw IllegalArgumentException("Unknown ViewModel class")
    }
}

class MainViewModel(
    private val appContext: Context,
    private val repository: ConfigRepository,
    private var permissionGateway: PermissionGateway,
) : ViewModel() {
    var isRunning by mutableStateOf(false)
    var statusText by mutableStateOf("DISCONNECTED")

    // Process-lifetime cache: scanning AndroidCAStore walks every trusted
    // alias and is too expensive to repeat on every Home recomposition.
    // Null = not scanned yet (Home shows "checking"); Help / verify force
    // a refresh so a just-installed cert is picked up.
    var isCertInstalled by mutableStateOf<Boolean?>(null)
        private set

    init {
        // ktlib's EngineState flow (via VpnStatusManager) is the single source
        // of truth; the ViewModel only republishes it for Compose.
        VpnStatusManager.attach(SnirectClient.from(appContext))
        viewModelScope.launch {
            VpnStatusManager.isRunning.collect { isRunning = it }
        }
        viewModelScope.launch {
            VpnStatusManager.statusText.collect { statusText = it }
        }
        refreshCertStatus()
    }

    /** Scan once; later Home visits reuse [isCertInstalled]. */
    fun ensureCertStatus() {
        if (isCertInstalled != null) return
        refreshCertStatus()
    }

    fun refreshCertStatus() {
        viewModelScope.launch { scanCertStatus() }
    }

    /** Walks AndroidCAStore and publishes the result. Used by Help after install. */
    suspend fun scanCertStatus(): Boolean {
        val installed = runCatching {
            SnirectClient.from(appContext).isCaCertificateInstalled()
        }.getOrElse { e ->
            AppLogger.e("CA cert check failed", e)
            false
        }
        isCertInstalled = installed
        return installed
    }

    /**
     * Re-points the gateway at the current activity's launchers. The ViewModel
     * survives configuration changes; the launchers do not, so every
     * MainActivity.onCreate refreshes this reference.
     */
    fun attachGateway(gateway: PermissionGateway) {
        permissionGateway = gateway
    }

    fun toggleVpn(context: Context) {
        if (isRunning || statusText == "STARTING") {
            stopVpn(context)
        } else if (statusText == "STOPPING") {
            // Teardown is still closing the TUN; a start here used to race
            // the previous session and crash. The button is disabled, but
            // keep the ViewModel itself idempotent.
            return
        } else {
            // Publish before the suspend cert scan so a tile launch and the
            // Home auto-start cannot both decide to connect.
            VpnStatusManager.updateStatus(false, "STARTING")
            viewModelScope.launch {
                val skipCheck = repository.skipCertCheck.first()
                // Reuse the Home cache when it already says installed; only
                // rescan when unknown or previously missing (user may have
                // just imported the CA).
                if (!skipCheck) {
                    val installed = if (isCertInstalled == true) true else scanCertStatus()
                    if (!installed) {
                        VpnStatusManager.updateStatus(false, "DISCONNECTED")
                        Toast.makeText(context, context.getString(R.string.toast_cert_install_required), Toast.LENGTH_LONG).show()
                        return@launch
                    }
                }
                prepareVpn(context)
            }
        }
    }

    private fun prepareVpn(context: Context) {
        val intent = VpnService.prepare(context)
        if (intent != null) {
            permissionGateway.launchVpnConsent(intent)
        } else {
            startVpn(context)
        }
    }

    /** Called by MainActivity's VPN-consent launcher with the dialog outcome. */
    fun onVpnConsentResult(granted: Boolean) {
        if (!granted) {
            AppLogger.w("VPN consent denied by user")
            VpnStatusManager.updateStatus(false, "DISCONNECTED")
            Toast.makeText(appContext, appContext.getString(R.string.toast_vpn_permission_denied), Toast.LENGTH_SHORT).show()
            return
        }
        viewModelScope.launch {
            val skipCheck = repository.skipCertCheck.first()
            if (!skipCheck) {
                val installed = if (isCertInstalled == true) true else scanCertStatus()
                if (!installed) {
                    AppLogger.w("VPN Launcher: Blocked - CA certificate not installed")
                    VpnStatusManager.updateStatus(false, "DISCONNECTED")
                    Toast.makeText(appContext, appContext.getString(R.string.toast_cert_install_required), Toast.LENGTH_LONG).show()
                    return@launch
                }
            }
            startVpn(appContext)
        }
    }

    fun startVpn(context: Context) {
        val intent = Intent(context, SnirectVpnService::class.java).apply {
            action = SnirectVpnService.ACTION_START
        }
        // Plain started service: the app is in the foreground here, and no
        // notification/status-bar presence is wanted for the VPN session.
        context.startService(intent)
        VpnStatusManager.updateStatus(false, "STARTING")
    }

    private fun stopVpn(context: Context) {
        val intent = Intent(context, SnirectVpnService::class.java).apply {
            action = SnirectVpnService.ACTION_STOP
        }
        context.startService(intent)
        VpnStatusManager.updateStatus(false, "STOPPING")
    }

    fun installCert(context: Context) {
        viewModelScope.launch {
            try {
                AppLogger.i("Starting CA certificate export using MediaStore...")
                val certBytes = SnirectClient.from(context).caCertificate() ?: throw Exception("Go core returned null cert")
                if (certBytes.isEmpty()) throw Exception("Go core returned empty cert")

                val resolver = context.contentResolver
                val contentValues = android.content.ContentValues().apply {
                    put(android.provider.MediaStore.MediaColumns.DISPLAY_NAME, "snirect_ca.crt")
                    put(android.provider.MediaStore.MediaColumns.MIME_TYPE, "application/x-x509-ca-cert")
                    put(android.provider.MediaStore.MediaColumns.RELATIVE_PATH, android.os.Environment.DIRECTORY_DOWNLOADS)
                }

                val uri = resolver.insert(android.provider.MediaStore.Downloads.EXTERNAL_CONTENT_URI, contentValues)
                uri?.let {
                    resolver.openOutputStream(it)?.use { outputStream ->
                        outputStream.write(certBytes)
                    }
                    AppLogger.i("CA cert saved to Downloads via MediaStore: $uri")
                    Toast.makeText(context, context.getString(R.string.toast_saved_to_downloads), Toast.LENGTH_LONG).show()

                    val intent = Intent(android.provider.Settings.ACTION_SECURITY_SETTINGS)
                    context.startActivity(intent)
                } ?: throw Exception("Failed to create MediaStore entry")
            } catch (e: Exception) {
                AppLogger.e("CA export failed", e)
                Toast.makeText(context, context.getString(R.string.toast_export_failed, e.message), Toast.LENGTH_SHORT).show()
            }
        }
    }
}

class MainActivity : AppCompatActivity() {
    companion object {
        const val EXTRA_START_FROM_TILE = "com.xihale.snirect.START_FROM_TILE"

        val logBuffer = mutableStateListOf<LogEntry>()
        fun log(message: String) {
            val level = when {
                message.contains("[ERROR]", true) || message.contains("ERROR:", true) -> LogLevel.ERROR
                message.contains("[WARN]", true) || message.contains("WARN:", true) -> LogLevel.WARN
                message.contains("[DEBUG]", true) || message.contains("DEBUG:", true) -> LogLevel.DEBUG
                else -> LogLevel.INFO
            }
            val cleanMessage = message
                .replace("[ERROR]", "")
                .replace("[WARN]", "")
                .replace("[DEBUG]", "")
                .replace("[INFO]", "")
                .trim()

            Handler(Looper.getMainLooper()).post {
                if (logBuffer.size > 2000) logBuffer.removeAt(0)
                logBuffer.add(LogEntry(level = level, message = cleanMessage))
            }
        }
    }

    private lateinit var viewModel: MainViewModel

    // Registered during construction (well before onStart), so they stay valid
    // for the whole activity lifetime instead of only while HomeScreen is
    // composed. Both route their outcome through the shared ViewModel.
    private val vpnPermissionLauncher: ActivityResultLauncher<Intent> =
        registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
            viewModel.onVpnConsentResult(result.resultCode == Activity.RESULT_OK)
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        AppLogger.i("App Starting...")
        AppLogger.init(applicationContext)

        SnirectClient.from(this)

        val repository = ConfigRepository(applicationContext)
        val gateway = object : PermissionGateway {
            override fun launchVpnConsent(intent: Intent) {
                vpnPermissionLauncher.launch(intent)
            }
        }
        val factory = MainViewModelFactory(applicationContext, repository, gateway)
        viewModel = ViewModelProvider(this, factory)[MainViewModel::class.java]
        viewModel.attachGateway(gateway)
        handleTileIntent(intent)

        lifecycleScope.launch {
            repository.language.collect { lang ->
                val appLocale: LocaleListCompat = if (lang == ConfigRepository.LANGUAGE_SYSTEM) {
                    LocaleListCompat.getEmptyLocaleList()
                } else {
                    LocaleListCompat.forLanguageTags(lang)
                }
                AppCompatDelegate.setApplicationLocales(appLocale)
            }
        }
        // The log-level setting also gates the in-app logger, so changing it
        // visibly changes what the Logs screen captures.
        lifecycleScope.launch {
            repository.logLevel.collect { AppLogger.setMinLevel(it) }
        }

        setContent {
            SnirectTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background
                ) {
                    val navController = rememberNavController()
                    NavHost(
                        navController = navController,
                        startDestination = "home",
                        // Quarter-width slide keeps the drill-down direction cue
                        // without the heaviness of a full-screen push; exits are
                        // always faster than entrances.
                        enterTransition = {
                            slideInHorizontally(
                                animationSpec = tween(
                                    SnirectMotion.ContentDurationMillis,
                                    easing = SnirectMotion.EnterEasing
                                )
                            ) { it / 4 } + fadeIn(
                                animationSpec = tween(SnirectMotion.MediumDurationMillis)
                            )
                        },
                        exitTransition = {
                            fadeOut(animationSpec = tween(SnirectMotion.ExitDurationMillis))
                        },
                        popEnterTransition = {
                            fadeIn(animationSpec = tween(SnirectMotion.MediumDurationMillis))
                        },
                        popExitTransition = {
                            slideOutHorizontally(
                                animationSpec = tween(
                                    SnirectMotion.ContentDurationMillis,
                                    easing = SnirectMotion.EnterEasing
                                )
                            ) { it / 4 } + fadeOut(
                                animationSpec = tween(SnirectMotion.ExitDurationMillis)
                            )
                        }
                    ) {
                        composable("home") {
                            HomeScreen(navController = navController, viewModel = viewModel, repository = repository)
                        }
                        composable("help") {
                            HelpScreen(navController = navController, viewModel = viewModel)
                        }
                        composable("settings") {
                            SettingsScreen(navController = navController, repository = repository)
                        }
                        composable("risk_settings") {
                            RiskSettingsScreen(navController = navController, repository = repository)
                        }
                        composable("dns") {
                            DnsScreen(navController = navController, repository = repository)
                        }
                        composable("app_whitelist") {
                            AppWhitelistScreen(navController = navController, repository = repository)
                        }
                        composable("logs") {
                            LogsScreen(navController = navController)
                        }
                    }
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleTileIntent(intent)
    }

    private fun handleTileIntent(intent: Intent?) {
        if (intent?.getBooleanExtra(EXTRA_START_FROM_TILE, false) != true) return
        intent.removeExtra(EXTRA_START_FROM_TILE)
        viewModel.toggleVpn(this)
    }
}
