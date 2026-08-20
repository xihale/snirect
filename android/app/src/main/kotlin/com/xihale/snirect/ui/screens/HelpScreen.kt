package com.xihale.snirect.ui.screens

import android.content.ContentValues
import android.content.Context
import android.content.Intent
import android.os.Environment
import android.provider.MediaStore
import android.provider.Settings
import android.widget.Toast
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.Crossfade
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.Download
import androidx.compose.material.icons.outlined.GppBad
import androidx.compose.material.icons.outlined.VerifiedUser
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.navigation.NavController
import com.xihale.snirect.MainViewModel
import com.xihale.snirect.R
import com.xihale.snirect.ktlib.SnirectClient
import com.xihale.snirect.ui.components.AppScreenScaffold
import com.xihale.snirect.ui.components.SettingsGroup
import com.xihale.snirect.ui.theme.SnirectMotion
import com.xihale.snirect.ui.theme.SnirectSpacing
import com.xihale.snirect.ui.theme.pressScale
import com.xihale.snirect.util.AppLogger
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

@Composable
fun HelpScreen(
    navController: NavController,
    viewModel: MainViewModel? = null,
) {
    val context = LocalContext.current
    val coroutineScope = rememberCoroutineScope()

    var currentStep by remember { mutableIntStateOf(1) }
    var isExporting by remember { mutableStateOf(false) }
    var isExported by remember { mutableStateOf(false) }
    var isVerifying by remember { mutableStateOf(false) }
    var isVerified by remember { mutableStateOf(false) }
    var verifyChecked by remember { mutableStateOf(false) }
    var awaitingSettingsReturn by remember { mutableStateOf(false) }

    // Re-check the CA cert on every ON_RESUME: first entry, and when the user
    // comes back from system security settings. Export already jumps to install;
    // returning from settings jumps to verify the same way.
    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        var isFirstResume = true
        val observer = LifecycleEventObserver { _, event ->
            if (event != Lifecycle.Event.ON_RESUME) return@LifecycleEventObserver
            val isInitial = isFirstResume
            isFirstResume = false
            val fromSettings = awaitingSettingsReturn
            awaitingSettingsReturn = false
            coroutineScope.launch {
                val installed = viewModel?.scanCertStatus() ?: withContext(Dispatchers.IO) {
                    SnirectClient.from(context).isCaCertificateInstalled()
                }
                isVerified = installed
                if (installed) {
                    verifyChecked = true
                    isExported = true
                    if (isInitial || currentStep == 2) currentStep = 3
                } else if (fromSettings) {
                    verifyChecked = true
                    currentStep = 3
                } else if (!isInitial) {
                    verifyChecked = true
                }
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    AppScreenScaffold(
        title = stringResource(R.string.help_title),
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
            // 1. STEP PROGRESS INDICATOR
            StepProgressHeader(currentStep = currentStep, onStepClick = { step -> currentStep = step })

            // 2. MAIN INTERACTIVE STEP CARD
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(24.dp),
                color = MaterialTheme.colorScheme.surfaceContainer,
                shadowElevation = 1.dp
            ) {
                Column(modifier = Modifier.fillMaxWidth()) {
                    // Decorative Illustration Area
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(140.dp)
                            .background(
                                brush = Brush.linearGradient(
                                    colors = listOf(
                                        MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.35f),
                                        MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.25f)
                                    )
                                )
                            ),
                        contentAlignment = Alignment.Center
                    ) {
                        Surface(
                            shape = CircleShape,
                            color = MaterialTheme.colorScheme.surface.copy(alpha = 0.85f),
                            modifier = Modifier.size(72.dp),
                            shadowElevation = 2.dp
                        ) {
                            Box(contentAlignment = Alignment.Center) {
                                Crossfade(
                                    targetState = currentStep,
                                    animationSpec = tween(SnirectMotion.MediumDurationMillis),
                                    label = "stepIcon"
                                ) { step ->
                                    Icon(
                                        imageVector = when (step) {
                                            1 -> Icons.Outlined.Download
                                            2 -> Icons.Default.Settings
                                            else -> Icons.Outlined.VerifiedUser
                                        },
                                        contentDescription = null,
                                        tint = MaterialTheme.colorScheme.primary,
                                        modifier = Modifier.size(36.dp)
                                    )
                                }
                            }
                        }
                    }

                    // Content & Actions based on Step — slides horizontally in
                    // the direction of travel so the stepper has a sense of place.
                    AnimatedContent(
                        targetState = currentStep,
                        transitionSpec = {
                            val direction = if (targetState > initialState) 1 else -1
                            val enter = slideInHorizontally(
                                tween(SnirectMotion.ContentDurationMillis, easing = SnirectMotion.MoveEasing)
                            ) { direction * it / 3 } + fadeIn(
                                tween(SnirectMotion.MediumDurationMillis)
                            )
                            val exit = slideOutHorizontally(
                                tween(SnirectMotion.ContentDurationMillis, easing = SnirectMotion.MoveEasing)
                            ) { -direction * it / 3 } + fadeOut(
                                tween(SnirectMotion.ExitDurationMillis)
                            )
                            enter togetherWith exit
                        },
                        label = "stepContent"
                    ) { step ->
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(20.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(16.dp)
                        ) {
                            when (step) {
                            1 -> {
                                Text(
                                    text = stringResource(R.string.setup_step_1_title),
                                    style = MaterialTheme.typography.titleLarge,
                                    fontWeight = FontWeight.Bold,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    textAlign = TextAlign.Center
                                )
                                Text(
                                    text = stringResource(R.string.setup_step_1_desc),
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    textAlign = TextAlign.Center
                                )

                                val exportInteraction = remember { MutableInteractionSource() }
                                Button(
                                    onClick = {
                                        if (!isExporting) {
                                            isExporting = true
                                            coroutineScope.launch {
                                                exportCertificate(context) { success ->
                                                    isExporting = false
                                                    if (success) {
                                                        isExported = true
                                                        currentStep = 2
                                                    }
                                                }
                                            }
                                        }
                                    },
                                    shape = CircleShape,
                                    interactionSource = exportInteraction,
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .height(50.dp)
                                        .pressScale(interactionSource = exportInteraction),
                                    colors = if (isExported) {
                                        ButtonDefaults.buttonColors(
                                            containerColor = MaterialTheme.colorScheme.tertiaryContainer,
                                            contentColor = MaterialTheme.colorScheme.onTertiaryContainer
                                        )
                                    } else {
                                        ButtonDefaults.buttonColors(
                                            containerColor = MaterialTheme.colorScheme.primary,
                                            contentColor = MaterialTheme.colorScheme.onPrimary
                                        )
                                    }
                                ) {
                                    if (isExporting) {
                                        CircularProgressIndicator(
                                            modifier = Modifier.size(20.dp),
                                            color = MaterialTheme.colorScheme.onPrimary,
                                            strokeWidth = 2.dp
                                        )
                                        Spacer(modifier = Modifier.width(8.dp))
                                        Text(stringResource(R.string.exporting))
                                    } else if (isExported) {
                                        Icon(Icons.Default.Check, contentDescription = null)
                                        Spacer(modifier = Modifier.width(8.dp))
                                        Text(stringResource(R.string.certificate_exported))
                                    } else {
                                        Icon(Icons.Outlined.Download, contentDescription = null)
                                        Spacer(modifier = Modifier.width(8.dp))
                                        Text(stringResource(R.string.action_export_downloads))
                                    }
                                }
                            }

                            2 -> {
                                Text(
                                    text = stringResource(R.string.setup_step_2_title),
                                    style = MaterialTheme.typography.titleLarge,
                                    fontWeight = FontWeight.Bold,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    textAlign = TextAlign.Center
                                )
                                Text(
                                    text = stringResource(R.string.setup_step_2_desc),
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    textAlign = TextAlign.Start
                                )

                                val openSettingsInteraction = remember { MutableInteractionSource() }
                                Button(
                                    onClick = {
                                        try {
                                            awaitingSettingsReturn = true
                                            val intent = Intent(Settings.ACTION_SECURITY_SETTINGS)
                                            context.startActivity(intent)
                                        } catch (e: Exception) {
                                            awaitingSettingsReturn = false
                                            AppLogger.e("Failed to open security settings", e)
                                        }
                                    },
                                    shape = CircleShape,
                                    interactionSource = openSettingsInteraction,
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .height(50.dp)
                                        .pressScale(interactionSource = openSettingsInteraction)
                                ) {
                                    Icon(Icons.Default.Settings, contentDescription = null)
                                    Spacer(modifier = Modifier.width(8.dp))
                                    Text(stringResource(R.string.action_open_security_settings))
                                }
                            }

                            3 -> {
                                Text(
                                    text = stringResource(R.string.setup_step_3_title),
                                    style = MaterialTheme.typography.titleLarge,
                                    fontWeight = FontWeight.Bold,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    textAlign = TextAlign.Center
                                )
                                Text(
                                    text = stringResource(R.string.setup_step_3_desc),
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    textAlign = TextAlign.Center
                                )

                                if (verifyChecked) {
                                    Surface(
                                        shape = RoundedCornerShape(12.dp),
                                        color = if (isVerified) MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.35f)
                                        else MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.35f),
                                        modifier = Modifier.fillMaxWidth()
                                    ) {
                                        Row(
                                            modifier = Modifier.padding(14.dp),
                                            verticalAlignment = Alignment.CenterVertically,
                                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                                        ) {
                                            Icon(
                                                imageVector = if (isVerified) Icons.Outlined.VerifiedUser else Icons.Outlined.GppBad,
                                                contentDescription = null,
                                                tint = if (isVerified) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.error
                                            )
                                            Text(
                                                text = if (isVerified) stringResource(R.string.cert_verified_success) else stringResource(R.string.cert_not_verified),
                                                style = MaterialTheme.typography.bodyMedium,
                                                color = MaterialTheme.colorScheme.onSurface
                                            )
                                        }
                                    }
                                }

                                val verifyInteraction = remember { MutableInteractionSource() }
                                Button(
                                    onClick = {
                                        isVerifying = true
                                        coroutineScope.launch {
                                            val installed = viewModel?.scanCertStatus()
                                                ?: withContext(Dispatchers.IO) {
                                                    SnirectClient.from(context).isCaCertificateInstalled()
                                                }
                                            isVerifying = false
                                            isVerified = installed
                                            verifyChecked = true
                                        }
                                    },
                                    shape = CircleShape,
                                    interactionSource = verifyInteraction,
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .height(50.dp)
                                        .pressScale(interactionSource = verifyInteraction)
                                ) {
                                    if (isVerifying) {
                                        CircularProgressIndicator(
                                            modifier = Modifier.size(20.dp),
                                            color = MaterialTheme.colorScheme.onPrimary,
                                            strokeWidth = 2.dp
                                        )
                                        Spacer(modifier = Modifier.width(8.dp))
                                    }
                                    Text(stringResource(R.string.action_verify_cert))
                                }
                            }
                        }
                    }
                }
            }
            }

            // 3. FAQ / REFERENCE ACCORDION SECTION
            SettingsGroup(title = stringResource(R.string.help_title)) {
                AccordionItem(
                    title = stringResource(R.string.help_section_1_title),
                    content = stringResource(R.string.help_section_1_content)
                )
                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)
                AccordionItem(
                    title = stringResource(R.string.help_section_2_title),
                    content = stringResource(R.string.help_section_2_content)
                )
                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)
                AccordionItem(
                    title = stringResource(R.string.help_section_3_title),
                    content = stringResource(R.string.help_section_3_content)
                )
                HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)
                AccordionItem(
                    title = stringResource(R.string.help_section_4_title),
                    content = stringResource(R.string.help_section_4_content)
                )
            }
        }
    }
}

