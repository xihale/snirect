package com.xihale.snirect.ui.screens

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.widget.Toast
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.BugReport
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.KeyboardArrowDown
import androidx.compose.material.icons.outlined.Layers
import androidx.compose.material.icons.outlined.Share
import androidx.compose.material.icons.outlined.Terminal
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
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
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import com.xihale.snirect.util.AppLogger
import java.util.Date
import java.util.Locale

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LogsScreen(navController: NavController) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val logs = MainActivity.logBuffer
    val listState = rememberLazyListState()

    var searchQuery by remember { mutableStateOf("") }
    var selectedLevel by remember { mutableStateOf<LogLevel?>(null) }
    var showClearDialog by remember { mutableStateOf(false) }

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

    // Counts for filter chips
    val countsByLevel = remember(logCount) {
        logs.groupingBy { it.level }.eachCount()
    }

    val dateFormat = remember { SimpleDateFormat("HH:mm:ss.SSS", Locale.getDefault()) }

    // Follow incoming logs when user is at the bottom
    LaunchedEffect(filteredLogs.size) {
        if (filteredLogs.isEmpty()) return@LaunchedEffect
        val info = listState.layoutInfo
        val lastVisible = info.visibleItemsInfo.lastOrNull()?.index ?: -1
        if (lastVisible >= info.totalItemsCount - 3) {
            listState.scrollToItem(filteredLogs.size - 1)
        }
    }

    // Detect if user has scrolled away from bottom
    val isAtBottom by remember {
        derivedStateOf {
            val info = listState.layoutInfo
            val lastVisible = info.visibleItemsInfo.lastOrNull()?.index ?: -1
            info.totalItemsCount == 0 || lastVisible >= info.totalItemsCount - 2
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
                    "[" + dateFormat.format(Date(entry.timestamp)) + "] [" + entry.level + "] " +
                        AppLogger.sanitize(entry.message)
                }
                val intent = Intent(Intent.ACTION_SEND).apply {
                    type = "text/plain"
                    putExtra(Intent.EXTRA_TEXT, logText)
                }
                context.startActivity(Intent.createChooser(intent, context.getString(R.string.share_logs_chooser)))
            }) {
                Icon(
                    imageVector = Icons.Outlined.Share,
                    contentDescription = stringResource(R.string.action_share_logs),
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            IconButton(onClick = { showClearDialog = true }) {
                Icon(
                    imageVector = Icons.Outlined.Delete,
                    contentDescription = stringResource(R.string.action_clear_logs),
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        },
        header = {
            // 1. M3 Search Field
            AppSearchField(
                query = searchQuery,
                onQueryChange = { searchQuery = it },
                placeholder = stringResource(R.string.search_logs_placeholder)
            )

            // 2. M3 Filter Chips with Icons & Counts
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                LogFilterChip(
                    selected = selectedLevel == null,
                    onClick = { selectedLevel = null },
                    icon = Icons.Outlined.Layers,
                    label = stringResource(R.string.tab_all),
                    count = logs.size
                )
                LogFilterChip(
                    selected = selectedLevel == LogLevel.INFO,
                    onClick = { selectedLevel = LogLevel.INFO },
                    icon = Icons.Outlined.Info,
                    label = "Info",
                    count = countsByLevel[LogLevel.INFO] ?: 0,
                    accentColor = MaterialTheme.colorScheme.primary
                )
                LogFilterChip(
                    selected = selectedLevel == LogLevel.WARN,
                    onClick = { selectedLevel = LogLevel.WARN },
                    icon = Icons.Outlined.WarningAmber,
                    label = "Warn",
                    count = countsByLevel[LogLevel.WARN] ?: 0,
                    accentColor = SnirectWarning
                )
                LogFilterChip(
                    selected = selectedLevel == LogLevel.ERROR,
                    onClick = { selectedLevel = LogLevel.ERROR },
                    icon = Icons.Outlined.ErrorOutline,
                    label = "Error",
                    count = countsByLevel[LogLevel.ERROR] ?: 0,
                    accentColor = MaterialTheme.colorScheme.error
                )
                LogFilterChip(
                    selected = selectedLevel == LogLevel.DEBUG,
                    onClick = { selectedLevel = LogLevel.DEBUG },
                    icon = Icons.Outlined.BugReport,
                    label = "Debug",
                    count = countsByLevel[LogLevel.DEBUG] ?: 0,
                    accentColor = MaterialTheme.colorScheme.tertiary
                )
            }
        },
        floatingActionButton = {
            // Jump to latest FAB with smooth slide/fade
            AnimatedVisibility(
                visible = !isAtBottom && filteredLogs.isNotEmpty(),
                enter = fadeIn(tween(SnirectMotion.SmallDurationMillis)) + scaleIn(tween(SnirectMotion.SmallDurationMillis)),
                exit = fadeOut(tween(SnirectMotion.SmallDurationMillis)) + scaleOut(tween(SnirectMotion.SmallDurationMillis))
            ) {
                ExtendedFloatingActionButton(
                    onClick = {
                        scope.launch {
                            if (filteredLogs.isNotEmpty()) {
                                listState.animateScrollToItem(filteredLogs.size - 1)
                            }
                        }
                    },
                    icon = { Icon(Icons.Outlined.KeyboardArrowDown, contentDescription = null) },
                    text = { Text(stringResource(R.string.action_jump_to_latest)) },
                    containerColor = MaterialTheme.colorScheme.primaryContainer,
                    contentColor = MaterialTheme.colorScheme.onPrimaryContainer,
                    shape = CircleShape,
                    elevation = FloatingActionButtonDefaults.elevation(4.dp)
                )
            }
        }
    ) { padding ->
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
                    top = SnirectSpacing.small,
                    end = SnirectSpacing.marginMobile,
                    bottom = 88.dp
                ),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                items(filteredLogs, key = { it.id }) { entry ->
                    LogItemCard(
                        entry = entry,
                        dateFormat = dateFormat,
                        onCopy = {
                            val formatted = "[" + dateFormat.format(Date(entry.timestamp)) + "] [" + entry.level + "] " + entry.message
                            val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
                            clipboard.setPrimaryClip(ClipData.newPlainText("Snirect Log", formatted))
                            Toast.makeText(context, context.getString(R.string.toast_log_copied), Toast.LENGTH_SHORT).show()
                        }
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

    // Clear Logs Confirmation Dialog
    if (showClearDialog) {
        AlertDialog(
            onDismissRequest = { showClearDialog = false },
            icon = {
                Icon(
                    Icons.Outlined.Delete,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.error
                )
            },
            title = { Text(stringResource(R.string.dialog_clear_logs_title)) },
            text = { Text(stringResource(R.string.dialog_clear_logs_msg)) },
            confirmButton = {
                TextButton(
                    onClick = {
                        logs.clear()
                        showClearDialog = false
                    },
                    colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error)
                ) {
                    Text(stringResource(R.string.action_clear_logs))
                }
            },
            dismissButton = {
                TextButton(onClick = { showClearDialog = false }) {
                    Text(stringResource(R.string.action_cancel))
                }
            }
        )
    }
}

@Composable
private fun LogFilterChip(
    selected: Boolean,
    onClick: () -> Unit,
    icon: ImageVector,
    label: String,
    count: Int,
    accentColor: Color? = null
) {
    val tintColor = if (selected) {
        MaterialTheme.colorScheme.onSecondaryContainer
    } else {
        accentColor ?: MaterialTheme.colorScheme.onSurfaceVariant
    }

    FilterChip(
        selected = selected,
        onClick = onClick,
        leadingIcon = {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = tintColor,
                modifier = Modifier.size(16.dp)
            )
        },
        label = {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Text(label, fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal)
                if (count > 0) {
                    Text(
                        text = "(" + count + ")",
                        style = MaterialTheme.typography.labelSmall,
                        color = if (selected) MaterialTheme.colorScheme.onSecondaryContainer else MaterialTheme.colorScheme.outline
                    )
                }
            }
        },
        shape = RoundedCornerShape(8.dp),
        colors = FilterChipDefaults.filterChipColors(
            selectedContainerColor = MaterialTheme.colorScheme.secondaryContainer,
            selectedLabelColor = MaterialTheme.colorScheme.onSecondaryContainer
        )
    )
}

