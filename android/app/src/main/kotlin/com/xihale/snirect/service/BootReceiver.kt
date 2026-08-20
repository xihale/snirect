package com.xihale.snirect.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.widget.Toast
import com.xihale.snirect.data.repository.ConfigRepository
import com.xihale.snirect.ktlib.SnirectClient
import com.xihale.snirect.util.AppLogger
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

import com.xihale.snirect.R

class BootReceiver : BroadcastReceiver() {

    companion object {
        // Not exposed as an SDK constant on all API levels; matches the
        // action registered in AndroidManifest.xml.
        private const val ACTION_QUICKBOOT_POWERON = "android.intent.action.QUICKBOOT_POWERON"
    }

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED &&
            intent.action != ACTION_QUICKBOOT_POWERON
        ) {
            return
        }
        AppLogger.i("BootReceiver: Received ${intent.action}")
        AppLogger.init(context.applicationContext)
        val snirectClient = SnirectClient.from(context)
        val repository = ConfigRepository(context)

        // goAsync() keeps the receiver alive while the cert check (suspend IO:
        // possible RSA keygen on first run) runs off the critical path.
        val pendingResult = goAsync()
        CoroutineScope(SupervisorJob() + Dispatchers.Main).launch {
            try {
                val shouldActivate = repository.activateOnBoot.first()
                if (shouldActivate) {
                    val skipCheck = repository.skipCertCheck.first()
                    if (!skipCheck && !snirectClient.isCaCertificateInstalled()) {
                        AppLogger.w("BootReceiver: Auto-start blocked - CA certificate not installed")
                        Toast.makeText(context, context.getString(R.string.toast_boot_auto_start_blocked), Toast.LENGTH_LONG).show()
                        return@launch
                    }

                    AppLogger.i("BootReceiver: Auto-starting VPN service")
                    val vpnIntent = Intent(context, SnirectVpnService::class.java).apply {
                        action = SnirectVpnService.ACTION_START
                    }
                    // A running broadcast receiver keeps the app active, so a
                    // plain startService is permitted here.
                    context.startService(vpnIntent)
                } else {
                    AppLogger.i("BootReceiver: Auto-start on boot is disabled")
                }
            } finally {
                pendingResult.finish()
            }
        }
    }
}
