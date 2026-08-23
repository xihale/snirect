package com.xihale.snirect.ui.screens

import android.content.Intent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Share
import androidx.compose.material.icons.outlined.Terminal
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.navigation.NavController
import com.xihale.snirect.MainActivity
import com.xihale.snirect.R
import com.xihale.snirect.data.model.LogEntry
import com.xihale.snirect.data.model.LogLevel
import com.xihale.snirect.ui.components.AppEmptyState
import com.xihale.snirect.ui.components.AppScreenScaffold
import com.xihale.snirect.ui.components.AppSearchField
import com.xihale.snirect.ui.components.AppTopBarStyle
import com.xihale.snirect.ui.theme.SnirectMotion
import com.xihale.snirect.ui.theme.SnirectOnWarning
import com.xihale.snirect.ui.theme.SnirectSpacing
import com.xihale.snirect.ui.theme.SnirectWarning
import com.xihale.snirect.ui.theme.SnirectWarningContainer
import java.text.SimpleDateFormat
import com.xihale.snirect.util.AppLogger
import java.util.Date
import java.util.Locale

@Composable
fun LogsScreen(navController: NavController) {
    val context = LocalContext.current
    val logs = MainActivity.logBuffer
    val listState = rememberLazyListState()

    var searchQuery by remember { mutableStateOf("") }
    var selectedLevel by remember { mutableStateOf<LogLevel?>(null) }

    // The unconditional size read keeps this recomposing per append; the
    // remember then skips the O(n) filter copy entirely for the default
    // "All" view (the common case while logs stream in).
    val logCount = logs.size
    val filteredLogs: List<LogEntry> = remember(logCount, searchQuery, selectedLevel) {
        if (selectedLevel == null && searchQuery.isEmpty()) {
            logs
        } else {
            logs.filter { entry ->
                (selectedLevel == null || entry.level == selectedLevel) &&
                    (searchQuery.isEmpty() || entry.message.contains(searchQuery, ignoreCase = true))
            }
        }
    }

    val dateFormat = remember { SimpleDateFormat("HH:mm:ss.SSS", Locale.getDefault()) }

    LaunchedEffect(filteredLogs.size) {
        if (filteredLogs.isEmpty()) return@LaunchedEffect
        // Only follow incoming logs when the user is already parked at the
        // bottom; never yank the view while they are reading history.
        val info = listState.layoutInfo
        val lastVisible = info.visibleItemsInfo.lastOrNull()?.index ?: -1
        if (lastVisible >= info.totalItemsCount - 3) {
            // Snap instead of animating: an animated follow per appended line
            // fights the list's own scroll and drops frames while streaming.
            listState.scrollToItem(filteredLogs.size - 1)
        }
    }

    AppScreenScaffold(
        title = stringResource(R.string.logs_title),
        onBack = { navController.popBackStack() },
        backContentDescription = stringResource(R.string.action_back),
        topBarStyle = AppTopBarStyle.Top,
        actions = {
            IconButton(onClick = {
                // Share is the one channel where logs leave the device, so the
                // in-memory buffer (kept raw for debugging) is scrubbed here.
                val logText = filteredLogs.joinToString("\n") { entry ->
                    "[${dateFormat.format(Date(entry.timestamp))}] [${entry.level}] ${AppLogger.sanitize(entry.message)}"
                }
                val intent = Intent(Intent.ACTION_SEND).apply {
                    type = "text/plain"
                    putExtra(Intent.EXTRA_TEXT, logText)
                }
                context.startActivity(Intent.createChooser(intent, context.getString(R.string.share_logs_chooser)))
            }) {
                Icon(
                    imageVector = Icons.Default.Share,
                    contentDescription = stringResource(R.string.action_share_logs),
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            IconButton(onClick = { logs.clear() }) {
                Icon(
                    imageVector = Icons.Default.Delete,
                    contentDescription = stringResource(R.string.action_clear_logs),
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        },
        header = {
            // Search Input
            AppSearchField(
                query = searchQuery,
                onQueryChange = { searchQuery = it },
                placeholder = stringResource(R.string.search_logs_placeholder)
            )

            // Severity Tab Chips
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                FilterChip(
                    selected = selectedLevel == null,
                    onClick = { selectedLevel = null },
                    label = { Text(stringResource(R.string.tab_all)) }
                )
                FilterChip(
                    selected = selectedLevel == LogLevel.INFO,
                    onClick = { selectedLevel = LogLevel.INFO },
                    label = { Text("Info") }
                )
                FilterChip(
                    selected = selectedLevel == LogLevel.WARN,
                    onClick = { selectedLevel = LogLevel.WARN },
                    label = { Text("Warn") }
                )
                FilterChip(
                    selected = selectedLevel == LogLevel.ERROR,
                    onClick = { selectedLevel = LogLevel.ERROR },
                    label = { Text("Error") }
                )
                FilterChip(
                    selected = selectedLevel == LogLevel.DEBUG,
                    onClick = { selectedLevel = LogLevel.DEBUG },
                    label = { Text("Debug") }
                )
            }
        }
    ) { padding ->
        // The list stays composed at all times; the empty state overlays it.
        // A Crossfade here used to dispose and rebuild the LazyColumn every
        // time a level filter or search made the list empty — losing scroll
        // position and visibly hitching.
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
        ) {
            LazyColumn(
                state = listState,
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(
                    start = SnirectSpacing.marginMobile,
                    top = SnirectSpacing.medium,
                    end = SnirectSpacing.marginMobile,
                    bottom = SnirectSpacing.xxLarge
                ),
                verticalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                items(filteredLogs, key = { it.id }) { entry ->
                    // Deliberately no animateItem(): this list only ever
                    // appends (placement never changes), and the placement
                    // springs visibly glitch — overlapping cards — when a
                    // level filter removes hundreds of items at once.
                    LogItemCard(
                        entry = entry,
                        dateFormat = dateFormat
                    )
                }
            }

            AnimatedVisibility(
                visible = filteredLogs.isEmpty(),
                enter = fadeIn(tween(SnirectMotion.MediumDurationMillis)),
                exit = fadeOut(tween(SnirectMotion.ExitDurationMillis))
            ) {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center
                ) {
                    AppEmptyState(
                        icon = Icons.Outlined.Terminal,
                        title = stringResource(R.string.no_logs_found),
                        supportingText = stringResource(R.string.search_logs_placeholder)
                    )
                }
            }
        }
    }
}

@Composable
private fun LogItemCard(
    entry: LogEntry,
    dateFormat: SimpleDateFormat,
    modifier: Modifier = Modifier
) {
    val isDark = isSystemInDarkTheme()

    val barColor = when (entry.level) {
        LogLevel.ERROR -> MaterialTheme.colorScheme.error
        LogLevel.WARN -> SnirectWarning
        LogLevel.DEBUG -> MaterialTheme.colorScheme.tertiary
        LogLevel.INFO -> MaterialTheme.colorScheme.primary
    }

    val tagBg = when (entry.level) {
        LogLevel.ERROR -> MaterialTheme.colorScheme.errorContainer
        // Amber on amber fails contrast in light mode; each theme gets a
        // tuned pair.
        LogLevel.WARN -> if (isDark) SnirectWarning.copy(alpha = 0.18f) else SnirectWarningContainer
        LogLevel.DEBUG -> MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.4f)
        LogLevel.INFO -> MaterialTheme.colorScheme.primary.copy(alpha = 0.12f)
    }

    val tagFg = when (entry.level) {
        LogLevel.ERROR -> MaterialTheme.colorScheme.onErrorContainer
        LogLevel.WARN -> if (isDark) SnirectWarning else SnirectOnWarning
        LogLevel.DEBUG -> MaterialTheme.colorScheme.tertiary
        LogLevel.INFO -> MaterialTheme.colorScheme.primary
    }

    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.surfaceContainerHighest
    ) {
        // The accent stripe is drawn behind the content instead of the old
        // Row(height(IntrinsicSize.Min)) layout — intrinsic measurement ran
        // every item through layout twice, which showed up as scroll jank.
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .drawBehind {
                    drawRect(color = barColor, size = Size(4.dp.toPx(), size.height))
                }
                .padding(10.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            // Header: Level Badge & Timestamp
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Surface(
                    shape = RoundedCornerShape(6.dp),
                    color = tagBg
                ) {
                    Text(
                        text = entry.level.name.uppercase(),
                        style = MaterialTheme.typography.labelSmall,
                        color = tagFg,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                    )
                }

                Text(
                    text = dateFormat.format(Date(entry.timestamp)),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.outline
                )
            }

            // Log Message Content
            Text(
                text = entry.message,
                style = MaterialTheme.typography.bodyMedium.copy(
                    fontFamily = FontFamily.Monospace,
                    fontSize = 13.sp,
                    lineHeight = 18.sp
                ),
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}
