package com.xihale.snirect.ui.components

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import com.xihale.snirect.BuildConfig
import com.xihale.snirect.R
import com.xihale.snirect.ktlib.AppUpdate
import com.xihale.snirect.ktlib.SnirectClient
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

const val SOURCE_URL = "https://github.com/xihale/snirect"
const val RELEASES_URL = "https://github.com/xihale/snirect/releases"

/** Queries GitHub Releases via the Go core. Runs on IO; safe to call from UI scopes. */
suspend fun checkForUpdate(context: Context): AppUpdate = withContext(Dispatchers.IO) {
    SnirectClient.from(context).checkUpdate(BuildConfig.VERSION_NAME)
}

fun openInBrowser(context: Context, url: String) {
    context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)))
}

/**
 * Result dialogs for an update check, shared by the manual check in
 * Settings → About and the silent auto-check on Home. Hosts keep their own
 * `AppUpdate?` / `String?` state and clear it via the callbacks; pass null
 * to hide a dialog (the auto-check never surfaces errors, only new versions).
 */
@Composable
fun UpdateCheckDialogs(
    updateResult: AppUpdate?,
    updateError: String?,
    onClearResult: () -> Unit,
    onClearError: () -> Unit
) {
    val context = LocalContext.current

    updateResult?.let { info ->
        val openUrl = info.url.ifBlank { RELEASES_URL }
        AlertDialog(
            onDismissRequest = onClearResult,
            title = {
                Text(
                    stringResource(
                        if (info.newer) R.string.update_available_title else R.string.update_current_title
                    )
                )
            },
            text = {
                Text(
                    if (info.newer) {
                        stringResource(R.string.update_available_msg, info.current, info.latest)
                    } else {
                        stringResource(R.string.update_current_msg, info.latest)
                    }
                )
            },
            confirmButton = {
                if (info.newer) {
                    Button(
                        onClick = {
                            onClearResult()
                            openInBrowser(context, openUrl)
                        }
                    ) {
                        Text(stringResource(R.string.action_open_release))
                    }
                } else {
                    TextButton(onClick = onClearResult) {
                        Text(stringResource(R.string.action_later))
                    }
                }
            },
            dismissButton = if (info.newer) {
                {
                    TextButton(onClick = onClearResult) {
                        Text(stringResource(R.string.action_later))
                    }
                }
            } else {
                null
            }
        )
    }
    updateError?.let { err ->
        AlertDialog(
            onDismissRequest = onClearError,
            title = { Text(stringResource(R.string.update_failed_title)) },
            text = { Text(stringResource(R.string.update_failed_msg, err)) },
            confirmButton = {
                Button(
                    onClick = {
                        onClearError()
                        openInBrowser(context, RELEASES_URL)
                    }
                ) {
                    Text(stringResource(R.string.action_open_release))
                }
            },
            dismissButton = {
                TextButton(onClick = onClearError) {
                    Text(stringResource(R.string.action_later))
                }
            }
        )
    }
}
