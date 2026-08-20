package com.xihale.snirect.ui.screens

import android.content.Context
import android.content.Intent
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import android.net.Uri
import androidx.compose.ui.graphics.asImageBitmap
import androidx.core.graphics.drawable.toBitmap
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext

/**
 * Process-lifetime cache of the installed-app list. Decoding every icon on
 * each visit to the whitelist screen is the slow path the user hits; keep
 * the snapshot until the process dies (or [invalidate] is called).
 */
object InstalledAppsCache {
    data class Snapshot(
        val apps: List<AppItem>,
        val browserPackages: Set<String>,
    )

    private val mutex = Mutex()
    @Volatile private var snapshot: Snapshot? = null

    fun peek(): Snapshot? = snapshot

    fun invalidate() {
        snapshot = null
    }

    suspend fun load(context: Context): Snapshot = mutex.withLock {
        snapshot?.let { return it }
        val result = withContext(Dispatchers.IO) { query(context) }
        snapshot = result
        result
    }

    private fun query(context: Context): Snapshot {
        val pm = context.packageManager
        val browserIntent = Intent(Intent.ACTION_VIEW, Uri.parse("https://example.com"))
            .addCategory(Intent.CATEGORY_BROWSABLE)
        val browsers = pm.queryIntentActivities(browserIntent, PackageManager.MATCH_ALL)
            .mapNotNull { it.activityInfo?.packageName }
            .toSet()

        val apps = pm.getInstalledApplications(PackageManager.GET_META_DATA)
            .map { info ->
                AppItem(
                    packageName = info.packageName,
                    label = pm.getApplicationLabel(info).toString(),
                    isSystem = (info.flags and ApplicationInfo.FLAG_SYSTEM) != 0,
                    icon = try {
                        pm.getApplicationIcon(info).toBitmap(96, 96).asImageBitmap()
                    } catch (_: Exception) {
                        null
                    }
                )
            }
            .sortedBy { it.label.lowercase() }

        return Snapshot(apps = apps, browserPackages = browsers)
    }
}
