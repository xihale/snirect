package com.xihale.snirect.ui.screens

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.togetherWith
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.GppBad
import androidx.compose.material.icons.outlined.PowerSettingsNew
import androidx.compose.material.icons.outlined.Terminal
import androidx.compose.material.icons.outlined.VerifiedUser
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.xihale.snirect.BuildConfig
import com.xihale.snirect.MainViewModel
import com.xihale.snirect.R
import com.xihale.snirect.data.repository.ConfigRepository
import com.xihale.snirect.ktlib.AppUpdate
import com.xihale.snirect.ktlib.SnirectClient
import com.xihale.snirect.ui.theme.SnirectMotion
import com.xihale.snirect.ui.theme.SnirectSpacing
import com.xihale.snirect.ui.theme.pressScale
import com.xihale.snirect.util.AppLogger
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(
    navController: NavController,
    viewModel: MainViewModel,
    repository: ConfigRepository
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()

    // Cached on the activity ViewModel so popping back to Home does not
    // walk AndroidCAStore again (or flash the "checking" spinner).
    val isCertInstalled = viewModel.isCertInstalled
    var showCertPrompt by remember { mutableStateOf(false) }
    var didPromptCert by rememberSaveable { mutableStateOf(false) }
    var didAttemptAutoStart by rememberSaveable { mutableStateOf(false) }
    var checkingUpdate by remember { mutableStateOf(false) }
    var updateResult by remember { mutableStateOf<AppUpdate?>(null) }
    var updateError by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) { viewModel.ensureCertStatus() }
    LaunchedEffect(isCertInstalled) {
        if (didPromptCert || isCertInstalled != false) return@LaunchedEffect
        val skipCheck = repository.skipCertCheck.first()
        if (!skipCheck) {
            didPromptCert = true
            showCertPrompt = true
        }
    }

    if (showCertPrompt) {
        AlertDialog(
            onDismissRequest = { showCertPrompt = false },
            icon = {
                Icon(
                    Icons.Outlined.GppBad,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.error
                )
            },
            title = { Text(stringResource(R.string.cert_required_title)) },
            text = { Text(stringResource(R.string.cert_required_msg)) },
            confirmButton = {
                Button(
                    onClick = {
                        showCertPrompt = false
                        navController.navigate("help")
                    }
                ) {
                    Text(stringResource(R.string.action_view_guide))
                }
            },
            dismissButton = {
                TextButton(onClick = { showCertPrompt = false }) {
                    Text(stringResource(R.string.action_later))
                }
            }
        )
    }

    val fallbackReleaseUrl = "https://github.com/xihale/snirect/releases"
    updateResult?.let { info ->
        val openUrl = info.url.ifBlank { fallbackReleaseUrl }
        AlertDialog(
            onDismissRequest = { updateResult = null },
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
                            updateResult = null
                            context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(openUrl)))
                        }
                    ) {
                        Text(stringResource(R.string.action_open_release))
                    }
                } else {
                    TextButton(onClick = { updateResult = null }) {
                        Text(stringResource(R.string.action_later))
                    }
                }
            },
            dismissButton = if (info.newer) {
                {
                    TextButton(onClick = { updateResult = null }) {
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
            onDismissRequest = { updateError = null },
            title = { Text(stringResource(R.string.update_failed_title)) },
            text = { Text(stringResource(R.string.update_failed_msg, err)) },
            confirmButton = {
                Button(
                    onClick = {
                        updateError = null
                        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(fallbackReleaseUrl)))
                    }
                ) {
                    Text(stringResource(R.string.action_open_release))
                }
            },
            dismissButton = {
                TextButton(onClick = { updateError = null }) {
                    Text(stringResource(R.string.action_later))
                }
            }
        )
    }

    LaunchedEffect(Unit) {
        if (!didAttemptAutoStart) {
            didAttemptAutoStart = true
            val shouldActivate = repository.activateOnStartup.first()
            if (shouldActivate &&
                !viewModel.isRunning &&
                viewModel.statusText != "STARTING" &&
                viewModel.statusText != "STOPPING"
            ) {
                AppLogger.i("Auto-activation: Triggering VPN flow...")
                viewModel.toggleVpn(context)
            }
        }
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
        topBar = {
            // Content never scrolls under the bar, so the container must match
            // the scaffold background exactly — translucency only adds a seam.
            TopAppBar(
                title = {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        Surface(
                            shape = RoundedCornerShape(8.dp),
                            color = MaterialTheme.colorScheme.surfaceContainerHigh,
                            modifier = Modifier.size(32.dp)
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                Image(
                                    painter = painterResource(id = R.drawable.snirect_logo),
                                    contentDescription = null,
                                    contentScale = ContentScale.Fit,
                                    modifier = Modifier
                                        .fillMaxSize()
                                        .padding(3.dp)
                                )
                            }
                        }
                        Text(
                            text = stringResource(R.string.app_name),
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                    }
                },
                actions = {
                    IconButton(onClick = { navController.navigate("help") }) {
                        Icon(
                            imageVector = Icons.Default.Info,
                            contentDescription = stringResource(R.string.help_title),
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    IconButton(onClick = { navController.navigate("logs") }) {
                        Icon(
                            imageVector = Icons.Outlined.Terminal,
                            contentDescription = stringResource(R.string.logs_title),
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    IconButton(onClick = { navController.navigate("settings") }) {
                        Icon(
                            imageVector = Icons.Default.Settings,
                            contentDescription = stringResource(R.string.settings_title),
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surfaceContainerLow
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = SnirectSpacing.marginMobile, vertical = SnirectSpacing.medium),
            verticalArrangement = Arrangement.spacedBy(SnirectSpacing.large)
        ) {
            // 1. Hero: the entire block is one big connect button
            DashboardHeroCard(
                isRunning = viewModel.isRunning,
                onToggle = { viewModel.toggleVpn(context) }
            )

            // 2. Secondary Card: CA Certificate Status
            CaCertificateCard(
                isInstalled = isCertInstalled,
                onFixNow = {
                    navController.navigate("help")
                }
            )

            // 3. Footer Version — tap to check GitHub Releases
            val versionLabel = if (checkingUpdate) {
                stringResource(R.string.update_checking)
            } else {
                stringResource(
                    R.string.version_format,
                    "${BuildConfig.VERSION_NAME} (Build ${BuildConfig.VERSION_CODE})"
                )
            }
            Text(
                text = versionLabel,
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable(enabled = !checkingUpdate) {
                        checkingUpdate = true
                        updateError = null
                        updateResult = null
                        scope.launch {
                            try {
                                val info = withContext(Dispatchers.IO) {
                                    SnirectClient.from(context).checkUpdate(BuildConfig.VERSION_NAME)
                                }
                                updateResult = info
                            } catch (e: Exception) {
                                AppLogger.e("check update failed", e)
                                updateError = e.message ?: e.toString()
                            } finally {
                                checkingUpdate = false
                            }
                        }
                    }
                    .padding(top = SnirectSpacing.medium, bottom = SnirectSpacing.large)
                    .defaultMinSize(minHeight = SnirectSpacing.minTouchTarget),
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.outline,
                textAlign = TextAlign.Center
            )
        }
    }
}

/** Distinct visual states of the hero toggle button. */
private enum class HeroButtonState { Connect, Disconnect }

@Composable
private fun DashboardHeroCard(
    isRunning: Boolean,
    onToggle: () -> Unit
) {
    // No STARTING/STOPPING interstitials: the button flips straight between
    // the two terminal states; the status machine (and the toggleVpn STOPPING
    // guard) still runs underneath, it just never shows a loading state.
    val buttonState = if (isRunning) HeroButtonState.Disconnect else HeroButtonState.Connect

    val buttonContentColor = if (isRunning) {
        MaterialTheme.colorScheme.onError
    } else {
        MaterialTheme.colorScheme.onPrimary
    }
    val buttonContainer by animateColorAsState(
        targetValue = if (isRunning) {
            MaterialTheme.colorScheme.error
        } else {
            MaterialTheme.colorScheme.primary
        },
        animationSpec = tween(SnirectMotion.MediumDurationMillis),
        label = "heroButtonContainer"
    )
    val buttonInteraction = remember { MutableInteractionSource() }

    // The hero block IS the button — no logo, no status caption, just action.
    Button(
        onClick = onToggle,
        shape = RoundedCornerShape(24.dp),
        interactionSource = buttonInteraction,
        colors = ButtonDefaults.buttonColors(
            containerColor = buttonContainer,
            contentColor = buttonContentColor
        ),
        modifier = Modifier
            .fillMaxWidth()
            .height(80.dp)
            .pressScale(interactionSource = buttonInteraction)
    ) {
        AnimatedContent(
            targetState = buttonState,
            transitionSpec = {
                val enter = fadeIn(
                    tween(SnirectMotion.SmallDurationMillis, easing = SnirectMotion.EnterEasing)
                ) + scaleIn(
                    initialScale = 0.95f,
                    animationSpec = tween(SnirectMotion.MediumDurationMillis, easing = SnirectMotion.EnterEasing)
                )
                enter togetherWith fadeOut(tween(SnirectMotion.ExitDurationMillis))
            },
            label = "heroButtonContent"
        ) { state ->
            // Full-width + Center: keeps the label dead-center in every state,
            // including the frames where AnimatedContent still holds both the
            // old and new (differently sized) contents.
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(14.dp, Alignment.CenterHorizontally)
            ) {
                Icon(
                    imageVector = Icons.Outlined.PowerSettingsNew,
                    contentDescription = null,
                    modifier = Modifier.size(28.dp)
                )
                Text(
                    text = when (state) {
                        HeroButtonState.Disconnect -> stringResource(R.string.action_disconnect)
                        HeroButtonState.Connect -> stringResource(R.string.action_connect)
                    },
                    style = MaterialTheme.typography.headlineMedium,
                    fontWeight = FontWeight.Medium
                )
            }
        }
    }
}

@Composable
private fun CaCertificateCard(
    isInstalled: Boolean?,
    onFixNow: () -> Unit
) {
    val iconColor = when (isInstalled) {
        true -> MaterialTheme.colorScheme.tertiaryContainer
        false -> MaterialTheme.colorScheme.errorContainer
        null -> MaterialTheme.colorScheme.surfaceContainerHighest
    }
    val statusText = when (isInstalled) {
        true -> stringResource(R.string.cert_installed_trusted)
        false -> stringResource(R.string.action_required)
        null -> stringResource(R.string.cert_checking)
    }
    val statusColor = when (isInstalled) {
        true -> MaterialTheme.colorScheme.tertiary
        false -> MaterialTheme.colorScheme.error
        null -> MaterialTheme.colorScheme.onSurfaceVariant
    }

    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surfaceContainer
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(16.dp),
                modifier = Modifier.weight(1f)
            ) {
                Surface(
                    shape = CircleShape,
                    color = iconColor,
                    modifier = Modifier.size(44.dp)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        if (isInstalled == null) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(20.dp),
                                strokeWidth = 2.dp,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        } else {
                            Icon(
                                imageVector = if (isInstalled) Icons.Outlined.VerifiedUser else Icons.Outlined.GppBad,
                                contentDescription = null,
                                tint = if (isInstalled) MaterialTheme.colorScheme.onTertiaryContainer else MaterialTheme.colorScheme.onErrorContainer,
                                modifier = Modifier.size(24.dp)
                            )
                        }
                    }
                }

                Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(
                        text = stringResource(R.string.ca_certificate),
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Text(
                        text = statusText,
                        style = MaterialTheme.typography.bodyMedium,
                        color = statusColor
                    )
                }
            }

            if (isInstalled == false) {
                OutlinedButton(
                    onClick = onFixNow,
                    shape = CircleShape,
                    border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
                    colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.primary),
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp)
                ) {
                    Text(
                        text = stringResource(R.string.action_fix_now),
                        style = MaterialTheme.typography.labelMedium
                    )
                }
            }
        }
    }
}
