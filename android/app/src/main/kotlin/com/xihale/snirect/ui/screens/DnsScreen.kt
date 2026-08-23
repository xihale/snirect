package com.xihale.snirect.ui.screens

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.Speed
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.navigation.NavController
import com.xihale.snirect.R
import com.xihale.snirect.data.repository.ConfigRepository
import com.xihale.snirect.ui.components.AppEmptyState
import com.xihale.snirect.ui.components.AppScreenScaffold
import com.xihale.snirect.ui.components.SettingsGroup
import com.xihale.snirect.ui.theme.SnirectSpacing
import com.xihale.snirect.util.DnsProbe
import kotlinx.coroutines.launch

/** Per-server state of the one-shot connectivity test. */
private sealed interface DnsTestResult {
    data object Running : DnsTestResult
    data class Ok(val latencyMs: Long) : DnsTestResult
    data class Failed(val reason: String) : DnsTestResult
}

@Composable
fun DnsScreen(
    navController: NavController,
    repository: ConfigRepository
) {
    var nameservers by remember { mutableStateOf<List<String>>(emptyList()) }
    var bootstrapDnsText by remember { mutableStateOf("") }
    // Last persisted value — drives the inline save affordance.
    var savedBootstrap by remember { mutableStateOf("") }
    var showAddDialog by remember { mutableStateOf(false) }
    var dialogValue by remember { mutableStateOf("") }
    // Null = adding, otherwise the index being edited.
    var editingIndex by remember { mutableStateOf<Int?>(null) }
    val testStates = remember { mutableStateMapOf<String, DnsTestResult>() }

    val scope = rememberCoroutineScope()

    LaunchedEffect(Unit) { repository.nameservers.collect { nameservers = it } }
    LaunchedEffect(Unit) {
        repository.bootstrapDns.collect { bootstrapDns ->
            val joined = bootstrapDns.joinToString(",")
            bootstrapDnsText = joined
            savedBootstrap = joined
        }
    }

    AppScreenScaffold(
        title = stringResource(R.string.dns_title),
        onBack = { navController.popBackStack() },
        backContentDescription = stringResource(R.string.action_back),
        floatingActionButton = {
            FloatingActionButton(
                onClick = {
                    editingIndex = null
                    dialogValue = ""
                    showAddDialog = true
                },
                containerColor = MaterialTheme.colorScheme.primary,
                contentColor = MaterialTheme.colorScheme.onPrimary,
                shape = CircleShape
            ) {
                Icon(Icons.Outlined.Add, contentDescription = stringResource(R.string.action_add_server))
            }
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
            // 1. BOOTSTRAP DNS GROUP — one field, inline save when it differs
            // from the persisted value.
            SettingsGroup(title = stringResource(R.string.group_bootstrap)) {
                OutlinedTextField(
                    value = bootstrapDnsText,
                    onValueChange = { bootstrapDnsText = it },
                    placeholder = { Text(stringResource(R.string.placeholder_bootstrap_dns)) },
                    supportingText = { Text(stringResource(R.string.bootstrap_hint)) },
                    trailingIcon = {
                        if (bootstrapDnsText != savedBootstrap) {
                            IconButton(onClick = {
                                val list = bootstrapDnsText
                                    .split(",")
                                    .map { value -> value.trim() }
                                    .filter { value -> value.isNotBlank() }
                                scope.launch { repository.setBootstrapDns(list) }
                            }) {
                                Icon(
                                    Icons.Outlined.Check,
                                    contentDescription = stringResource(R.string.action_save),
                                    tint = MaterialTheme.colorScheme.primary
                                )
                            }
                        }
                    },
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    singleLine = true,
                    shape = RoundedCornerShape(12.dp),
                    textStyle = MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace)
                )
            }

            // 2. UPSTREAM NAMESERVERS GROUP
            SettingsGroup(title = stringResource(R.string.group_upstream)) {
                if (nameservers.isEmpty()) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = SnirectSpacing.xxLarge),
                        contentAlignment = Alignment.Center
                    ) {
                        AppEmptyState(
                            icon = Icons.Outlined.Dns,
                            title = stringResource(R.string.no_custom_nameservers)
                        )
                    }
                } else {
                    nameservers.forEachIndexed { index, server ->
                        if (index > 0) {
                            HorizontalDivider(color = MaterialTheme.colorScheme.surfaceContainerLow, thickness = 1.dp)
                        }

                        // Tapping the row edits the entry in place.
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable {
                                    editingIndex = index
                                    dialogValue = server
                                    showAddDialog = true
                                }
                                .padding(horizontal = 16.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Outlined.Dns,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.secondary,
                                modifier = Modifier.size(24.dp)
                            )

                            Column(
                                modifier = Modifier.weight(1f),
                                verticalArrangement = Arrangement.spacedBy(2.dp)
                            ) {
                                Text(
                                    text = server,
                                    style = MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace),
                                    color = MaterialTheme.colorScheme.onSurface
                                )
                                val typeLabel = when {
                                    server.startsWith("https://") -> "DoH (DNS over HTTPS)"
                                    server.startsWith("tls://") -> "DoT (DNS over TLS)"
                                    else -> "UDP"
                                }
                                Text(
                                    text = typeLabel,
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                                when (val test = testStates[server]) {
                                    DnsTestResult.Running -> Text(
                                        text = stringResource(R.string.dns_test_running),
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant
                                    )
                                    is DnsTestResult.Ok -> Text(
                                        text = stringResource(R.string.dns_test_ok, test.latencyMs),
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.tertiary
                                    )
                                    is DnsTestResult.Failed -> Text(
                                        text = stringResource(R.string.dns_test_failed, test.reason),
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.error
                                    )
                                    null -> Unit
                                }
                            }

                            // Connectivity test: probes the server with the
                            // same transport it is configured for.
                            IconButton(
                                enabled = testStates[server] !is DnsTestResult.Running,
                                onClick = {
                                    scope.launch {
                                        testStates[server] = DnsTestResult.Running
                                        testStates[server] = DnsProbe.test(server).fold(
                                            onSuccess = { DnsTestResult.Ok(it) },
                                            onFailure = { DnsTestResult.Failed(it.message ?: "error") }
                                        )
                                    }
                                }
                            ) {
                                if (testStates[server] is DnsTestResult.Running) {
                                    CircularProgressIndicator(
                                        modifier = Modifier.size(20.dp),
                                        strokeWidth = 2.dp
                                    )
                                } else {
                                    Icon(
                                        Icons.Outlined.Speed,
                                        contentDescription = stringResource(R.string.action_test_server),
                                        tint = MaterialTheme.colorScheme.secondary
                                    )
                                }
                            }

                            IconButton(
                                onClick = {
                                    val newList = nameservers.toMutableList()
                                    newList.remove(server)
                                    testStates.remove(server)
                                    scope.launch { repository.setNameservers(newList) }
                                }
                            ) {
                                Icon(
                                    Icons.Outlined.Delete,
                                    contentDescription = stringResource(R.string.action_remove),
                                    tint = MaterialTheme.colorScheme.error
                                )
                            }
                        }
                    }
                }
            }
        }
    }

    // Add / edit server dialog (text input, so a dialog stays appropriate).
    if (showAddDialog) {
        val isEditing = editingIndex != null
        AlertDialog(
            onDismissRequest = {
                showAddDialog = false
                editingIndex = null
            },
            title = {
                Text(
                    stringResource(
                        if (isEditing) R.string.dialog_edit_upstream else R.string.dialog_add_upstream
                    )
                )
            },
            text = {
                OutlinedTextField(
                    value = dialogValue,
                    onValueChange = { dialogValue = it },
                    label = { Text(stringResource(R.string.label_endpoint_url)) },
                    placeholder = { Text(stringResource(R.string.placeholder_endpoint_url)) },
                    modifier = Modifier.fillMaxWidth(),
                    singleLine = true,
                    shape = RoundedCornerShape(12.dp),
                    textStyle = MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace)
                )
            },
            confirmButton = {
                Button(
                    onClick = {
                        if (dialogValue.isNotBlank()) {
                            val trimmed = dialogValue.trim()
                            val newList = nameservers.toMutableList()
                            val index = editingIndex
                            if (index != null && index < newList.size) {
                                testStates.remove(newList[index])
                                newList[index] = trimmed
                            } else {
                                newList.add(trimmed)
                            }
                            scope.launch { repository.setNameservers(newList) }
                            dialogValue = ""
                            editingIndex = null
                            showAddDialog = false
                        }
                    }
                ) {
                    Text(
                        stringResource(
                            if (isEditing) R.string.action_save else R.string.action_add
                        )
                    )
                }
            },
            dismissButton = {
                TextButton(
                    onClick = {
                        showAddDialog = false
                        editingIndex = null
                    }
                ) {
                    Text(stringResource(R.string.action_cancel))
                }
            }
        )
    }
}