private val StepCircleSize = 40.dp

@Composable
private fun StepProgressHeader(
    currentStep: Int,
    onStepClick: (Int) -> Unit
) {
    val steps = listOf(
        Triple(1, stringResource(R.string.step_export), Icons.Outlined.Download),
        Triple(2, stringResource(R.string.step_install), Icons.Default.Settings),
        Triple(3, stringResource(R.string.step_verify), Icons.Outlined.VerifiedUser),
    )

    BoxWithConstraints(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = SnirectSpacing.gutter)
    ) {
        val slotWidth = maxWidth / steps.size

        Column(
            modifier = Modifier.fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(StepCircleSize)
            ) {
                // Track sits on the circle centers so the dots don't float above the line.
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .align(Alignment.Center)
                        .padding(horizontal = slotWidth / 2)
                        .height(2.dp)
                ) {
                    StepConnector(active = currentStep >= 2)
                    StepConnector(active = currentStep >= 3)
                }
                Row(modifier = Modifier.fillMaxSize()) {
                    steps.forEach { (number, label, icon) ->
                        Box(
                            modifier = Modifier
                                .weight(1f)
                                .fillMaxHeight(),
                            contentAlignment = Alignment.Center
                        ) {
                            StepCircle(
                                label = label,
                                icon = icon,
                                isActive = currentStep >= number,
                                isCurrent = currentStep == number,
                                onClick = { onStepClick(number) }
                            )
                        }
                    }
                }
            }
            Row(modifier = Modifier.fillMaxWidth()) {
                steps.forEach { (number, label, _) ->
                    Text(
                        text = label,
                        style = MaterialTheme.typography.labelMedium,
                        color = if (currentStep >= number) {
                            MaterialTheme.colorScheme.primary
                        } else {
                            MaterialTheme.colorScheme.onSurfaceVariant
                        },
                        fontWeight = if (currentStep == number) FontWeight.Bold else FontWeight.Normal,
                        textAlign = TextAlign.Center,
                        maxLines = 1,
                        modifier = Modifier
                            .weight(1f)
                            .clickable { onStepClick(number) }
                    )
                }
            }
        }
    }
}

