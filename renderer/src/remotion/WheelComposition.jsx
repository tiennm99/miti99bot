import React from 'react';
import {AbsoluteFill, Easing, interpolate, useCurrentFrame, useVideoConfig} from 'remotion';
import {Circle, Pie} from '@remotion/shapes';
import {
  cssDegreesToPieRadians,
  getFinalWheelRotationDegrees,
  getPointerDeflectionDegrees,
  getSliceCenterDegrees,
  getSliceDegrees,
  getSpinRecoilDegrees,
  getSpinTurns,
  getWheelRotationDegrees,
} from './wheel-layout.js';
import {
  estimateTextWidth,
  getLabelLines,
  getRadialLabelLayout,
  labelLineHeightRatio,
} from './wheel-label-layout.js';
import {getSliceColorIndex, getTheme} from './themes.js';
import {Confetti} from './Confetti.jsx';

const baseFont =
  'Quicksand, Inter, "Noto Sans", "Noto Sans Vietnamese", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

/**
 * @typedef {import('../schemas/wheel-request.js').WheelRenderRequest} WheelRenderRequest
 */

/**
 * @param {WheelRenderRequest} props
 */
export const WheelComposition = (props) => {
  const frame = useCurrentFrame();
  const {durationInFrames, fps, width, height} = useVideoConfig();
  const theme = getTheme(props.theme);
  const spinFrames = Math.max(1, durationInFrames - Math.round((props.holdMs / 1000) * fps));
  // Any ease-out ends at zero speed, so the wheel is already visually stopped a
  // few frames before it mathematically settles. Firing the celebration there
  // lands it on the perceived stop instead of after a pause, and costs only a
  // fraction of a degree of rotation.
  const revealFrame = Math.max(0, spinFrames - Math.round(fps * 0.33));
  const revealProgress = interpolate(frame, [revealFrame, durationInFrames - 1], [0, 1], {
    easing: Easing.out(Easing.sin),
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });
  const winnerPulse = frame >= revealFrame ? Math.sin(revealProgress * Math.PI) : 0;
  const isRevealed = frame >= revealFrame;

  const size = Math.min(width, height);
  const center = size / 2;
  const radius = size * 0.41;
  const centerRingRadius = size * 0.09;
  const centerRadius = size * 0.06;
  const sliceDegrees = getSliceDegrees(props.options.length);
  const winnerCenterDegrees = getSliceCenterDegrees(props.options.length, props.winnerIndex);
  // Turn count scales with the frame budget and the slice width: a dense wheel
  // spun as far as a sparse one moves more than a whole wedge per frame and
  // aliases into a backwards-looking blur.
  const spinTurns = getSpinTurns(spinFrames, props.options.length);
  const finalRotation = getFinalWheelRotationDegrees(
    props.options.length,
    props.winnerIndex,
    spinTurns,
  );
  const startRotation = -24;
  const rotation =
    getWheelRotationDegrees({
      finalRotationDegrees: finalRotation,
      frame,
      spinFrames,
      startRotationDegrees: startRotation,
    }) + getSpinRecoilDegrees(frame, spinFrames, sliceDegrees);
  const pointerDeflection = getPointerDeflectionDegrees(rotation, sliceDegrees);

  const paletteLength = theme.slices.length;
  // The width estimator is calibrated against the 700-weight slice labels, so
  // the 800-weight pill runs wider than it predicts. Budgeting for that keeps
  // the browser from re-wrapping past the two lines chosen here.
  const pillBoldWidthFactor = 1.12;
  const pillMaxWidth = size * 0.58;
  const pillBaseFontSize = size * 0.075;
  const winnerText = props.options[props.winnerIndex] ?? '';
  const winnerLines = getLabelLines(winnerText, pillBaseFontSize, pillMaxWidth, 2);
  const longestWinnerWidth = winnerLines.reduce(
    (widest, line) => Math.max(widest, estimateTextWidth(line, pillBaseFontSize)),
    0,
  ) * pillBoldWidthFactor;
  const winnerFontSize = Math.max(
    size * 0.038,
    longestWinnerWidth > pillMaxWidth
      ? pillBaseFontSize * (pillMaxWidth / longestWinnerWidth)
      : pillBaseFontSize,
  );
  const pillPop = interpolate(frame, [revealFrame, revealFrame + 4], [0, 1], {
    easing: Easing.out(Easing.cubic),
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });

  return (
    <AbsoluteFill
      style={{
        alignItems: 'center',
        background: theme.background,
        display: 'flex',
        fontFamily: baseFont,
        justifyContent: 'center',
      }}
    >
      <div
        style={{
          height: size,
          position: 'relative',
          width: size,
        }}
      >
        <div
          style={{
            borderRadius: '50%',
            boxShadow: theme.wheelShadow,
            height: radius * 2,
            left: center - radius,
            position: 'absolute',
            top: center - radius,
            width: radius * 2,
          }}
        />

        {/* translateZ promotes this one layer, which switches Chromium from LCD
            sub-pixel text antialiasing to grayscale for every label inside it.
            Sub-pixel AA emits saturated color fringes on each glyph edge that
            consume most of the 256-entry GIF palette. Promoting the labels
            individually works too, but costs one layer per option and doubles
            render time on a dense wheel. */}
        <div
          style={{
            height: size,
            left: 0,
            position: 'absolute',
            top: 0,
            transform: `rotate(${rotation}deg) translateZ(0)`,
            transformOrigin: '50% 50%',
            width: size,
          }}
        >
          {props.options.map((option, index) => {
            const start = index * sliceDegrees;
            const centerDegrees = getSliceCenterDegrees(props.options.length, index);
            const isWinner = index === props.winnerIndex;
            const colorIndex = getSliceColorIndex(index, props.options.length, paletteLength);
            const color = theme.slices[colorIndex] ?? theme.slices[0] ?? '#cccccc';
            const label = getRadialLabelLayout({
              center,
              radius,
              hubRadius: centerRingRadius,
              centerDegrees,
              optionCount: props.options.length,
              text: option,
              winnerCenterDegrees,
            });

            return (
              <React.Fragment key={`${option}-${index}`}>
                <Pie
                  fill={color}
                  progress={1 / props.options.length}
                  radius={radius}
                  rotation={cssDegreesToPieRadians(start)}
                  // An opaque stroke instead of a blurred drop-shadow: a filter
                  // on a Pie paints outside the shape, so later siblings
                  // overpaint it and only one neighbour shows the halo. A stroke
                  // also stays legible after GIF quantisation.
                  stroke={isWinner && isRevealed ? theme.winnerStroke : theme.sliceStroke}
                  strokeWidth={isWinner && isRevealed ? 4 + Math.round(winnerPulse * 3) : 2}
                  style={{
                    left: center - radius,
                    position: 'absolute',
                    top: center - radius,
                  }}
                />
                <div
                  style={{
                    alignItems: 'center',
                    boxSizing: 'border-box',
                    color: theme.text,
                    display: 'flex',
                    fontSize: label.fontSize,
                    flexDirection: 'column',
                    fontWeight: 700,
                    height: label.height,
                    justifyContent: 'center',
                    left: label.x,
                    letterSpacing: 0,
                    lineHeight: labelLineHeightRatio,
                    overflow: 'hidden',
                    padding: `0 ${label.horizontalPadding}px`,
                    position: 'absolute',
                    textAlign: 'center',
                    textShadow: `0 1px 0 ${theme.textHalo}, 0 -1px 0 ${theme.textHalo}, 1px 0 0 ${theme.textHalo}, -1px 0 0 ${theme.textHalo}`,
                    top: label.y,
                    transform: `translate(-50%, -50%) rotate(${label.rotation}deg)`,
                    transformOrigin: '50% 50%',
                    width: label.width,
                  }}
                >
                  {label.lines.map((line, lineIndex) => (
                    <div key={`${line}-${lineIndex}`} style={{flex: '0 0 auto', whiteSpace: 'nowrap'}}>
                      {line}
                    </div>
                  ))}
                </div>
              </React.Fragment>
            );
          })}
        </div>

        <Circle
          fill={theme.centerRing}
          radius={centerRingRadius}
          style={{left: center - centerRingRadius, position: 'absolute', top: center - centerRingRadius}}
        />
        <Circle
          fill={theme.center}
          radius={centerRadius}
          style={{left: center - centerRadius, position: 'absolute', top: center - centerRadius}}
        />
        {/* Drawn as SVG so the outline actually renders: clip-path discards a
            border and a box-shadow along with everything else it clips. */}
        <svg
          width={size * 0.1}
          height={size * 0.105}
          viewBox="0 0 100 105"
          style={{
            left: center + radius - size * 0.045,
            position: 'absolute',
            top: center - size * 0.0525,
            transform: `rotate(${pointerDeflection}deg)`,
            transformOrigin: '100% 50%',
            zIndex: 4,
          }}
        >
          <polygon
            points="4,52.5 96,5 96,100"
            fill={theme.pointer}
            stroke={theme.pointerStroke}
            strokeWidth={7}
            strokeLinejoin="round"
          />
        </svg>

        {isRevealed && (
          <div
            style={{
              alignItems: 'center',
              background: theme.winnerPillBg,
              borderRadius: size * 0.03,
              color: theme.winnerPillText,
              display: 'flex',
              fontSize: winnerFontSize,
              fontWeight: 800,
              justifyContent: 'center',
              left: '50%',
              lineHeight: 1.2,
              outline: `${Math.round(size * 0.008)}px solid ${theme.winnerPillRing}`,
              padding: `${size * 0.022}px ${size * 0.042}px`,
              position: 'absolute',
              textAlign: 'center',
              top: '50%',
              transform: `translate(-50%, -50%) scale(${0.72 + 0.28 * pillPop}) translateZ(0)`,
              // `pre`, not `pre-line`: the two lines above are already chosen to
              // fit, and letting the browser re-wrap them turned a long name
              // into a four-line block that covered the wheel.
              whiteSpace: 'pre',
              // Above the confetti layer so the announcement is never occluded.
              zIndex: 12,
            }}
          >
            {winnerLines.join('\n')}
          </div>
        )}
      </div>

      <Confetti colors={theme.confetti} seed={props.winnerIndex} startFrame={revealFrame} />
    </AbsoluteFill>
  );
};
