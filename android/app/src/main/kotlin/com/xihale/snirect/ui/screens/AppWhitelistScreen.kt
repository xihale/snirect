package com.xihale.snirect.ui.screens

import android.Manifest
import android.content.pm.PackageManager
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.Crossfade
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.VisibilityThreshold
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Apps
import androidx.compose.material.icons.outlined.FilterList
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.navigation.NavController
import com.xihale.snirect.R
import com.xihale.snirect.data.repository.ConfigRepository
import com.xihale.snirect.ui.components.AppEmptyState
import com.xihale.snirect.ui.components.AppScreenScaffold
import com.xihale.snirect.ui.components.AppSearchField
import com.xihale.snirect.ui.components.AppTopBarStyle
import com.xihale.snirect.ui.theme.SnirectMotion
import com.xihale.snirect.ui.theme.SnirectSpacing
import kotlinx.coroutines.launch

@Immutable
data class AppItem(
    val packageName: String,
    val label: String,
    val isSystem: Boolean,
    val icon: ImageBitmap? = null
)

/** Quick category filters for the app list. */
private enum class AppCategoryFilter { All, Browsers, Selected }

@Composable
fun AppWhitelistScreen(
    navController: NavController,
    repository: ConfigRepository
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    var searchQuery by remember { mutableStateOf("") }
    var showSystemApps by remember { mutableStateOf(false) }
    var sortBySelectedFirst by remember { mutableStateOf(false) }
    var categoryFilter by remember { mutableStateOf(AppCategoryFilter.All) }
    var whitelistPackages by remember { mutableStateOf(setOf<String>()) }
    val cached = InstalledAppsCache.peek()
    var allApps by remember { mutableStateOf(cached?.apps ?: emptyList()) }
    var browserPackages by remember { mutableStateOf(cached?.browserPackages ?: emptySet()) }
    var isLoading by remember { mutableStateOf(cached == null) }

    LaunchedEffect(Unit) {
        repository.whitelistPackages.collect { whitelistPackages = it }
    }

    // Permission first, list second: QUERY_ALL_PACKAGES is an install-time
    // (normal) permission — no runtime dialog exists for it. If it is somehow
    // missing we show a notice instead of silently loading a partial list.
    var canQueryPackages by remember {
        mutableStateOf(
            ContextCompat.checkSelfPermission(
                context, Manifest.permission.QUERY_ALL_PACKAGES
            ) == PackageManager.PERMISSION_GRANTED
        )
    }

    // Re-check on every ON_RESUME: some ROMs expose QUERY_ALL_PACKAGES as a
    // toggle in system settings. Without this the screen would keep showing
    // the no-permission state after the user grants it there, until they
    // manually left and re-entered this screen.
    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event != Lifecycle.Event.ON_RESUME) return@LifecycleEventObserver
            canQueryPackages = ContextCompat.checkSelfPermission(
                context, Manifest.permission.QUERY_ALL_PACKAGES
            ) == PackageManager.PERMISSION_GRANTED
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    LaunchedEffect(canQueryPackages) {
        if (!canQueryPackages) {
            allApps = emptyList()
            isLoading = false
            return@LaunchedEffect
        }
        val snap = InstalledAppsCache.load(context)
        allApps = snap.apps
        browserPackages = snap.browserPackages
        isLoading = false
    }

    val filteredApps = remember(searchQuery, showSystemApps, sortBySelectedFirst, categoryFilter, browserPackages, allApps, whitelistPackages) {
        val list = allApps.asSequence()
            .filter { if (!showSystemApps) !it.isSystem else true }
            .filter {
                when (categoryFilter) {
                    AppCategoryFilter.Browsers -> it.packageName in browserPackages
                    AppCategoryFilter.Selected -> it.packageName in whitelistPackages
                    AppCategoryFilter.All -> true
                }
            }
            .filter {
                if (searchQuery.isBlank()) {
                    true
                } else {
                    it.label.contains(searchQuery, ignoreCase = true) ||
                        it.packageName.contains(searchQuery, ignoreCase = true)
                }
            }
            .toList()

        if (sortBySelectedFirst) {
            list.sortedWith(
                compareByDescending<AppItem> { whitelistPackages.contains(it.packageName) }
                    .thenBy { it.label.lowercase() }
            )
        } else {
            list
        }
    }

    AppScreenScaffold(
        title = stringResource(R.string.dialog_select_apps),
        onBack = { navController.popBackStack() },
        backContentDescription = stringResource(R.string.action_back),
        topBarStyle = AppTopBarStyle.Top,
        header = {
            // Header Search & Counter
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Box(modifier = Modifier.weight(1f)) {
                    AppSearchField(
                        query = searchQuery,
                        onQueryChange = { searchQuery = it },
                        placeholder = stringResource(R.string.search_apps_placeholder)
                    )
                }

                Column(
                    horizontalAlignment = Alignment.End,
                    verticalArrangement = Arrangement.Center
                ) {
                    Text(
                        text = whitelistPackages.size.toString(),
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.primary
                    )
                    Text(
                        text = stringResource(R.string.setting_whitelist_apps),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            // Quick category filters
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                FilterChip(
                    selected = categoryFilter == AppCategoryFilter.All,
                    onClick = { categoryFilter = AppCategoryFilter.All },
                    label = { Text(stringResource(R.string.tab_all)) }
                )
                FilterChip(
                    selected = categoryFilter == AppCategoryFilter.Browsers,
                    onClick = { categoryFilter = AppCategoryFilter.Browsers },
                    label = { Text(stringResource(R.string.filter_browsers)) }
                )
                FilterChip(
                    selected = categoryFilter == AppCategoryFilter.Selected,
                    onClick = { categoryFilter = AppCategoryFilter.Selected },
                    label = { Text(stringResource(R.string.filter_selected_apps)) }
                )
            }

            // Controls Bar: System apps switch & Sort button
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 4.dp, vertical = 2.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                    // clip before clickable so the ripple stays pill-shaped
                    // instead of a hard rectangle.
                    modifier = Modifier
                        .clip(CircleShape)
                        .clickable { showSystemApps = !showSystemApps }
                ) {
                    Switch(
                        checked = showSystemApps,
                        onCheckedChange = { showSystemApps = it },
                        modifier = Modifier.scale(0.85f)
                    )
                    Text(
                        text = stringResource(R.string.show_system_apps),
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }

                TextButton(
                    onClick = { sortBySelectedFirst = !sortBySelectedFirst },
                    contentPadding = PaddingValues(horizontal = 12.dp, vertical = 4.dp)
                ) {
                    Icon(
                        imageVector = Icons.Outlined.FilterList,
                        contentDescription = null,
                        modifier = Modifier.size(18.dp)
                    )
                    Spacer(modifier = Modifier.width(6.dp))
                    Text(
                        text = if (sortBySelectedFirst) stringResource(R.string.sort_selected_first) else stringResource(R.string.sort_alphabetical),
                        style = MaterialTheme.typography.labelMedium
                    )
                }
            }
        }
    ) { padding ->
        val listState = when {
            isLoading -> AppListState.Loading
            !canQueryPackages -> AppListState.NoPermission
            else -> AppListState.Ready
        }

        Crossfade(
            targetState = listState,
            animationSpec = tween(SnirectMotion.MediumDurationMillis),
            label = "appListState"
        ) { state ->
            when (state) {
                AppListState.Loading -> {
                    Box(
                        modifier = Modifier
                            .fillMaxSize()
                            .padding(padding),
                        contentAlignment = Alignment.Center
                    ) {
                        CircularProgressIndicator(color = MaterialTheme.colorScheme.primary)
                    }
                }

                AppListState.NoPermission -> {
                    Box(
                        modifier = Modifier
                            .fillMaxSize()
                            .padding(padding),
                        contentAlignment = Alignment.Center
                    ) {
                        AppEmptyState(
                            icon = Icons.Outlined.Apps,
                            title = stringResource(R.string.app_list_no_permission_title),
                            supportingText = stringResource(R.string.app_list_no_permission_hint)
                        )
                    }
                }

                AppListState.Ready -> Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding)
                ) {
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        contentPadding = PaddingValues(
                            start = SnirectSpacing.marginMobile,
                            top = SnirectSpacing.gutter,
                            end = SnirectSpacing.marginMobile,
                            bottom = SnirectSpacing.xxLarge
                        ),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        items(filteredApps, key = { it.packageName }) { app ->
                            val isChecked = whitelistPackages.contains(app.packageName)
                            AppSelectionItem(
                                app = app,
                                isChecked = isChecked,
                                onToggle = {
                                    val newSet = if (isChecked) {
                                        whitelistPackages - app.packageName
                                    } else {
                                        whitelistPackages + app.packageName
                                    }
                                    whitelistPackages = newSet
                                    scope.launch { repository.setWhitelistPackages(newSet) }
                                },
                                // Snappier than the default placement spring
                                // (StiffnessMediumLow): reorder/search moves
                                // settle in ~150ms instead of dawdling.
                                modifier = Modifier.animateItem(
                                    placementSpec = spring(
                                        dampingRatio = Spring.DampingRatioNoBouncy,
                                        stiffness = Spring.StiffnessMedium,
                                        visibilityThreshold = IntOffset.VisibilityThreshold
                                    )
                                )
                            )
                        }
                    }

                    // The empty state overlays the always-composed list, so
                    // empty↔non-empty during search doesn't tear the list
                    // down, reset scroll and rebuild it on every keystroke.
                    AnimatedVisibility(
                        visible = filteredApps.isEmpty(),
                        enter = fadeIn(tween(SnirectMotion.MediumDurationMillis)),
                        exit = fadeOut(tween(SnirectMotion.ExitDurationMillis))
                    ) {
                        Box(
                            modifier = Modifier.fillMaxSize(),
                            contentAlignment = Alignment.Center
                        ) {
                            AppEmptyState(
                                icon = Icons.Outlined.Apps,
                                title = stringResource(R.string.no_apps_found),
                                supportingText = stringResource(R.string.search_apps_placeholder)
                            )
                        }
                    }
                }
            }
        }
    }
}