@Composable
private fun RowScope.StepConnector(active: Boolean) {
    val color by animateColorAsState(
        targetValue = if (active) MaterialTheme.colorScheme.primary
        else MaterialTheme.colorScheme.outlineVariant,
        animationSpec = tween(SnirectMotion.MediumDurationMillis),
        label = "connectorColor"
    )
    Box(
        modifier = Modifier
            .weight(1f)
            .fillMaxHeight()
            .background(color)
    )
}

@Composable
private fun StepCircle(
    label: String,
    icon: ImageVector,
    isActive: Boolean,
    isCurrent: Boolean,
    onClick: () -> Unit
) {
    val bgColor by animateColorAsState(
        targetValue = when {
            isCurrent -> MaterialTheme.colorScheme.primary
            isActive -> MaterialTheme.colorScheme.primaryContainer
            else -> MaterialTheme.colorScheme.surfaceVariant
        },
        animationSpec = tween(SnirectMotion.MediumDurationMillis),
        label = "stepBg"
    )
    val contentColor by animateColorAsState(
        targetValue = when {
            isCurrent -> MaterialTheme.colorScheme.onPrimary
            isActive -> MaterialTheme.colorScheme.onPrimaryContainer
            else -> MaterialTheme.colorScheme.onSurfaceVariant
        },
        animationSpec = tween(SnirectMotion.MediumDurationMillis),
        label = "stepFg"
    )

    Surface(
        onClick = onClick,
        shape = CircleShape,
        color = bgColor,
        contentColor = contentColor,
        modifier = Modifier.size(StepCircleSize),
        shadowElevation = if (isCurrent) 2.dp else 0.dp
    ) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                imageVector = icon,
                contentDescription = label,
                modifier = Modifier.size(20.dp)
            )
        }
    }
}

