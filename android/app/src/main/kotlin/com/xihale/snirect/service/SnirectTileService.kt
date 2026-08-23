package com.xihale.snirect.service

import android.app.PendingIntent
import android.content.Intent
import android.graphics.drawable.Icon
import android.net.VpnService
import android.os.Build
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import android.widget.Toast
import com.xihale.snirect.MainActivity
import com.xihale.snirect.R
import com.xihale.snirect.data.repository.ConfigRepository
import com.xihale.snirect.ktlib.SnirectClient
import com.xihale.snirect.util.AppLogger
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.launchIn
import kotlinx.coroutines.flow.onEach
import kotlinx.coroutines.launch

class SnirectTileService : TileService() {

    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private var statusCollectorJob: Job? = null

    override fun onCreate() {
        super.onCreate()
        AppLogger.init(applicationContext)
    }

    override fun onStartListening() {
        super.onStartListening()
        AppLogger.d("Tile: onStartListening")

        VpnStatusManager.attach(SnirectClient.from(applicationContext))
        statusCollectorJob?.cancel()
        statusCollectorJob = VpnStatusManager.statusText
            .onEach { refreshTile() }
            .launchIn(serviceScope)
        refreshTile()
    }

    override fun onStopListening() {
        statusCollectorJob?.cancel()
        statusCollectorJob = null
        super.onStopListening()
    }

    override fun onDestroy() {
        serviceScope.cancel()
        super.onDestroy()
    }

    override fun onClick() {
        super.onClick()
        // unlockAndRun keeps the privileged window until the runnable
        // finishes, including after the user unlocks the device.
        unlockAndRun { handleClick() }
    }

    private fun handleClick() {
        val status = VpnStatusManager.statusText.value
        val running = VpnStatusManager.isRunning.value
        AppLogger.i("Tile: onClick status=$status running=$running")

        // A toggle: connecting or connected stops; only a settled idle
        // state starts. Treating STOPPING as "start" used to bounce the
        // VPN back up and made the tile feel stuck on.
        if (running || status == "STARTING" || status == "STOPPING") {
            stopFromTile()
        } else {
            startFromTile()
        }
    }

    private fun stopFromTile() {
        VpnStatusManager.updateStatus(false, "STOPPING")
        refreshTile()
        sendServiceAction(SnirectVpnService.ACTION_STOP)
    }

    private fun startFromTile() {
        val prepare = VpnService.prepare(this)
        if (prepare != null) {
            // Consent has to run in an activity; after grant, MainActivity
            // continues the start. The tile itself cannot collect the result.
            collapseToActivity(
                Intent(this, MainActivity::class.java).apply {
                    action = Intent.ACTION_MAIN
                    addCategory(Intent.CATEGORY_LAUNCHER)
                    putExtra(MainActivity.EXTRA_START_FROM_TILE, true)
                }
            )
            return
        }

        VpnStatusManager.updateStatus(false, "STARTING")
        refreshTile()
        // Must stay inside onClick/unlockAndRun. startService after a
        // suspend (cert scan, DataStore) is already in the cached state
        // and Android rejects it — the tile then "does nothing".
        if (!sendServiceAction(SnirectVpnService.ACTION_START)) {
            VpnStatusManager.updateStatus(false, "DISCONNECTED")
            refreshTile()
            return
        }

        serviceScope.launch {
            val skipCheck = ConfigRepository(applicationContext).skipCertCheck.first()
            if (skipCheck) return@launch
            val installed = runCatching {
                SnirectClient.from(applicationContext).isCaCertificateInstalled()
            }.getOrDefault(false)
            if (installed) return@launch
            AppLogger.w("Tile: start blocked — CA certificate not installed")
            Toast.makeText(
                applicationContext,
                getString(R.string.toast_cert_install_required),
                Toast.LENGTH_LONG
            ).show()
            stopFromTile()
        }
    }

    private fun sendServiceAction(action: String): Boolean {
        val intent = Intent(this, SnirectVpnService::class.java).apply {
            this.action = action
        }
        return try {
            startService(intent)
            true
        } catch (e: Exception) {
            AppLogger.e("Tile: startService($action) failed", e)
            Toast.makeText(
                applicationContext,
                getString(R.string.toast_error, e.message ?: action),
                Toast.LENGTH_SHORT
            ).show()
            false
        }
    }

    private fun collapseToActivity(intent: Intent) {
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            val pending = PendingIntent.getActivity(
                this,
                TILE_ACTIVITY_REQUEST,
                intent,
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
            )
            startActivityAndCollapse(pending)
        } else {
            @Suppress("DEPRECATION")
            startActivityAndCollapse(intent)
        }
    }

    private fun refreshTile() {
        val tile = qsTile ?: return
        val status = VpnStatusManager.statusText.value
        val running = VpnStatusManager.isRunning.value
        tile.state = when {
            running || status == "STARTING" -> Tile.STATE_ACTIVE
            else -> Tile.STATE_INACTIVE
        }
        tile.label = getString(R.string.app_name)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            tile.subtitle = when {
                status == "STARTING" -> getString(R.string.vpn_status_starting)
                status == "STOPPING" -> getString(R.string.vpn_status_stopping)
                running -> getString(R.string.vpn_status_running)
                else -> ""
            }
        }
        tile.icon = Icon.createWithResource(this, R.drawable.ic_qs_snirect)
        tile.updateTile()
    }

    companion object {
        private const val TILE_ACTIVITY_REQUEST = 1
    }
}