private enum class AppListState { Loading, NoPermission, Ready }

@Composable
private fun AppSelectionItem(
    app: AppItem,
    isChecked: Boolean,
    onToggle: () -> Unit,
    modifier: Modifier = Modifier
) {
    // Row tint is the only custom bit; the check itself is the platform Checkbox.
    val rowColor by animateColorAsState(
        targetValue = if (isChecked) {
            MaterialTheme.colorScheme.primary.copy(alpha = 0.08f)
        } else {
            MaterialTheme.colorScheme.surfaceContainer
        },
        animationSpec = tween(SnirectMotion.SmallDurationMillis),
        label = "appRowColor"
    )

    // onClick on the Surface itself: a clickable on the passed-in modifier
    // draws its ripple OUTSIDE the surface's shape clip — a square highlight
    // sticking out past the rounded card. Surface(onClick) clips it.
    Surface(
        onClick = onToggle,
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = rowColor
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            // App Icon Container
            Surface(
                shape = RoundedCornerShape(12.dp),
                color = MaterialTheme.colorScheme.surfaceContainerHigh,
                modifier = Modifier.size(44.dp)
            ) {
                Box(contentAlignment = Alignment.Center) {
                    if (app.icon != null) {
                        Image(
                            bitmap = app.icon,
                            contentDescription = app.label,
                            modifier = Modifier.size(32.dp)
                        )
                    } else {
                        Icon(
                            imageVector = Icons.Outlined.Apps,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.secondary,
                            modifier = Modifier.size(24.dp)
                        )
                    }
                }
            }

            // Labels
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(2.dp)
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    Text(
                        text = app.label,
                        style = MaterialTheme.typography.bodyLarge,
                        fontWeight = FontWeight.Medium,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    if (app.isSystem) {
                        Surface(
                            shape = RoundedCornerShape(6.dp),
                            color = MaterialTheme.colorScheme.surfaceContainerHighest
                        ) {
                            Text(
                                text = stringResource(R.string.label_system_short),
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(horizontal = 4.dp, vertical = 1.dp)
                            )
                        }
                    }
                }

                Text(
                    text = app.packageName,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }

            Checkbox(
                checked = isChecked,
                onCheckedChange = null
            )
        }
    }
}