@Composable
private fun AccordionItem(
    title: String,
    content: String
) {
    var expanded by remember { mutableStateOf(false) }

    Column(modifier = Modifier.fillMaxWidth()) {
        ListItem(
            headlineContent = { Text(title) },
            supportingContent = if (expanded) {
                { Text(content) }
            } else {
                null
            },
            trailingContent = {
                Icon(
                    imageVector = Icons.Default.KeyboardArrowDown,
                    contentDescription = null,
                    modifier = Modifier.graphicsLayer {
                        rotationZ = if (expanded) 180f else 0f
                    }
                )
            },
            colors = ListItemDefaults.colors(containerColor = Color.Transparent),
            modifier = Modifier.clickable { expanded = !expanded }
        )
    }
}

private suspend fun exportCertificate(context: Context, onComplete: (Boolean) -> Unit) {
    withContext(Dispatchers.IO) {
        try {
            AppLogger.i("Exporting CA cert via MediaStore...")
            val certBytes = SnirectClient.from(context).caCertificate() ?: throw Exception("Null cert")
            if (certBytes.isEmpty()) throw Exception("Empty cert")

            val resolver = context.contentResolver
            val contentValues = ContentValues().apply {
                put(MediaStore.MediaColumns.DISPLAY_NAME, "snirect_ca.crt")
                put(MediaStore.MediaColumns.MIME_TYPE, "application/x-x509-ca-cert")
                put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS)
            }

            val uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, contentValues)
            if (uri != null) {
                resolver.openOutputStream(uri)?.use { outputStream ->
                    outputStream.write(certBytes)
                }
                withContext(Dispatchers.Main) {
                    Toast.makeText(context, context.getString(R.string.toast_saved_to_downloads), Toast.LENGTH_LONG).show()
                    onComplete(true)
                }
            } else {
                throw Exception("Could not insert MediaStore entry")
            }
        } catch (e: Exception) {
            AppLogger.e("Export error", e)
            withContext(Dispatchers.Main) {
                Toast.makeText(context, context.getString(R.string.toast_export_failed, e.message), Toast.LENGTH_SHORT).show()
                onComplete(false)
            }
        }
    }
}
