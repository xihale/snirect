package com.xihale.snirect.ui.theme

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

private val LightColorScheme = lightColorScheme(
    primary = MdLightPrimary,
    onPrimary = MdLightOnPrimary,
    primaryContainer = MdLightPrimaryContainer,
    onPrimaryContainer = MdLightOnPrimaryContainer,
    inversePrimary = MdLightInversePrimary,
    secondary = MdLightSecondary,
    onSecondary = MdLightOnSecondary,
    secondaryContainer = MdLightSecondaryContainer,
    onSecondaryContainer = MdLightOnSecondaryContainer,
    tertiary = MdLightTertiary,
    onTertiary = MdLightOnTertiary,
    tertiaryContainer = MdLightTertiaryContainer,
    onTertiaryContainer = MdLightOnTertiaryContainer,
    error = MdLightError,
    onError = MdLightOnError,
    errorContainer = MdLightErrorContainer,
    onErrorContainer = MdLightOnErrorContainer,
    background = MdLightBackground,
    onBackground = MdLightOnBackground,
    surface = MdLightSurface,
    onSurface = MdLightOnSurface,
    surfaceVariant = MdLightSurfaceVariant,
    onSurfaceVariant = MdLightOnSurfaceVariant,
    surfaceDim = MdLightSurfaceDim,
    surfaceBright = MdLightSurfaceBright,
    surfaceContainerLowest = MdLightSurfaceContainerLowest,
    surfaceContainerLow = MdLightSurfaceContainerLow,
    surfaceContainer = MdLightSurfaceContainer,
    surfaceContainerHigh = MdLightSurfaceContainerHigh,
    surfaceContainerHighest = MdLightSurfaceContainerHighest,
    outline = MdLightOutline,
    outlineVariant = MdLightOutlineVariant,
    surfaceTint = MdLightSurfaceTint,
    inverseSurface = MdLightInverseSurface,
    inverseOnSurface = MdLightInverseOnSurface,
    scrim = Color.Black
)

private val DarkColorScheme = darkColorScheme(
    primary = MdDarkPrimary,
    onPrimary = MdDarkOnPrimary,
    primaryContainer = MdDarkPrimaryContainer,
    onPrimaryContainer = MdDarkOnPrimaryContainer,
    inversePrimary = MdDarkInversePrimary,
    secondary = MdDarkSecondary,
    onSecondary = MdDarkOnSecondary,
    secondaryContainer = MdDarkSecondaryContainer,
    onSecondaryContainer = MdDarkOnSecondaryContainer,
    tertiary = MdDarkTertiary,
    onTertiary = MdDarkOnTertiary,
    tertiaryContainer = MdDarkTertiaryContainer,
    onTertiaryContainer = MdDarkOnTertiaryContainer,
    error = MdDarkError,
    onError = MdDarkOnError,
    errorContainer = MdDarkErrorContainer,
    onErrorContainer = MdDarkOnErrorContainer,
    background = MdDarkBackground,
    onBackground = MdDarkOnBackground,
    surface = MdDarkSurface,
    onSurface = MdDarkOnSurface,
    surfaceVariant = MdDarkSurfaceVariant,
    onSurfaceVariant = MdDarkOnSurfaceVariant,
    surfaceDim = MdDarkSurfaceDim,
    surfaceBright = MdDarkSurfaceBright,
    surfaceContainerLowest = MdDarkSurfaceContainerLowest,
    surfaceContainerLow = MdDarkSurfaceContainerLow,
    surfaceContainer = MdDarkSurfaceContainer,
    surfaceContainerHigh = MdDarkSurfaceContainerHigh,
    surfaceContainerHighest = MdDarkSurfaceContainerHighest,
    outline = MdDarkOutline,
    outlineVariant = MdDarkOutlineVariant,
    surfaceTint = MdDarkSurfaceTint,
    inverseSurface = MdDarkInverseSurface,
    inverseOnSurface = MdDarkInverseOnSurface,
    scrim = Color.Black
)

@Composable
fun SnirectTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val colorScheme = if (darkTheme) DarkColorScheme else LightColorScheme

    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as Activity).window
            WindowCompat.getInsetsController(window, view).isAppearanceLightStatusBars = !darkTheme
            WindowCompat.getInsetsController(window, view).isAppearanceLightNavigationBars = !darkTheme
        }
    }

    MaterialTheme(
        colorScheme = colorScheme,
        typography = Typography,
        shapes = SnirectShapes,
        content = content
    )
}