@Composable
private fun LogItemCard(
    entry: LogEntry,
    dateFormat: SimpleDateFormat,
    onCopy: () -> Unit,
    modifier: Modifier = Modifier
) {
    val isDark = isSystemInDarkTheme()

    // Parse component module tag if present (e.g. "[dns]", "[upstream]", "[rules]", "[system]")
    val parsed = remember(entry.message) {
        parseLogMessage(entry.message)
    }

    val (levelBg, levelFg, levelIcon) = when (entry.level) {
        LogLevel.ERROR -> Triple(
            MaterialTheme.colorScheme.errorContainer,
            MaterialTheme.colorScheme.onErrorContainer,
            Icons.Outlined.ErrorOutline
        )
        LogLevel.WARN -> Triple(
            if (isDark) SnirectWarning.copy(alpha = 0.22f) else SnirectWarningContainer,
            if (isDark) SnirectWarning else SnirectOnWarning,
            Icons.Outlined.WarningAmber
        )
        LogLevel.DEBUG -> Triple(
            MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.5f),
            MaterialTheme.colorScheme.onTertiaryContainer,
            Icons.Outlined.BugReport
        )
        LogLevel.INFO -> Triple(
            MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.45f),
            MaterialTheme.colorScheme.onPrimaryContainer,
            Icons.Outlined.Info
        )
    }

    Surface(
        modifier = modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .clickable(onClick = onCopy),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceContainerLow,
        border = BorderStroke(
            1.dp,
            if (entry.level == LogLevel.ERROR) MaterialTheme.colorScheme.error.copy(alpha = 0.25f)
            else MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.4f)
        )
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 10.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            // Header Row: Level Badge + Module Pill + Timestamp + Copy Action
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    // Level Pill
                    Surface(
                        shape = RoundedCornerShape(6.dp),
                        color = levelBg
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(3.dp)
                        ) {
                            Icon(
                                imageVector = levelIcon,
                                contentDescription = null,
                                tint = levelFg,
                                modifier = Modifier.size(12.dp)
                            )
                            Text(
                                text = entry.level.name.uppercase(),
                                style = MaterialTheme.typography.labelSmall.copy(
                                    fontSize = 11.sp,
                                    fontWeight = FontWeight.Bold
                                ),
                                color = levelFg
                            )
                        }
                    }

                    // Optional Module Tag (e.g. "dns", "upstream", "system")
                    if (parsed.module != null) {
                        Surface(
                            shape = RoundedCornerShape(6.dp),
                            color = MaterialTheme.colorScheme.surfaceContainerHigh
                        ) {
                            Text(
                                text = parsed.module,
                                style = MaterialTheme.typography.labelSmall.copy(
                                    fontSize = 11.sp,
                                    fontWeight = FontWeight.Medium
                                ),
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                            )
                        }
                    }
                }

                // Timestamp + Copy icon
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Text(
                        text = dateFormat.format(Date(entry.timestamp)),
                        style = MaterialTheme.typography.labelSmall.copy(
                            fontFamily = FontFamily.Monospace,
                            fontSize = 11.sp
                        ),
                        color = MaterialTheme.colorScheme.outline
                    )
                    Icon(
                        imageVector = Icons.Outlined.ContentCopy,
                        contentDescription = stringResource(R.string.toast_log_copied),
                        tint = MaterialTheme.colorScheme.outline.copy(alpha = 0.7f),
                        modifier = Modifier.size(13.dp)
                    )
                }
            }

            // Body: Syntax-highlighted log message
            Text(
                text = formatLogMessage(parsed.body, isDark),
                style = MaterialTheme.typography.bodyMedium.copy(
                    fontFamily = FontFamily.Monospace,
                    fontSize = 12.5.sp,
                    lineHeight = 18.sp
                ),
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

private data class ParsedLog(val module: String?, val body: String)

/**
 * Extracts optional module tags like "[dns] query..." or "[upstream] connected..."
 */
private fun parseLogMessage(raw: String): ParsedLog {
    val trimmed = raw.trim()
    val moduleRegex = Regex("^\\[([a-zA-Z0-9_-]+)\\]\\s*(.*)$")
    val match = moduleRegex.find(trimmed)
    return if (match != null) {
        ParsedLog(module = match.groupValues[1], body = match.groupValues[2])
    } else {
        ParsedLog(module = null, body = trimmed)
    }
}

/**
 * Formats key=value tokens (e.g. error="...", addr="127.0.0.1:9") with subtle MD3 syntax tinting.
 */
@Composable
private fun formatLogMessage(text: String, isDark: Boolean): androidx.compose.ui.text.AnnotatedString {
    val keyColor = MaterialTheme.colorScheme.secondary
    val valueColor = if (isDark) Color(0xFF80CBC4) else Color(0xFF00796B)
    val errorColor = MaterialTheme.colorScheme.error

    return buildAnnotatedString {
        val kvRegex = Regex("([a-zA-Z_]+)=([^\\s]+)")
        var lastIndex = 0

        for (match in kvRegex.findAll(text)) {
            val range = match.range
            if (range.first > lastIndex) {
                append(text.substring(lastIndex, range.first))
            }

            val key = match.groupValues[1]
            val value = match.groupValues[2]

            withStyle(SpanStyle(color = if (key.contains("err", ignoreCase = true)) errorColor else keyColor, fontWeight = FontWeight.SemiBold)) {
                append(key + "=")
            }
            withStyle(SpanStyle(color = if (key.contains("err", ignoreCase = true)) errorColor else valueColor)) {
                append(value)
            }

            lastIndex = range.last + 1
        }

        if (lastIndex < text.length) {
            append(text.substring(lastIndex))
        }
    }
}
