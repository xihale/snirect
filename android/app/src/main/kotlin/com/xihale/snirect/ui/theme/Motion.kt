package com.xihale.snirect.ui.theme

import androidx.compose.animation.core.EaseInOutCubic
import androidx.compose.animation.core.EaseOutCubic
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.graphics.graphicsLayer

/**
 * Shared motion language for the whole app.
 *
 * Rules (kept in one place so every screen moves the same way):
 * - Entering elements use ease-out; on-screen movement uses ease-in-out.
 * - Exits are always faster than entrances.
 * - Nothing ever animates from scale 0.
 * - Interactions animate with springs so they stay interruptible.
 */
object SnirectMotion {
    /** Entrances: starts fast, feels responsive. */
    val EnterEasing = EaseOutCubic

    /** On-screen movement (steps, accordions, directional slides). */
    val MoveEasing = EaseInOutCubic

    /** Continuous ambient loops (waves). */
    val AmbientEasing = LinearEasing

    /** Press feedback settles in ~120ms. */
    const val PressDurationMillis = 120

    /** Small elements: chips, tags, checkboxes. */
    const val SmallDurationMillis = 150

    /** Color and fade transitions. */
    const val MediumDurationMillis = 220

    /** Content transitions (navigation slides, step changes). */
    const val ContentDurationMillis = 320

    /** Exits finish faster than the matching entrance. */
    const val ExitDurationMillis = 160

    /** Subtle press-down scale; big surfaces feel dead without it. */
    const val PressScale = 0.97f
}

/**
 * Scales the element down slightly while pressed, springing back on release.
 * Pass the same [InteractionSource] the clickable/button uses.
 */
fun Modifier.pressScale(
    interactionSource: InteractionSource,
    pressedScale: Float = SnirectMotion.PressScale
): Modifier = composed {
    val pressed by interactionSource.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed) pressedScale else 1f,
        animationSpec = spring(
            dampingRatio = Spring.DampingRatioNoBouncy,
            stiffness = Spring.StiffnessMedium
        ),
        label = "pressScale"
    )
    graphicsLayer {
        scaleX = scale
        scaleY = scale
    }
}
